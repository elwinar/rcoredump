package main

import (
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
)

func main() {
	var cfg struct {
		bind string
		dir  string
	}
	flag.StringVar(&cfg.bind, "bind", "localhost:1105", "address to listen to")
	flag.StringVar(&cfg.dir, "dir", "/var/rcoredumpd/", "path of the directory to store the coredumps into")
	flag.Parse()

	// Ensure the output directory exists.
	err := os.Mkdir(cfg.dir, os.ModeDir)
	if !errors.Is(err, os.ErrExist) {
		log.Println(err)
		os.Exit(1)
	}

	// Open the connection to the backend.
	listener, err := net.Listen("tcp", cfg.bind)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer listener.Close()

	log.Println("listening")

	for {
		// Accept a connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		// Create a temporary file to dump the connection's content in.
		f, err := ioutil.TempFile(cfg.dir, "*.gz")
		if err != nil {
			conn.Close()
			log.Println(err)
			continue
		}

		// Do the actual dumping.
		n, err := io.Copy(f, conn)
		if err != nil {
			conn.Close()
			log.Println(err)
			continue
		}

		log.Println("received", n, "bytes from", conn.RemoteAddr().String(), "in", f.Name())

		conn.Close()
		f.Close()
	}

	// All done, k-thx-bye.
}
