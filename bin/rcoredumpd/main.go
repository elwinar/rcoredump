package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log/syslog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/elwinar/rcoredump/bin/rcoredumpd/internal"
	"github.com/elwinar/rcoredump/pkg/conf"
	. "github.com/elwinar/rcoredump/pkg/rcoredump"

	"github.com/c2h5oh/datasize"
	"github.com/inconshreveable/log15"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakyll/statik/fs"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
)

var (
	Version = "N/C"
	BuiltAt = "N/C"
	Commit  = "N/C"
)

// main is tasked to bootstrap the service and notify of termination signals.
func main() {
	var s service
	s.configure()

	err := s.init()
	if err != nil {
		s.logger.Crit("initializing", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		cancel()
	}()

	s.run(ctx)
}

// wrap an error using the provided message and arguments.
func wrap(err error, msg string, args ...interface{}) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(msg, args...), err)
}

type service struct {
	// Configuration.
	bind              string
	dataDir           string
	syslog            bool
	filelog           string
	printVersion      bool
	sizeBuckets       string
	retentionDuration time.Duration
	indexType         string
	storeType         string
	goAnalyzer        string
	cAnalyzer         string

	// Dependencies
	assets        http.FileSystem
	index         Index
	logger        log15.Logger
	analysisQueue chan Coredump
	cleanupQueue  chan Coredump
	received      *prometheus.CounterVec
	receivedSizes *prometheus.HistogramVec
	store         Store
	rootHTML      string
}

// configure read and validate the configuration of the service and populate
// the appropriate fields.
func (s *service) configure() {
	fs := flag.NewFlagSet("rcoredumpd-"+Version, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage of rcoredumpd: rcoredumpd [options]")
		fs.PrintDefaults()
	}

	// General options.
	fs.StringVar(&s.bind, "bind", "localhost:1105", "address to listen to")
	fs.StringVar(&s.dataDir, "data-dir", "/var/lib/rcoredumpd", "directory to store server's data")
	fs.BoolVar(&s.syslog, "syslog", false, "output logs to syslog")
	fs.StringVar(&s.filelog, "filelog", "-", "path of the file to log into (\"-\" for stdout)")
	fs.BoolVar(&s.printVersion, "version", false, "print the version of rcoredumpd")
	fs.StringVar(&s.sizeBuckets, "size-buckets", "1MB,10MB,100MB,1GB,10GB", "buckets report the coredump sizes for")
	fs.DurationVar(&s.retentionDuration, "retention-duration", 0, "duration to keep an indexed coredump (e.g: \"168h\"), 0 to disable")

	// Interface options.
	fs.StringVar(&s.indexType, "index-type", "bleve", "type of index to use (values: bleve)")
	fs.StringVar(&s.storeType, "store-type", "file", "type of store to use (values: file)")

	// Analyzer options.
	fs.StringVar(&s.goAnalyzer, "go.analyzer", "bt", "delve command to run to generate the stack trace for Go coredumps")
	fs.StringVar(&s.cAnalyzer, "c.analyzer", "bt", "gdb command to run to generate the stack trace for C coredumps")

	fs.String("conf", "/etc/rcoredump/rcoredumpd.conf", "configuration file to load")
	conf.Parse(fs, "conf")
}

