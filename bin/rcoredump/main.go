package main

import (
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
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
	dest       string
	src        string
	sendBinary bool
	args       []string

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
	fs.BoolVar(&s.sendBinary, "send-binary", true, "send the binary along with the dump")
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

	// Look up the binary in the server by using its sha1 hash. The
	// operation can fail in which case we will continue and consider that
	// the binary wasn't found so we don't lose the dump.
	var found bool
	hash, err := s.hashBinary(executable)
	if s.sendBinary && err == nil {
		found, err = s.lookupBinary(hash)
	}
	if err != nil {
		s.logger.Error("looking up binary", "err", err)
	}
	var sendBinary = s.sendBinary && !found

	// We will use chunked transfer encoding to avoid keeping the whole
	// dump in memory more than necessary. We will do this by giving the
	// request a pipe as body, so it will read from it and send the content
	// in multiple packets. This is a necessity given that a dump can
	// measure in GB.
	pr, pw := io.Pipe()

	// Fill up the pipe in a routine so the sending happens in parallel and
	// the memory consumption is kept in check.
	go func() {
		defer pw.Close()

		// Send the header.
		w := gzip.NewWriter(pw)
		defer w.Close()

		err := json.NewEncoder(w).Encode(rcoredump.IndexRequest{
			Date:           time.Unix(timestamp, 0),
			Hostname:       hostname,
			ExecutablePath: executable,
			BinaryHash:     hash,
			IncludeBinary:  sendBinary,
		})
		if err != nil {
			s.logger.Error("sending header", "err", err)
			return
		}

		err = w.Close()
		if err != nil {
			s.logger.Error("closing header stream", "err", err)
			return
		}

		// Send the core.
		w.Reset(pw)

		err = s.sendFile(w, s.src)
		if err != nil {
			s.logger.Error("sending core", "err", err)
			return
		}

		err = w.Close()
		if err != nil {
			s.logger.Error("closing header stream", "err", err)
			return
		}

		// Check if we want to send the binary.
		if !sendBinary {
			return
		}

		// Send the binary.
		w.Reset(pw)

		err = s.sendFile(w, executable)
		if err != nil {
			s.logger.Error("sending binary", "err", err)
			return
		}

		err = w.Close()
		if err != nil {
			s.logger.Error("closing binary stream", "err", err)
			return
		}
	}()

	// Send the request by giving it the reader end of the pipe.
	res, err := http.Post(fmt.Sprintf("%s/cores", s.dest), "application/octet-stream", pr)
	if err != nil {
		s.logger.Error("sending core", "err", err)
		return
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		var err rcoredump.Error
		_ = json.NewDecoder(res.Body).Decode(&err)
		s.logger.Error("unexpected status", "err", err.Err)
		return
	}
}

func (s *service) hashBinary(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", wrap(err, "opening executable")
	}
	defer f.Close()

	h := sha1.New()

	_, err = io.Copy(h, f)
	if err != nil {
		return "", wrap(err, "hashing executable")
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *service) lookupBinary(hash string) (bool, error) {
	res, err := http.Head(fmt.Sprintf("%s/binaries/%s", s.dest, hash))
	if err != nil {
		return false, wrap(err, "executing request")
	}
	defer res.Body.Close()

	raw, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, wrap(err, "reading response")
	}

	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		var err rcoredump.Error
		json.Unmarshal(raw, &err)
		return false, wrap(errors.New(err.Err), "unexpected response")
	}
}

func (s *service) sendFile(w io.Writer, path string) error {
	var err error
	var f io.ReadCloser
	if path == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(path)
		if err != nil {
			return wrap(err, "opening file")
		}
		defer f.Close()
	}

	_, err = io.Copy(w, f)
	if err != nil {
		return wrap(err, "writing file")
	}

	return nil
}

// wrap an error using the provided message and arguments.
func wrap(err error, msg string, args ...interface{}) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(msg, args...), err)
}
