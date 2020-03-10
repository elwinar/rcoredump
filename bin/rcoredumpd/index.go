package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blevesearch/bleve"
)

type Index interface {
	Find(string) (Coredump, error)
	FindUnanalyzed() ([]string, error)
	Index(Coredump) error
	Lookup(string) (bool, error)
	Search(string, string, int) ([]Coredump, error)
}

var (
	ErrNotFound = errors.New(`not found`)
)

func NewBleveIndex(path string) (Index, error) {
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, wrap(err, `checking for index`)
	}

	var index bleve.Index
	if errors.Is(err, os.ErrNotExist) {
		index, err = bleve.New(path, bleve.NewIndexMapping())
	} else {
		index, err = bleve.Open(path)
	}
	if err != nil {
		return nil, wrap(err, `opening index`)
	}

	return BleveIndex{i: index}, nil
}

type BleveIndex struct {
	i bleve.Index
}

func (i BleveIndex) Find(uid string) (c Coredump, err error) {
	req := bleve.NewSearchRequest(bleve.NewDocIDQuery([]string{uid}))
	req.Fields = []string{"*"}

	res, err := i.i.Search(req)
	if err != nil {
		return c, wrap(err, `looking for coredump`)
	}

	if len(res.Hits) == 0 {
		return c, ErrNotFound
	}

	raw, err := json.Marshal(res.Hits[0].Fields)
	if err != nil {
		return c, wrap(err, `marshaling result`)
	}

	err = json.Unmarshal(raw, &c)
	if err != nil {
		return c, wrap(err, `parsing result`)
	}

	return c, nil
}

func (i BleveIndex) FindUnanalyzed() ([]string, error) {
	query := bleve.NewBoolFieldQuery(false)
	query.SetField("analyzed")

	req := bleve.NewSearchRequest(query)
	req.Fields = []string{"uid"}

	res, err := i.i.Search(req)
	if err != nil {
		return nil, wrap(err, `searching for unanalyzed coredumps`)
	}

	if len(res.Hits) == 0 {
		return nil, nil
	}

	var uids []string
	for _, d := range res.Hits {
		uid, ok := d.Fields["uid"].(string)
		if !ok {
			return nil, fmt.Errorf(`invalid uid type %T`, d.Fields["uid"])
		}
		uids = append(uids, uid)
	}
	return uids, nil
}

func (i BleveIndex) Index(c Coredump) error {
	return i.i.Index(c.UID, c)
}

func (i BleveIndex) Lookup(uid string) (exists bool, err error) {
	d, err := i.i.Document(uid)
	if err != nil {
		return false, wrap(err, `looking for coredump`)
	}

	return d != nil, nil
}

func (i BleveIndex) Search(q, sort string, size int) (cores []Coredump, err error) {
	req := bleve.NewSearchRequest(bleve.NewQueryStringQuery(q))
	req.Fields = []string{"*"}
	req.SortBy(strings.Split(sort, ","))

	res, err := i.i.Search(req)
	if err != nil {
		return nil, wrap(err, `searching for coredumps`)
	}

	for _, d := range res.Hits {
		raw, err := json.Marshal(d.Fields)
		if err != nil {
			return nil, wrap(err, `marshaling result`)
		}

		var c Coredump
		err = json.Unmarshal(raw, &c)
		if err != nil {
			return nil, wrap(err, `parsing result`)
		}

		cores = append(cores, c)
	}

	return cores, nil
}
