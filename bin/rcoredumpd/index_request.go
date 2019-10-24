package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve"
	"github.com/elwinar/rcoredump"
	"github.com/inconshreveable/log15"
	"github.com/rs/xid"
)

type indexRequest struct {
	r     *http.Request
	log   log15.Logger
	dir   string
	index bleve.Index

	uid    string
	body   *bufio.Reader
	reader *gzip.Reader
	header rcoredump.Header

	err error
}

func newIndexRequest(r *http.Request, log log15.Logger, dir string, index bleve.Index) *indexRequest {
	req := &indexRequest{
		r:     r,
		dir:   dir,
		index: index,
	}

	req.uid = xid.New().String()
	req.log = log.New("uid", req.uid)
	req.body = bufio.NewReader(r.Body)

	return req
}

func (r *indexRequest) close() {
	if r.reader != nil {
		r.reader.Close()
	}
	io.Copy(ioutil.Discard, r.r.Body)
	r.r.Body.Close()
}

func (r *indexRequest) prepareReader() {
	reader, err := gzip.NewReader(r.body)
	if err != nil {
		r.err = wrap(err, "preparing gzip decoding")
		return
	}
	r.reader = reader
	r.reader.Multistream(false)
}

func (r *indexRequest) resetReader() {
	if r.err != nil {
		return
	}

	err := r.reader.Reset(r.body)
	if err != nil {
		r.err = wrap(err, "reseting gzip reader")
		return
	}
	r.reader.Multistream(false)
}

func (r *indexRequest) readHeader() {
	r.prepareReader()
	if r.err != nil {
		return
	}
	r.log.Debug("reading header")

	f, err := os.Create(filepath.Join(r.dir, fmt.Sprintf("%s.json", r.uid)))
	if err != nil {
		r.err = wrap(err, "creating header file")
		return
	}
	defer f.Close()

	buf := &bytes.Buffer{}

	_, err = io.Copy(io.MultiWriter(f, buf), r.reader)
	if err != nil {
		r.err = wrap(err, "reading header")
		return
	}

	err = json.NewDecoder(buf).Decode(&r.header)
	if err != nil {
		r.err = wrap(err, "parsing header")
		return
	}
}

func (r *indexRequest) readCore() {
	r.resetReader()
	if r.err != nil {
		return
	}
	r.log.Debug("reading core")

	f, err := os.Create(filepath.Join(r.dir, fmt.Sprintf("%s.core", r.uid)))
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

func (r *indexRequest) readBinary() {
	r.resetReader()
	if r.err != nil {
		return
	}
	r.log.Debug("reading binary")

	f, err := os.Create(filepath.Join(r.dir, fmt.Sprintf("%s.bin", r.uid)))
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

func (r *indexRequest) indexCore() {
	if r.err != nil {
		return
	}
	r.log.Debug("indexing core")

	err := r.index.Index(r.uid, r.header)
	if err != nil {
		r.err = wrap(err, "indexing core")
		return
	}
}
