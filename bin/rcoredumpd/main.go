package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elwinar/rcoredump"
	_ "github.com/elwinar/rcoredump/bin/rcoredumpd/internal"
	"github.com/elwinar/rcoredump/conf"

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
		signal.Notify(signals, os.Interrupt, os.Kill)
		<-signals
		cancel()
	}()

	s.run(ctx)
}

type service struct {
	// Configuration.
	bind         string
	dataDir      string
	syslog       bool
	filelog      string
	printVersion bool
	sizeBuckets  string
	indexType    string
	storeType    string
	goAnalyzer   string
	cAnalyzer    string

	// Dependencies
	assets        http.FileSystem
	index         Index
	logger        log15.Logger
	queue         chan string
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

	s.queue = make(chan string)

	s.logger.Debug("building assets")
	s.rootHTML = fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
			<head>
				<meta charset="utf-8" />
				<meta name="viewport" content="width=device-width, initial-scale=1" />
				<title>RCoredump</title>
				<link rel="stylesheet" href="/index.css">
			</head>
			<body>
				<noscript>You need to enable JavaScript to run this app.</noscript>
				<div id="root"></div>
				<script>document.Version = '%s'; document.BuiltAt = '%s'; document.Commit = '%s';</script>
				<script src="/index.js"></script>
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
		for uid := range s.queue {
			s.analyze(uid)
		}
		s.logger.Debug("stopping analysis queue")
	}()

	s.logger.Debug("looking for leftwover cores to analyze")
	go func() {
		uids, err := s.index.FindUnanalyzed()
		if err != nil {
			s.logger.Error("initializing analysis", "err", err)
			return
		}
		if len(uids) == 0 {
			return
		}

		s.logger.Debug("found leftover cores to analyze", "count", len(uids))
	loop:
		for _, uid := range uids {
			select {
			case <-ctx.Done():
				break loop
			case s.queue <- uid:
			}
		}
		s.logger.Debug("done analyzing leftover cores")
	}()

	s.logger.Debug("registering routes")
	router := httprouter.New()
	router.NotFound = http.FileServer(s.assets)
	router.GET("/", s.root)
	router.GET("/about", s.about)
	router.POST("/cores", s.indexCore)
	router.GET("/cores", s.searchCore)
	router.GET("/cores/:uid", s.getCore)
	router.POST("/cores/:uid/_analyze", s.analyzeCore)
	router.HEAD("/executables/:hash", s.lookupExecutable)
	router.GET("/executables/:hash", s.getExecutable)
	router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	s.logger.Debug("registering middlewares")
	stack := negroni.New()
	stack.Use(negroni.NewRecovery())
	stack.Use(negroni.HandlerFunc(s.logRequest))
	stack.Use(cors.Default())
	stack.UseHandler(router)

	s.logger.Debug("starting server")
	server := &http.Server{
		Addr:    s.bind,
		Handler: stack,
	}
	go func() {
		<-ctx.Done()
		ctx, _ := context.WithTimeout(ctx, 1*time.Minute)
		close(s.queue)
		server.Shutdown(ctx)
	}()
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("closing server", "err", err)
	}
	s.logger.Info("stopping server")
}

// logRequest is the logging middleware for the HTTP server.
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

func (s *service) root(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	rw.Write([]byte(s.rootHTML))
}

func (s *service) about(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	write(rw, http.StatusOK, map[string]string{
		"built_at": BuiltAt,
		"commit":   Commit,
		"version":  Version,
	})
}

// indexCore handle the requests for adding cores to the service. It exposes a
// prometheus metric for monitoring its activity, and only deals with storing
// the core and indexing the immutable information about it. Once done, it send
// the UID of the core in the analysis channel for the analyzis routine to pick
// it up.
func (s *service) indexCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	req := &indexRequest{
		index: s.index,
		log:   s.logger,
		r:     r,
		store: s.store,
	}
	req.init()
	req.read()
	req.readCore()
	if req.req.IncludeExecutable {
		req.readExecutable()
	} else {
		req.computeExecutableSize()
	}
	req.indexCore()
	req.close()

	if req.err != nil {
		s.logger.Error("indexing", "uid", req.uid, "err", req.err)
		writeError(w, http.StatusInternalServerError, req.err)
		return
	}

	s.received.With(prometheus.Labels{
		"hostname":   req.coredump.Hostname,
		"executable": req.coredump.Executable,
	}).Inc()

	s.receivedSizes.With(prometheus.Labels{
		"hostname":   req.coredump.Hostname,
		"executable": req.coredump.Executable,
	}).Observe(datasize.ByteSize(req.coredump.Size).MBytes())

	s.queue <- req.uid

	w.WriteHeader(http.StatusOK)
}

