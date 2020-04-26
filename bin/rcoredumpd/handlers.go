package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/c2h5oh/datasize"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
)

func (s *service) root(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	rw.Write([]byte(s.rootHTML))
}

func (s *service) about(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	write(rw, http.StatusOK, map[string]string{
		"built_at": BuiltAt,
		"commit":   Commit,
		"version":  Version,
	})
}

// indexCore handle the requests for adding cores to the service. It exposes a
// prometheus metric for monitoring its activity, and only deals with storing
// the core and indexing the immutable information about it. Once done, it send
// the UID of the core in the analysis channel for the analyzis routine to pick
// it up.
func (s *service) indexCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	req := &indexRequest{
		index: s.index,
		log:   s.logger,
		r:     r,
		store: s.store,
	}
	req.init()
	req.read()
	req.readCore()
	if req.req.IncludeExecutable {
		req.readExecutable()
	} else {
		req.computeExecutableSize()
	}
	req.indexCore()
	req.close()

	if req.err != nil {
		s.logger.Error("indexing", "uid", req.uid, "err", req.err)
		writeError(w, http.StatusInternalServerError, req.err)
		return
	}

	s.received.With(prometheus.Labels{
		"hostname":   req.coredump.Hostname,
		"executable": req.coredump.Executable,
	}).Inc()

	s.receivedSizes.With(prometheus.Labels{
		"hostname":   req.coredump.Hostname,
		"executable": req.coredump.Executable,
	}).Observe(datasize.ByteSize(req.coredump.Size).MBytes())

	s.analysisQueue <- req.coredump

	w.WriteHeader(http.StatusOK)
}

// analyzeCore handle the requests for re-analyzing a particular core. It
// should be useful when new features are implemented to re-analyze already
// existing cores and update them.
func (s *service) analyzeCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	uid := p.ByName("uid")

	c, err := s.index.Find(uid)
	switch err {
	case nil:
		s.analysisQueue <- c
		w.WriteHeader(http.StatusAccepted)
	case ErrNotFound:
		writeError(w, http.StatusBadRequest, errors.New("unknown core"))
	default:
		s.logger.Error("analyzing", "uid", uid, "err", err)
		writeError(w, http.StatusInternalServerError, err)
	}
}

// searchCore handle the requests to search cores matching a number of parameters.
func (s *service) searchCore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var err error

	q := r.FormValue("q")
	if len(q) == 0 {
		q = "*"
	}

	sort := r.FormValue("sort")
	if len(sort) == 0 {
		sort = "dumped_at"
	}
	switch sort {
	case "dumped_at", "hostname":
		break
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid sort field '%s'", sort))
		return
	}

	order := r.FormValue("order")
	if len(order) == 0 {
		order = "desc"
	}
	switch order {
	case "asc", "desc":
		break
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid sort order '%s'", order))
		return
	}

	rawSize := r.FormValue("size")
	if len(rawSize) == 0 {
		rawSize = "50"
	}
	size, err := strconv.Atoi(rawSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, wrap(err, "invalid size parameter"))
		return
	}

	rawFrom := r.FormValue("from")
	if len(rawFrom) == 0 {
		rawFrom = "0"
	}
	from, err := strconv.Atoi(rawFrom)
	if err != nil {
		writeError(w, http.StatusBadRequest, wrap(err, "invalid from parameter"))
		return
	}

	res, total, err := s.index.Search(q, sort, order, size, from)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	write(w, http.StatusOK, SearchResult{Results: res, Total: total})
}

// getCore handles the requests to get the actual core dump file.
func (s *service) getCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := s.store.Core(p.ByName("uid"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	// We ignore the error here, because the zero-value is fine in case of
	// error.
	info, _ := f.Stat()
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// deleteCore handle the request to remove a coredump.
func (s *service) deleteCore(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	uid := p.ByName("uid")

	c, err := s.index.Find(uid)
	switch err {
	case nil:
		s.cleanupQueue <- c
		w.WriteHeader(http.StatusOK)
	case ErrNotFound:
		writeError(w, http.StatusBadRequest, errors.New("unknown core"))
	default:
		s.logger.Error("analyzing", "uid", uid, "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
}

// lookupExecutable handles the requests to check if a executable matching the given
// hash actually exists. It doesn't return anything (except in case of error).
func (s *service) lookupExecutable(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	exists, err := s.store.ExecutableExists(p.ByName("hash"))
	if err != nil {
		s.logger.Warn("looking up executable", "hash", p.ByName("hash"), "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, errors.New(`not found`))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// getExecutable handles the requests to get the actual executable.
func (s *service) getExecutable(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	f, err := s.store.Executable(p.ByName("hash"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	// We ignore the error here, because the zero-value is fine in case of
	// error.
	info, _ := f.Stat()
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
