package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/elwinar/rcoredump"
	_ "github.com/elwinar/rcoredump/bin/rcoredumpd/internal"
	"github.com/elwinar/rcoredump/conf"
	"github.com/inconshreveable/log15"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakyll/statik/fs"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
)

var Version = "N/C"

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
	bind         string
	dir          string
	log          string
	printVersion bool
	goAnalyzer   string
	cAnalyzer    string

	logger    log15.Logger
	received  *prometheus.CounterVec
	router    *httprouter.Router
	stack     *negroni.Negroni
	index     bleve.Index
	queue     chan string
	analyzers map[string]*template.Template
}

// configure read and validate the configuration of the service and populate
// the appropriate fields.
func (s *service) configure() {
	fs := flag.NewFlagSet("rcoredumpd-"+Version, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage of rcoredumpd: rcoredumpd [options]")
		fs.PrintDefaults()
	}
	fs.StringVar(&s.bind, "bind", "localhost:1105", "address to listen to")
	fs.StringVar(&s.dir, "dir", "/var/lib/rcoredumpd/", "path of the directory to store the coredumps into")
	fs.StringVar(&s.goAnalyzer, "go.analyzer", "dlv core {{ .Executable }} {{ .Core }} --init {{ .Dir }}/delve.cmd", "command to run to analyze Go core dumps")
	fs.StringVar(&s.cAnalyzer, "c.analyzer", "gdb --nx --ex bt --batch {{ .Executable }} {{ .Core }}", "command to run to analyze C core dumps")
	fs.BoolVar(&s.printVersion, "version", false, "print the version of rcoredumpd")
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

	// Logger
	s.logger = log15.New()
	s.logger.SetHandler(log15.StreamHandler(os.Stdout, log15.LogfmtFormat()))

	// Data dir
	s.logger.Debug("creating data directories")
	for _, dir := range []string{
		s.dir,
		exepath(s.dir, ""),
		corepath(s.dir, ""),
	} {
		err = os.Mkdir(dir, os.ModeDir|0774)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return wrap(err, `creating data directory`)
		}
	}

	// Ensure the default command file for Go is in the data directory.
	if _, err := os.Stat(filepath.Join(s.dir, "delve.cmd")); os.IsNotExist(err) {
		err := ioutil.WriteFile(filepath.Join(s.dir, "delve.cmd"), []byte("bt\nq\n"), 0774)
		if err != nil {
			return wrap(err, `writing default delve command file`)
		}
	}

	// Parse the analyzer command templates.
	s.analyzers = make(map[string]*template.Template)
	for lang, src := range map[string]string{
		rcoredump.LangGo: s.goAnalyzer,
		rcoredump.LangC:  s.cAnalyzer,
	} {
		src = strings.TrimSpace(src)
		if len(src) == 0 {
			continue
		}

		tpl, err := template.New(lang).Parse(src)
		if err != nil {
			return wrap(err, "parsing analyze command for %s", lang)
		}
		s.analyzers[lang] = tpl
	}

	// Prometheus metrics
	s.logger.Debug("registering metrics")
	s.received = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rcoredumpd_received_total",
		Help: "number of core dump received",
	}, []string{"hostname", "executable"})
	prometheus.MustRegister(s.received)

	// Static files
	s.logger.Debug("fetching static files")
	public, err := fs.New()
	if err != nil {
		return wrap(err, `retrieving assets`)
	}

	// API Routes
	s.logger.Debug("registering routes")
	s.router = httprouter.New()
	s.router.NotFound = http.FileServer(public)

	s.router.POST("/cores", s.indexCore)
	s.router.GET("/cores", s.searchCore)
	s.router.GET("/cores/:uid", s.getCore)
	s.router.POST("/cores/:uid/_analyze", s.analyzeCore)

	s.router.HEAD("/executables/:hash", s.lookupExecutable)
	s.router.GET("/executables/:hash", s.getExecutable)

	s.router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	// Middleware stack
	s.stack = negroni.New()
	s.stack.Use(negroni.NewRecovery())
	s.stack.Use(negroni.HandlerFunc(s.logRequest))
	s.stack.Use(cors.Default())
	s.stack.UseHandler(s.router)

	// Fulltext Index
	indexPath := filepath.Join(s.dir, "index")
	_, err = os.Stat(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return wrap(err, `checking for index`)
	}

	if errors.Is(err, os.ErrNotExist) {
		s.logger.Debug("creating index", "path", indexPath)
		s.index, err = bleve.New(indexPath, bleve.NewIndexMapping())
	} else {
		s.logger.Debug("opening index", "path", indexPath)
		s.index, err = bleve.Open(indexPath)
	}
	if err != nil {
		return wrap(err, `opening index`)
	}

	// Analysis channel and routines.
	s.logger.Debug("starting analysis queue")
	s.queue = make(chan string)
	go func() {
		for uid := range s.queue {
			s.analyze(uid)
		}
	}()

	return nil
}

