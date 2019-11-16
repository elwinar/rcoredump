package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	bind string
	dir  string
	log  string

	logger   log15.Logger
	received *prometheus.CounterVec
	router   *httprouter.Router
	stack    *negroni.Negroni
	index    bleve.Index
}

func (s *service) configure() {
	fs := flag.NewFlagSet("rcoredumpd", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage of rcoredumpd:")
		fs.PrintDefaults()
	}
	fs.StringVar(&s.bind, "bind", "localhost:1105", "address to listen to")
	fs.StringVar(&s.dir, "dir", "/var/lib/rcoredumpd/", "path of the directory to store the coredumps into")
	fs.String("conf", "/etc/rcoredump/rcoredumpd.conf", "configuration file to load")
	conf.Parse(fs, "conf")
}

func (s *service) init() (err error) {
	// Logger
	s.logger = log15.New()
	s.logger.SetHandler(log15.StreamHandler(os.Stdout, log15.LogfmtFormat()))

	// Data dir
	for _, dir := range []string{
		s.dir,
		filepath.Join(s.dir, "binaries"),
		filepath.Join(s.dir, "cores"),
	} {
		err = os.Mkdir(dir, os.ModeDir|0776)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf(`creating data directory: %w`, err)
		}
	}

	// Prometheus metrics
	s.received = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rcoredumpd_received_total",
		Help: "number of core dump received",
	}, []string{"hostname", "executable"})
	prometheus.MustRegister(s.received)

	// Static files
	public, err := fs.New()
	if err != nil {
		return fmt.Errorf(`retrieving assets: %w`, err)
	}

	// API Routes
	s.router = httprouter.New()
	s.router.NotFound = http.FileServer(public)

	s.router.POST("/cores", s.indexCore)
	s.router.GET("/cores", s.searchCore)
	s.router.HEAD("/cores/:uid", s.lookupCore)
	s.router.GET("/cores/:uid", s.getCore)

	s.router.HEAD("/binaries/:hash", s.lookupBinary)
	s.router.GET("/binaries/:hash", s.getBinary)

	s.router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	// Middleware stack
	s.stack = negroni.New()
	s.stack.Use(negroni.NewRecovery())
	s.stack.Use(cors.Default())
	s.stack.UseHandler(s.router)

	// Fulltext Index
	indexPath := filepath.Join(s.dir, "index")
	_, err = os.Stat(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf(`checking for index: %w`, err)
	}

	if errors.Is(err, os.ErrNotExist) {
		s.index, err = bleve.New(indexPath, bleve.NewIndexMapping())
	} else {
		s.index, err = bleve.Open(indexPath)
	}
	if err != nil {
		return fmt.Errorf(`opening index: %w`, err)
	}

	return nil
}

func (s *service) run(ctx context.Context) {
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

func (s *service) indexCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	req := newIndexRequest(r, s.logger)
	defer req.close()

	req.read()
	req.readCore(filepath.Join(s.dir, "cores", req.uid))
	if req.IncludeBinary {
		req.readBinary(filepath.Join(s.dir, "binaries", req.BinaryHash))
	}
	req.indexCore(s.index)

	if req.err != nil {
		s.logger.Error("indexing", "uid", req.uid, "err", req.err)
		write(w, http.StatusInternalServerError, rcoredump.Error{Err: req.err.Error()})
		return
	}

	s.received.With(prometheus.Labels{
		"hostname":   req.Hostname,
		"executable": req.ExecutablePath,
	}).Inc()
}

func (s *service) searchCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	res, err := s.index.Search(req)
	if err != nil {
		write(w, http.StatusBadRequest, rcoredump.Error{Err: err.Error()})
		return
	}

	write(w, http.StatusOK, res)
}

func (s *service) lookupCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_, err := os.Stat(filepath.Join(s.dir, "cores", p.ByName("uid")))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("looking up core", "uid", p.ByName("uid"), "err", err)
		write(w, http.StatusInternalServerError, rcoredump.Error{Err: err.Error()})
		return
	}

	if err != nil {
		write(w, http.StatusNotFound, rcoredump.Error{Err: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *service) getCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := os.Open(filepath.Join(s.dir, "cores", p.ByName("uid")))
	if err != nil {
		write(w, http.StatusInternalServerError, rcoredump.Error{Err: err.Error()})
		return
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	io.Copy(w, f)
}

func (s *service) lookupBinary(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_, err := os.Stat(filepath.Join(s.dir, "binaries", p.ByName("hash")))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("looking up binary", "hash", p.ByName("hash"), "err", err)
		write(w, http.StatusInternalServerError, rcoredump.Error{Err: err.Error()})
		return
	}

	if err != nil {
		write(w, http.StatusNotFound, rcoredump.Error{Err: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *service) getBinary(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := os.Open(filepath.Join(s.dir, "binaries", p.ByName("hash")))
	if err != nil {
		write(w, http.StatusInternalServerError, rcoredump.Error{Err: err.Error()})
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

// wrap an error using the provided message and arguments.
func wrap(err error, msg string, args ...interface{}) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(msg, args...), err)
}
