package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/elwinar/rcoredump/conf"
	_ "github.com/elwinar/rcoredump/bin/rcoredumpd/internal"
	"github.com/inconshreveable/log15"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
	"github.com/rakyll/statik/fs"
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
	bind      string
	dir       string
	log       string

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
	err = os.Mkdir(s.dir, os.ModeDir|0776)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
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
		return err
	}

	// API Routes
	s.router = httprouter.New()
	s.router.NotFound = http.FileServer(public)
	s.router.POST("/_index", s._index)
	s.router.GET("/_search", s._search)
	s.router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	s.stack = negroni.New()
	s.stack.Use(negroni.NewRecovery())
	s.stack.Use(cors.Default())
	s.stack.UseHandler(s.router)

	// Fulltext Index
	indexPath := filepath.Join(s.dir, "index")
	_, err = os.Stat(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if errors.Is(err, os.ErrNotExist) {
		s.index, err = bleve.New(indexPath, bleve.NewIndexMapping())
	} else {
		s.index, err = bleve.Open(indexPath)
	}
	if err != nil {
		return err
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

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("closing server", "err", err)
	}
}

func (s *service) _index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	req := newIndexRequest(r, s.logger, s.dir, s.index)
	defer req.close()

	req.readHeader()
	req.readCore()
	req.readBinary()
	req.indexCore()

	if req.err != nil {
		s.logger.Error("indexing", "uid", req.uid, "err", req.err)
		write(w, http.StatusInternalServerError, Error{Err: req.err.Error()})
		return
	}

	s.received.With(prometheus.Labels{
		"hostname":   req.header.Hostname,
		"executable": req.header.Executable,
	}).Inc()
}

func (s *service) _search(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	qs := r.FormValue("q")
	q := bleve.NewQueryStringQuery(qs)
	req := bleve.NewSearchRequest(q)
	req.Fields = []string{"*"}

	res, err := s.index.Search(req)
	if err != nil {
		write(w, http.StatusBadRequest, Error{Err: err.Error()})
		return
	}

	write(w, http.StatusOK, res)
}
