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
	"log/syslog"
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

var Version = "N/C"

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
	dest           string
	src            string
	sendExecutable bool
	syslog         bool
	filelog        string
	printVersion   bool
	args           []string
	metadata       map[string]string

	logger log15.Logger
}

func (s *service) configure() {
	fs := flag.NewFlagSet("rcoredump-"+Version, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage of rcoredump: rcoredump [options] <executable path> <timestamp of dump>")
		fs.PrintDefaults()
	}
	fs.StringVar(&s.dest, "dest", "http://localhost:1105", "address of the destination host")
	fs.StringVar(&s.src, "src", "-", "path of the coredump to send to the host (\"-\" for stdin)")
	fs.BoolVar(&s.syslog, "syslog", false, "output logs to syslog")
	fs.StringVar(&s.filelog, "filelog", "-", "path of the file to log into (\"-\" for stdout)")
	fs.BoolVar(&s.printVersion, "version", false, "print the version of rcoredump")
	fs.Var(conf.MapFlag(&s.metadata), "metadata", "list of metadata to send alongside the coredump (key=value, can be specified multiple times or separated by ';')")
	fs.String("conf", "/etc/rcoredump/rcoredump.conf", "configuration file to load")
	conf.Parse(fs, "conf")

	s.args = fs.Args()
}

func (s *service) init() error {
	if s.printVersion {
		fmt.Println("rcoredump", Version)
		os.Exit(0)
	}

	s.logger = log15.New()

	format := log15.LogfmtFormat()
	var handler log15.Handler
	var err error
	if s.syslog {
		handler, err = log15.SyslogHandler(syslog.LOG_KERN, "rcoredump", format)
	} else if s.filelog == "-" {
		handler, err = log15.StreamHandler(os.Stdout, format), nil
	} else {
		handler, err = log15.FileHandler(s.filelog, format)
	}
	if err != nil {
		return err
	}
	s.logger.SetHandler(handler)

	return nil
}

func (s *service) run(ctx context.Context) {
	s.logger.Debug("starting")

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

	// Look up the executable in the server by using its sha1 hash. The
	// operation can fail in which case we will continue and consider that
	// the executable wasn't found so we don't lose the dump.
	s.logger.Debug("hashing executable")
	sendExecutable := true
	hash, err := s.hashExecutable(executable)
	if err != nil {
		s.logger.Error("hashing executable", "err", err)
	} else {
		found, err := s.lookupExecutable(hash)
		if err != nil {
			s.logger.Error("looking up executable", "err", err)
		}
		sendExecutable = !found
	}

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

		s.logger.Debug("sending header")
		err := json.NewEncoder(w).Encode(rcoredump.IndexRequest{
			DumpedAt:          time.Unix(timestamp, 0),
			ExecutableHash:    hash,
			ExecutablePath:    executable,
			ForwarderVersion:  Version,
			Hostname:          hostname,
			IncludeExecutable: sendExecutable,
			Metadata:          s.metadata,
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

		s.logger.Debug("sending core")
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

		// Check if we want to send the executable.
		if !sendExecutable {
			return
		}

		// Send the executable.
		w.Reset(pw)

		s.logger.Debug("sending executable")
		err = s.sendFile(w, executable)
		if err != nil {
			s.logger.Error("sending executable", "err", err)
			return
		}

		err = w.Close()
		if err != nil {
			s.logger.Error("closing executable stream", "err", err)
			return
		}
	}()

	// Send the request by giving it the reader end of the pipe.
	s.logger.Debug("sending request")
	res, err := http.Post(fmt.Sprintf("%s/cores", s.dest), "application/octet-stream", pr)
	if err != nil {
		s.logger.Error("sending core", "err", err)
		return
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()

	s.logger.Debug("received response")
	if res.StatusCode != http.StatusOK {
		var err rcoredump.Error
		_ = json.NewDecoder(res.Body).Decode(&err)
		s.logger.Error("unexpected status", "err", err.Err)
		return
	}

	s.logger.Debug("done")
}

func (s *service) hashExecutable(path string) (string, error) {
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

func (s *service) lookupExecutable(hash string) (bool, error) {
	res, err := http.Head(fmt.Sprintf("%s/executables/%s", s.dest, hash))
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