// run does the actual running of the service until the context is closed.
func (s *service) run(ctx context.Context) {
	// Look for cores not yet analyzed and add them to the queue.
	go func() {
		query := bleve.NewBoolFieldQuery(false)
		query.SetField("analyzed")

		req := bleve.NewSearchRequest(query)
		req.Fields = []string{"uid"}

		res, err := s.index.Search(req)
		if err != nil {
			s.logger.Error("initializing analysis", "err", err)
			return
		}

		if len(res.Hits) == 0 {
			return
		}

		s.logger.Debug("found leftover dumps to analyze", "count", len(res.Hits))
		for _, d := range res.Hits {
			uid, ok := d.Fields["uid"].(string)
			if !ok {
				s.logger.Error("initializing analysis", "err", fmt.Errorf("uid field for document %s isn't a string", d.ID))
				continue
			}

			s.queue <- uid
		}
	}()

	server := &http.Server{
		Addr:    s.bind,
		Handler: s.stack,
	}

	go func() {
		<-ctx.Done()
		ctx, _ := context.WithTimeout(ctx, 1*time.Minute)
		server.Shutdown(ctx)
	}()

	s.logger.Info("starting")
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("closing server", "err", err)
	}
	s.logger.Info("stopping")
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

// indexCore handle the requests for adding cores to the service. It exposes a
// prometheus metric for monitoring its activity, and only deals with storing
// the core and indexing the immutable information about it. Once done, it send
// the UID of the core in the analysis channel for the analyzis routine to pick
// it up.
func (s *service) indexCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	req := &indexRequest{
		log:   s.logger,
		r:     r,
		dir:   s.dir,
		index: s.index,
	}
	req.init()
	req.read()
	req.readCore()
	if req.IncludeExecutable {
		req.readExecutable()
	}
	req.indexCore()
	req.close()

	if req.err != nil {
		s.logger.Error("indexing", "uid", req.uid, "err", req.err)
		writeError(w, http.StatusInternalServerError, req.err)
		return
	}

	s.received.With(prometheus.Labels{
		"hostname":   req.Hostname,
		"executable": req.ExecutablePath,
	}).Inc()

	s.queue <- req.uid

	w.WriteHeader(http.StatusOK)
}

// analyzeCore handle the requests for re-analyzing a particular core. It
// should be useful when new features are implemented to re-analyze already
// existing cores and update them.
func (s *service) analyzeCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	uid := p.ByName("uid")

	d, err := s.index.Document(uid)
	if err != nil {
		s.logger.Error("analyzing", "uid", uid, "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if d == nil {
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
		uid:       uid,
		log:       s.logger,
		index:     s.index,
		dir:       s.dir,
		analyzers: s.analyzers,
	}

	p.init()
	p.findCore()
	p.detectLanguage()
	p.extractStackTrace()
	p.indexResults()

	if p.err != nil {
		s.logger.Error("analyzing", "core", uid, "err", p.err)
		return
	}
}

// searchCore handle the requests to search cores matching a number of parameters.
func (s *service) searchCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var err error

	// Create the search request first.
	req := bleve.NewSearchRequest(
		bleve.NewQueryStringQuery(r.FormValue("q")),
	)
	// Add the fields to look for.
	req.Fields = []string{"*"}
	// If there is a sort parameter in the form, add it to the search
	// string.
	sort := r.FormValue("sort")
	if len(sort) != 0 {
		req.SortBy(strings.Split(sort, ","))
	} else {
		req.SortBy([]string{"-date"})
	}
	// If there is a size parameter in the form, add it to the search
	// string.
	size := r.FormValue("size")
	if len(size) != 0 {
		req.Size, err = strconv.Atoi(size)
		if err != nil {
			writeError(w, http.StatusBadRequest, wrap(err, "invalid size parameter"))
			return
		}
	} else {
		req.Size = 20
	}

	res, err := s.index.Search(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	write(w, http.StatusOK, res)
}

// getCore handles the requests to get the actual core dump file.
func (s *service) getCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := os.Open(filepath.Join(s.dir, "cores", p.ByName("uid")))
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
	_, err := os.Stat(exepath(s.dir, p.ByName("hash")))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("looking up executable", "hash", p.ByName("hash"), "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// getExecutable handles the requests to get the actual executable.
func (s *service) getExecutable(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := os.Open(exepath(s.dir, p.ByName("hash")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	io.Copy(w, f)
}

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

func exepath(base, file string) string {
	return filepath.Join(base, "executables", file)
}

func corepath(base, file string) string {
	return filepath.Join(base, "cores", file)
}
