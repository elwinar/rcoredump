package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/elwinar/rcoredump"
	"github.com/rs/xid"
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

		// Handle the connection.
		func() {
			// Accept a connection.
			conn, err := listener.Accept()
			if err != nil {
				log.Println(err)
				return
			}
			defer conn.Close()

			// Generate an UniqueID for this dump.
			uid := xid.New().String()
			log.Println("receiving dump", uid)

			// Uncompress the streams on the fly.
			bconn := bufio.NewReader(conn)
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

			f, err := os.Create(filepath.Join(cfg.dir, fmt.Sprintf("%s.json", uid)))
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

			f, err = os.Create(filepath.Join(cfg.dir, fmt.Sprintf("%s.core", uid)))
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

			f, err = os.Create(filepath.Join(cfg.dir, fmt.Sprintf("%s.bin", uid)))
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
		}()
	}

	// All done, k-thx-bye.
}
