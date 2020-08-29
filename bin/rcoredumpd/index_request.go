package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/elwinar/rcoredump/pkg/rcoredump"

	"github.com/inconshreveable/log15"
	"github.com/rs/xid"
)

type indexRequest struct {
	log   log15.Logger
	r     *http.Request
	index Index
	store Store

	err      error
	uid      string
	body     *bufio.Reader
	reader   *gzip.Reader
	req      IndexRequest
	coredump Coredump
}

func (r *indexRequest) init() {
	r.uid = xid.New().String()
	r.log = r.log.New("uid", r.uid)
	r.body = bufio.NewReader(r.r.Body)
	r.coredump = Coredump{
		IndexerVersion: Version,
		UID:            r.uid,
	}
}

func (r *indexRequest) close() {
	if r.reader != nil {
		r.reader.Close()
	}

	_, _ = io.Copy(ioutil.Discard, r.r.Body)

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

	err = json.NewDecoder(r.reader).Decode(&r.req)
	if err != nil {
		r.err = wrap(err, "parsing header")
		return
	}

	r.coredump.DumpedAt = r.req.DumpedAt
	r.coredump.Executable = filepath.Base(r.req.ExecutablePath)
	r.coredump.ExecutableHash = r.req.ExecutableHash
	r.coredump.ExecutablePath = r.req.ExecutablePath
	r.coredump.ForwarderVersion = r.req.ForwarderVersion
	r.coredump.Hostname = r.req.Hostname
	r.coredump.Metadata = r.req.Metadata
	r.coredump.Links = r.req.Links
}

func (r *indexRequest) readCore() {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	r.coredump.Size, r.err = r.store.StoreCore(r.uid, r.reader)
}

func (r *indexRequest) readExecutable() {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	r.coredump.ExecutableSize, r.err = r.store.StoreExecutable(r.req.ExecutableHash, r.reader)
}

func (r *indexRequest) readLink(i int) {
	if r.err != nil {
		return
	}

	link := r.req.Links[i]
	if len(link.Error) != 0 || !link.Found {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	_, r.err = r.store.StoreLink(r.req.ExecutableHash, link.Name, r.reader)
}

// computeExecutableSize is used if the executable wasn't sent by the forwarder
// because it already exists.
func (r *indexRequest) computeExecutableSize() {
	if r.err != nil {
		return
	}

	// We open the real file, this also ensure the file is available.
	executable, err := r.store.Executable(r.req.ExecutableHash)
	if err != nil {
		r.err = wrap(err, "opening executable file")
		return
	}
	defer executable.Close()

	info, err := os.Stat(executable.Name())
	if err != nil {
		r.err = wrap(err, "getting executable size")
		return
	}
	r.coredump.ExecutableSize = info.Size()
}

func (r *indexRequest) indexCore() {
	if r.err != nil {
		return
	}

	err := r.index.Index(r.coredump)
	if err != nil {
		r.err = wrap(err, "indexing core")
		return
	}
}
