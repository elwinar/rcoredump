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
	log  string

	args []string
}

func (s *service) configure() {
	fs := flag.NewFlagSet("rcoredump", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage of rcoredump: rcoredump [options] <executable path> <timestamp of dump>")
		fs.PrintDefaults()
	}
	fs.StringVar(&s.dest, "dest", "http://localhost:1105", "address of the destination host")
	fs.StringVar(&s.src, "src", "-", "path of the coredump to send to the host ('-' for stdin)")
	fs.StringVar(&s.log, "log", "/var/log/rcoredump.log", "path of the log file for rcoredump")
	fs.String("conf", "/etc/rcoredump/rcoredump.conf", "configuration file to load")
	conf.Parse(fs, "conf")

	s.args = fs.Args()
}

func (s *service) init() error {
	// Open the log file.
	l, err := os.OpenFile(s.log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer l.Close()
	log.SetOutput(l)

	return nil
}

func (s *service) run(ctx context.Context) {
	// Gather a few variables.
	// Args from the command line should be, in order:
	// - %E, pathname of executable
	// - %t, time of dump
	if len(s.args) != 2 {
		log.Println("unexpected number of arguments on command-line")
		return
	}

	// Pathname of the executable comes up with ! instead of /.
	executable := strings.Replace(s.args[0], "!", "/", -1)
	timestamp, err := strconv.ParseInt(s.args[1], 10, 64)
	if err != nil {
		log.Println("invalid timestamp format")
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
			log.Println("writing header:", err)
			return
		}
		err = w.Close()
		if err != nil {
			log.Println("closing header stream:", err)
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
				log.Println("opening core file:", err)
				return
			}
			defer in.Close()
		}
		_, err = io.Copy(w, in)
		if err != nil {
			log.Println("writing core:", err)
			return
		}
		err = w.Close()
		if err != nil {
			log.Println("closing core stream:", err)
			return
		}
		w.Reset(pw)

		// Then the binary.
		bin, err := os.Open(executable)
		if err != nil {
			log.Println("opening bin file:", err)
			return
		}
		defer in.Close()
		_, err = io.Copy(w, bin)
		if err != nil {
			log.Println("writing bin:", err)
			return
		}
		err = w.Close()
		if err != nil {
			log.Println("closing bin stream:", err)
			return
		}
	}()

	res, err := http.Post(fmt.Sprintf("%s/core", s.dest), "application/octet-stream", pr)
	if err != nil {
		log.Println("sending core:", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Println("unexpected status:", res.Status)
		return
	}

	// All done, k-thx-bye.
}
