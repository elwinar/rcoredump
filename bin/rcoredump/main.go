package main

import (
	"compress/gzip"
	"flag"
	"io"
	"log"
	"net"
	"os"
)

func main() {
	var cfg struct {
		dest string
		src  string
	}
	flag.StringVar(&cfg.dest, "dest", "localhost:1105", "address of the destination host")
	flag.StringVar(&cfg.src, "src", "-", "path of the coredump to send to the host ('-' for stdin)")
	flag.Parse()

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
