package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/elwinar/rcoredump"
)

func main() {
	var cfg struct {
		bind string
		dir  string
	}
	flag.StringVar(&cfg.bind, "bind", "localhost:1105", "address to listen to")
	flag.StringVar(&cfg.dir, "dir", "/var/lib/rcoredumpd/", "path of the directory to store the coredumps into")
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

		// Handle the connection.
		func() {
			// Read the header struct.
			var header rcoredump.Header
			err := json.NewDecoder(conn).Decode(&header)
			if err != nil {
				log.Println(err)
				return
			}

			log.Println("receiving dump for", header.Executable, "from", header.Hostname, "at", header.Date.String())

			// Create a temporary file to dump the connection's
			// content in.
			f, err := ioutil.TempFile(cfg.dir, "*.gz")
			if err != nil {
				log.Println(err)
				return
			}
			defer f.Close()

			// Do the actual dumping.
			n, err := io.Copy(f, conn)
			if err != nil {
				log.Println(err)
				return
			}

			log.Println("received", n, "bytes in", f.Name())
		}()

		// Close it.
		conn.Close()
	}

	// All done, k-thx-bye.
}
