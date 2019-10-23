package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/elwinar/rcoredump"
	"github.com/elwinar/rcoredump/conf"
	"github.com/inconshreveable/log15"
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
	dest string
	src  string

	args []string

	logger log15.Logger
}

func (s *service) configure() {
	fs := flag.NewFlagSet("rcoredump", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage of rcoredump: rcoredump [options] <executable path> <timestamp of dump>")
		fs.PrintDefaults()
	}
	fs.StringVar(&s.dest, "dest", "http://localhost:1105", "address of the destination host")
	fs.StringVar(&s.src, "src", "-", "path of the coredump to send to the host ('-' for stdin)")
	fs.String("conf", "/etc/rcoredump/rcoredump.conf", "configuration file to load")
	conf.Parse(fs, "conf")

	s.args = fs.Args()
}

func (s *service) init() error {
	s.logger = log15.New()
	s.logger.SetHandler(log15.StreamHandler(os.Stdout, log15.LogfmtFormat()))

	return nil
}

func (s *service) run(ctx context.Context) {
	// Gather a few variables.
	// Args from the command line should be, in order:
	// - %E, pathname of executable
	// - %t, time of dump
	if len(s.args) != 2 {
		s.logger.Error("unexpected number of arguments on command-line", "want", 2, "got", len(s.args))
		return
	}

	// Pathname of the executable comes up with ! instead of /.
	executable := strings.Replace(s.args[0], "!", "/", -1)
	timestamp, err := strconv.ParseInt(s.args[1], 10, 64)
	if err != nil {
		s.logger.Error("invalid timestamp format", "err", err)
		return
	}
	hostname, _ := os.Hostname()

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		// Compress the data before sending it.
		w := gzip.NewWriter(pw)
		defer w.Close()

		// Send the header first.
		err = json.NewEncoder(w).Encode(rcoredump.Header{
			Executable: executable,
			Date:       time.Unix(timestamp, 0),
			Hostname:   hostname,
		})
		if err != nil {
			s.logger.Error("writing header", "err", err)
			return
		}
		err = w.Close()
		if err != nil {
			s.logger.Error("closing header stream", "err", err)
			return
		}
		w.Reset(pw)

		// Then the core itself.
		var in io.ReadCloser
		if s.src == "-" {
			in = os.Stdin
		} else {
			in, err = os.Open(s.src)
			if err != nil {
				s.logger.Error("opening core file", "err", err)
				return
			}
			defer in.Close()
		}
		_, err = io.Copy(w, in)
		if err != nil {
			s.logger.Error("writing core", "err", err)
			return
		}
		err = w.Close()
		if err != nil {
			s.logger.Error("closing core stream", "err", err)
			return
		}
		w.Reset(pw)

		// Then the binary.
		bin, err := os.Open(executable)
		if err != nil {
			s.logger.Error("opening bin file", "err", err)
			return
		}
		defer in.Close()
		_, err = io.Copy(w, bin)
		if err != nil {
			s.logger.Error("writing bin", "err", err)
			return
		}
		err = w.Close()
		if err != nil {
			s.logger.Error("closing bin stream", "err", err)
			return
		}
	}()

	res, err := http.Post(fmt.Sprintf("%s/_index", s.dest), "application/octet-stream", pr)
	if err != nil {
		s.logger.Error("sending core", "err", err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		s.logger.Error("unexpected status", "err", err)
		return
	}

	// All done, k-thx-bye.
}