// analyzeCore handle the requests for re-analyzing a particular core. It
// should be useful when new features are implemented to re-analyze already
// existing cores and update them.
func (s *service) analyzeCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	uid := p.ByName("uid")

	found, err := s.index.Lookup(uid)
	if err != nil {
		s.logger.Error("analyzing", "uid", uid, "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !found {
		writeError(w, http.StatusBadRequest, errors.New("unknown core"))
		return
	}

	s.queue <- uid

	w.WriteHeader(http.StatusOK)
}

// analyze do the actual analysis of a core dump: language detection, strack
// trace extraction, etc.
func (s *service) analyze(uid string) {
	p := &analyzeProcess{
		dataDir: s.dataDir,
		index:   s.index,
		log:     s.logger.New("uid", uid),
		store:   s.store,
		uid:     uid,
	}

	p.init()
	p.detectLanguage()
	p.extractStackTrace()
	p.indexResults()
	p.cleanup()

	if p.err != nil {
		s.logger.Error("analyzing", "core", uid, "err", p.err)
		return
	}
}

// searchCore handle the requests to search cores matching a number of parameters.
func (s *service) searchCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var err error

	q := r.FormValue("q")
	if len(q) == 0 {
		q = "*"
	}

	sort := r.FormValue("sort")
	if len(sort) == 0 {
		sort = "dumped_at"
	}
	switch sort {
	case "dumped_at", "hostname":
		break
	default:
		writeError(w, http.StatusBadRequest, wrap(err, "invalid sort field"))
		return
	}

	order := r.FormValue("order")
	if len(order) == 0 {
		order = "desc"
	}
	switch order {
	case "asc", "desc":
		break
	default:
		writeError(w, http.StatusBadRequest, wrap(err, "invalid sort order"))
		return
	}

	rawSize := r.FormValue("size")
	if len(rawSize) == 0 {
		rawSize = "50"
	}
	size, err := strconv.Atoi(rawSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, wrap(err, "invalid size parameter"))
		return
	}

	rawFrom := r.FormValue("from")
	if len(rawFrom) == 0 {
		rawFrom = "0"
	}
	from, err := strconv.Atoi(rawFrom)
	if err != nil {
		writeError(w, http.StatusBadRequest, wrap(err, "invalid from parameter"))
		return
	}

	res, total, err := s.index.Search(q, sort, order, size, from)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	write(w, http.StatusOK, SearchResult{Results: res, Total: total})
}

// getCore handles the requests to get the actual core dump file.
func (s *service) getCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := s.store.Core(p.ByName("uid"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	io.Copy(w, f)
}

// lookupExecutable handles the requests to check if a executable matching the given
// hash actually exists. It doesn't return anything (except in case of error).
func (s *service) lookupExecutable(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	exists, err := s.store.ExecutableExists(p.ByName("hash"))
	if err != nil {
		s.logger.Warn("looking up executable", "hash", p.ByName("hash"), "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, errors.New(`not found`))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// getExecutable handles the requests to get the actual executable.
func (s *service) getExecutable(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := s.store.Executable(p.ByName("hash"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	io.Copy(w, f)
}

// Aliases of the shared types, for convenience.
type (
	Coredump     = rcoredump.Coredump
	IndexRequest = rcoredump.IndexRequest
	SearchResult = rcoredump.SearchResult
)

// write a payload and a status to the ResponseWriter.
func write(w http.ResponseWriter, status int, payload interface{}) {
	w.WriteHeader(status)
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(raw)
}

// write an error and a status to the ResponseWriter.
func writeError(w http.ResponseWriter, status int, err error) {
	write(w, status, rcoredump.Error{Err: err.Error()})
}

// wrap an error using the provided message and arguments.
func wrap(err error, msg string, args ...interface{}) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(msg, args...), err)
}
