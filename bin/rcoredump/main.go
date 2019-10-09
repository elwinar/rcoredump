package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elwinar/rcoredump"
)

func main() {
	var cfg struct {
		dest string
		src  string
	}
	flag.StringVar(&cfg.dest, "dest", "localhost:1105", "address of the destination host")
	flag.StringVar(&cfg.src, "src", "-", "path of the coredump to send to the host ('-' for stdin)")
	flag.Parse()

	// Gather a few variables.
	// Args from the command line should be, in order:
	// - %E, pathname of executable
	// - %t, time of dump
	// - %h, hostname
	args := flag.Args()
	if len(args) != 3 {
		log.Println("unexpected number of arguments on command-line")
		os.Exit(1)
	}

	// Pathname of the executable comes up with ! instead of /.
	executable := strings.Replace(args[0], "!", "/", -1)
	timestamp, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		log.Println("invalid timestamp format")
		os.Exit(1)
	}
	hostname := args[2]

	// Open the connection to the backend.
	conn, err := net.Dial("tcp", cfg.dest)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer conn.Close()

	// Find the input data, either stdin or a file, and get a reader for
	// it.
	var in io.ReadCloser
	if cfg.src == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(cfg.src)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		defer in.Close()
	}

	err = json.NewEncoder(conn).Encode(rcoredump.Header{
		Executable: executable,
		Date:       time.Unix(timestamp, 0),
		Hostname:   hostname,
	})
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Compress the data before sending it.
	w := gzip.NewWriter(conn)
	defer w.Close()

	// Send the input data to in the stream.
	n, err := io.Copy(w, in)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// All done, k-thx-bye.
	log.Println("sent", n, "bytes to", conn.RemoteAddr().String())
}
