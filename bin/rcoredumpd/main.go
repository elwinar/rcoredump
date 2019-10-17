package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/elwinar/rcoredump"
	"github.com/elwinar/rcoredump/conf"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/xid"
)

func main() {
	var s service
	s.configure()

	err := s.init()
	if err != nil {
		log.Println(err)
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

	received *prometheus.CounterVec
	router   *httprouter.Router
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

func (s *service) init() error {
	err := os.Mkdir(s.dir, os.ModeDir)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	s.received = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rcoredumpd_received_total",
		Help: "number of core dump received",
	}, []string{"hostname", "executable"})
	prometheus.MustRegister(s.received)

	s.router = httprouter.New()
	s.router.GET("/", s.home)
	s.router.POST("/core", s.receive)
	s.router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	return nil
}

func (s *service) run(ctx context.Context) {
	server := &http.Server{
		Addr:    s.bind,
		Handler: s.router,
	}

	go func() {
		<-ctx.Done()
		ctx, _ := context.WithTimeout(ctx, 1*time.Minute)
		server.Shutdown(ctx)
	}()

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Println("closing server:", err)
	}
}

func (s *service) home(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Write([]byte(`rcoredumpd`))
}

func (s *service) receive(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	defer func() {
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()
	}()

	uid := xid.New().String()
	log.Println("receiving dump", uid)

	// Uncompress the streams on the fly.
	bconn := bufio.NewReader(r.Body)
	zr, err := gzip.NewReader(bconn)
	if err != nil {
		log.Println("creating gzip reader")
		return
	}
	defer zr.Close()

	// Read the header struct.
	log.Println("reading header")
	zr.Multistream(false)
	var header rcoredump.Header
	err = json.NewDecoder(zr).Decode(&header)
	if err != nil {
		log.Println(err)
		return
	}

	f, err := os.Create(filepath.Join(s.dir, fmt.Sprintf("%s.json", uid)))
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(header)
	if err != nil {
		log.Println(err)
		return
	}

	// Read the core dump.
	log.Println("reading core")
	err = zr.Reset(bconn)
	if err != nil {
		log.Println(err)
		return
	}
	zr.Multistream(false)

	f, err = os.Create(filepath.Join(s.dir, fmt.Sprintf("%s.core", uid)))
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, zr)
	if err != nil {
		log.Println(err)
		return
	}

	// Read the binary file.
	log.Println("reading bin")
	err = zr.Reset(bconn)
	if err != nil {
		log.Println(err)
		return
	}
	zr.Multistream(false)

	f, err = os.Create(filepath.Join(s.dir, fmt.Sprintf("%s.bin", uid)))
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, zr)
	if err != nil {
		log.Println(err)
		return
	}

	s.received.With(prometheus.Labels{
		"hostname":   header.Hostname,
		"executable": header.Executable,
	}).Inc()
}
