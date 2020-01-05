package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/blevesearch/bleve"
	"github.com/elwinar/rcoredump"
	"github.com/inconshreveable/log15"
	"github.com/rs/xid"
)

type indexRequest struct {
	rcoredump.IndexRequest
	r    *http.Request
	log  log15.Logger
	uid  string
	body *bufio.Reader

	err    error
	reader *gzip.Reader
}

func newIndexRequest(r *http.Request, log log15.Logger) *indexRequest {
	uid := xid.New().String()
	return &indexRequest{
		r:    r,
		log:  log.New("uid", uid),
		uid:  uid,
		body: bufio.NewReader(r.Body),
	}
}

func (r *indexRequest) close() {
	if r.reader != nil {
		r.reader.Close()
	}
	io.Copy(ioutil.Discard, r.r.Body)
	r.r.Body.Close()
}

func (r *indexRequest) prepareReader() error {
	var err error
	if r.reader == nil {
		r.reader, err = gzip.NewReader(r.body)
	} else {
		err = r.reader.Reset(r.body)
	}
	if err != nil {
		return err
	}
	r.reader.Multistream(false)
	return nil
}

func (r *indexRequest) read() {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	err = json.NewDecoder(r.reader).Decode(&r.IndexRequest)
	if err != nil {
		r.err = wrap(err, "parsing header")
		return
	}
}

func (r *indexRequest) readCore(path string) {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	f, err := os.Create(path)
	if err != nil {
		r.err = wrap(err, "creating core file")
		return
	}
	defer f.Close()

	_, err = io.Copy(f, r.reader)
	if err != nil {
		r.err = wrap(err, "reading core")
		return
	}
}

func (r *indexRequest) readBinary(path string) {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	f, err := os.Create(path)
	if err != nil {
		r.err = wrap(err, "creating binary file")
		return
	}
	defer f.Close()

	_, err = io.Copy(f, r.reader)
	if err != nil {
		r.err = wrap(err, "writing binary file")
		return
	}
}

func (r *indexRequest) indexCore(i bleve.Index) {
	if r.err != nil {
		return
	}

	err := i.Index(r.uid, rcoredump.Coredump{
		UID:            r.uid,
		Date:           r.Date,
		Hostname:       r.Hostname,
		ExecutablePath: r.ExecutablePath,
		BinaryHash:     r.BinaryHash,
		Analyzed:       false,
	})
	if err != nil {
		r.err = wrap(err, "indexing core")
		return
	}
}