// init does the actual bootstraping of the service, once the configuration is
// read. It encompass any start-up task like ensuring the storage directories
// exist, initializing the index if needed, registering the endpoints, etc.
func (s *service) init() (err error) {
	if s.printVersion {
		fmt.Println("rcoredumpd", Version)
		os.Exit(0)
	}

	s.logger = log15.New()
	format := log15.LogfmtFormat()
	var handler log15.Handler
	if s.syslog {
		handler, err = log15.SyslogHandler(syslog.LOG_KERN, "rcoredumpd", format)
	} else if s.filelog == "-" {
		handler, err = log15.StreamHandler(os.Stdout, format), nil
	} else {
		handler, err = log15.FileHandler(s.filelog, format)
	}
	if err != nil {
		return err
	}
	s.logger.SetHandler(handler)

	s.logger.Debug("registering metrics")
	s.received = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rcoredumpd_received_total",
		Help: "number of core dump received",
	}, []string{"hostname", "executable"})
	prometheus.MustRegister(s.received)

	var buckets []float64
	for _, raw := range strings.Split(s.sizeBuckets, ",") {
		var b datasize.ByteSize
		err := b.UnmarshalText([]byte(raw))
		if err != nil {
			return wrap(err, `invalid value for size-buckets option`)
		}
		buckets = append(buckets, b.MBytes())
	}

	s.receivedSizes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rcoredumpd_received_sizes_megabytes",
		Help:    "sizes of the received core dumps",
		Buckets: buckets,
	}, []string{"hostname", "executable"})
	prometheus.MustRegister(s.receivedSizes)

	s.logger.Debug("retrieving embeded assets")
	s.assets, err = fs.New()
	if err != nil {
		return wrap(err, `retrieving embeded assets`)
	}

	s.logger.Debug("initializing data directory")
	err = os.Mkdir(s.dataDir, os.ModeDir|0774)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return wrap(err, `creating data directory`)
	}

	err = ioutil.WriteFile(filepath.Join(s.dataDir, "delve.cmd"), []byte(s.goAnalyzer+"\nq\n"), 0774)
	if err != nil {
		return wrap(err, `writing default delve command file`)
	}

	err = ioutil.WriteFile(filepath.Join(s.dataDir, "gdb.cmd"), []byte(s.cAnalyzer+"\nq\n"), 0774)
	if err != nil {
		return wrap(err, `writing default delve command file`)
	}

	s.logger.Debug("initializing store")
	switch s.storeType {
	case "file":
		s.store, err = NewFileStore(filepath.Join(s.dataDir, "store"))
	default:
		return fmt.Errorf(`unknown store type %s`, s.storeType)
	}
	if err != nil {
		return wrap(err, `initializing store`)
	}

	s.logger.Debug("initializing index")
	switch s.indexType {
	case "bleve":
		s.index, err = NewBleveIndex(filepath.Join(s.dataDir, "index"))
	default:
		return fmt.Errorf(`unknown index type %s`, s.indexType)
	}
	if err != nil {
		return wrap(err, `initializing index`)
	}

	s.analysisQueue = make(chan Coredump)
	s.cleanupQueue = make(chan Coredump)

	s.logger.Debug("building assets")
	s.rootHTML = fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
			<head>
				<meta charset="utf-8" />
				<meta name="viewport" content="width=device-width, initial-scale=1" />
				<title>RCoredump</title>
				<link rel="stylesheet" href="/assets/index.css">
				<link rel="shortcut icon" type="image/svg" href="/assets/favicon.svg"/>
			</head>
			<body>
				<noscript>You need to enable JavaScript to run this app.</noscript>
				<div id="root"></div>
				<script>document.Version = '%s'; document.BuiltAt = '%s'; document.Commit = '%s';</script>
				<script src="/assets/index.js"></script>
			</body>
		</html>
	`, Version, BuiltAt, Commit)

	return nil
}

// run does the actual running of the service until the context is closed.
func (s *service) run(ctx context.Context) {
	var wg sync.WaitGroup

	s.logger.Debug("starting analysis queue")
	wg.Add(1)
	go func() {
		defer wg.Done()
		for core := range s.analysisQueue {
			s.analyze(core)
		}
		s.logger.Debug("stopping analysis queue")
	}()
	go s.findUnanalyzed(ctx)

	s.logger.Debug("starting cleaning queue")
	wg.Add(1)
	go func() {
		defer wg.Done()
		for core := range s.cleanupQueue {
			s.cleanup(core)
		}
		s.logger.Debug("stopping cleaning queue")
	}()
	// Find cleanable cores in a separate routine, only if the retention
	// duration is configured.
	if s.retentionDuration != 0 {
		go s.findCleanable(ctx)
	}

	s.logger.Debug("registering routes")
	router := httprouter.New()
	router.GET("/", s.root)
	router.GET("/about", s.about)
	router.POST("/cores", s.indexCore)
	router.GET("/cores", s.searchCore)
	router.GET("/cores/:uid", s.getCore)
	router.DELETE("/cores/:uid", s.deleteCore)
	router.POST("/cores/:uid/_analyze", s.analyzeCore)
	router.HEAD("/executables/:hash", s.lookupExecutable)
	router.GET("/executables/:hash", s.getExecutable)
	router.Handler(http.MethodGet, "/metrics", promhttp.Handler())
	router.ServeFiles("/assets/*filepath", s.assets)
	router.NotFound = http.HandlerFunc(s.notFound)
	router.MethodNotAllowed = http.HandlerFunc(s.methodNotAllowed)

	s.logger.Debug("registering middlewares")
	stack := negroni.New()
	stack.Use(negroni.NewRecovery())
	stack.Use(negroni.HandlerFunc(s.logRequest))
	stack.Use(negroni.HandlerFunc(s.delayRequest))
	stack.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodDelete},
	}))
	stack.UseHandler(router)

	s.logger.Debug("starting server")
	server := &http.Server{
		Addr:    s.bind,
		Handler: stack,
	}
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		close(s.analysisQueue)
		close(s.cleanupQueue)
		err := server.Shutdown(ctx)
		if err != nil {
			s.logger.Error("shuting server down", "err", err)
			return
		}
	}()
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("closing server", "err", err)
	}
	s.logger.Info("stopping server")
}

// Log a request with a few metadata to ensure requests are monitorable.
func (s *service) logRequest(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	start := time.Now()

	next(rw, r)

	res := rw.(negroni.ResponseWriter)
	s.logger.Info("request",
		"started_at", start,
		"duration", time.Since(start),
		"method", r.Method,
		"path", r.URL.Path,
		"status", res.Status(),
	)
}

func (s *service) delayRequest(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// Arbitrary delay can be used to add artificial slowness to the API,
	// so we can test the API with various conditions.
	rawDelay := r.FormValue("delay")
	if len(rawDelay) != 0 {
		delay, err := time.ParseDuration(rawDelay)
		if err != nil {
			writeError(rw, http.StatusBadRequest, wrap(err, "parsing delay"))
			return
		}
		time.Sleep(delay)
	}

	next(rw, r)
}

// Find unanalyzed coredumps and feed them to the analyze queue.
func (s *service) findUnanalyzed(ctx context.Context) {
	for {
		// Note: searching for boolean fields in BleveSearch is fucked
		// up. See here:
		// https://github.com/blevesearch/bleve/issues/626
		cores, _, err := s.index.Search(`analyzed:F*`, "dumped_at", "asc", 100, 0)
		if err != nil {
			s.logger.Error("initializing analysis", "err", err)
			return
		}
		if len(cores) == 0 {
			return
		}

		s.logger.Debug("found leftover cores to analyze", "count", len(cores))
		defer s.logger.Debug("done analyzing leftover cores")
		for _, core := range cores {
			select {
			case <-ctx.Done():
				return
			case s.analysisQueue <- core:
			}
		}
	}
}

// Find cleanable coredumps and feed them to the cleanup queue.
func (s *service) findCleanable(ctx context.Context) {
	t := time.NewTimer(1 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for {
				cores, _, err := s.index.Search(fmt.Sprintf(`dumped_at:<"%s"`, time.Now().Add(-s.retentionDuration).Format(time.RFC3339)), "dumped_at", "asc", 100, 0)
				if err != nil {
					s.logger.Error("finding cleanable cores", "err", err)
					break
				}
				if len(cores) == 0 {
					s.logger.Debug("no core to clean")
					break
				}

				s.logger.Debug("found cleanable cores", "count", len(cores))
				for _, core := range cores {
					select {
					case <-ctx.Done():
						return
					case s.cleanupQueue <- core:
					}
				}
			}
		}
	}
}

// analyze do the actual analysis of a core dump: language detection, strack
// trace extraction, etc.
func (s *service) analyze(core Coredump) {
	p := &analyzeProcess{
		dataDir: s.dataDir,
		index:   s.index,
		log:     s.logger.New("uid", core.UID),
		store:   s.store,
		core:    core,
	}

	p.init()
	p.detectLanguage()
	p.extractStackTrace()
	p.indexResults()
	p.cleanup()

	if p.err != nil {
		s.logger.Error("analyzing", "core", core.UID, "err", p.err)
		return
	}
}

// cleanup do the actual cleanup of a core dump: removing the file, the indexed
// document, and eventually the executable.
func (s *service) cleanup(core Coredump) {
	p := &cleanupProcess{
		index: s.index,
		log:   s.logger.New("uid", core.UID),
		store: s.store,
		core:  core,
	}

	p.cleanIndex()
	p.cleanStore()
	if p.canCleanExecutable() {
		p.cleanExecutable()
	}

	if p.err != nil {
		s.logger.Error("analyzing", "core", core.UID, "err", p.err)
		return
	}
}
