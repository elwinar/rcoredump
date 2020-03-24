package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blevesearch/bleve"
	structmapper "gopkg.in/anexia-it/go-structmapper.v1"
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

type BleveIndex struct {
	// the index is the actual struct we are interfacing with.
	index bleve.Index

	// the mapper is used to convert between the Coredump struct itself and
	// the map[string]interface{} used internally by the bleve index. The
	// issue is that the types of fields allowed by bleve are quite
	// limited, for example Metadata, so we need to fake it using meta.x
	// fields. In addition, this allows searching on those fields, which
	// isn't possible by default.
	mapper *structmapper.Mapper
}

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

	// Initialize the structmapper to use the JSON tag. This avoid having
	// to re-define every field with yet another tag.
	mapper, err := structmapper.NewMapper(structmapper.OptionTagName("json"))
	if err != nil {
		return nil, wrap(err, `initializing mapper`)
	}

	return BleveIndex{
		index:  index,
		mapper: mapper,
	}, nil
}

func (i BleveIndex) Find(uid string) (c Coredump, err error) {
	req := bleve.NewSearchRequest(bleve.NewDocIDQuery([]string{uid}))
	req.Fields = []string{"*"}

	res, err := i.index.Search(req)
	if err != nil {
		return c, wrap(err, `looking for coredump`)
	}

	if len(res.Hits) == 0 {
		return c, ErrNotFound
	}

	err = i.mapper.ToStruct(res.Hits[0].Fields, &c)
	if err != nil {
		return c, wrap(err, `mapping result to coredump`)
	}

	c.Metadata = make(map[string]string)
	for k, v := range res.Hits[0].Fields {
		if !strings.HasPrefix(k, "meta.") {
			continue
		}
		if _, ok := v.(string); !ok {
			return c, fmt.Errorf(`unexpected type for metadata value %s in core %s: %T`, k, c.UID, v)
		}
		c.Metadata[strings.TrimPrefix(k, "meta.")] = v.(string)
	}

	return c, nil
}

func (i BleveIndex) FindUnanalyzed() ([]string, error) {
	query := bleve.NewBoolFieldQuery(false)
	query.SetField("analyzed")

	req := bleve.NewSearchRequest(query)
	req.Fields = []string{"uid"}

	res, err := i.index.Search(req)
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
	m, err := i.mapper.ToMap(c)
	if err != nil {
		return wrap(err, `mapping Coredump`)
	}

	for k, v := range c.Metadata {
		m[fmt.Sprintf("meta.%s", k)] = v
	}

	return i.index.Index(c.UID, m)
}

func (i BleveIndex) Lookup(uid string) (exists bool, err error) {
	d, err := i.index.Document(uid)
	if err != nil {
		return false, wrap(err, `looking for coredump`)
	}

	return d != nil, nil
}

func (i BleveIndex) Search(q, sort string, size int) (cores []Coredump, err error) {
	req := bleve.NewSearchRequest(bleve.NewQueryStringQuery(q))
	req.Fields = []string{"*"}
	req.SortBy(strings.Split(sort, ","))

	res, err := i.index.Search(req)
	if err != nil {
		return nil, wrap(err, `searching for coredumps`)
	}

	for _, d := range res.Hits {
		var c Coredump

		err := i.mapper.ToStruct(d.Fields, &c)
		if err != nil {
			return nil, wrap(err, `mapping to coredump`)
		}

		c.Metadata = make(map[string]string)
		for k, v := range d.Fields {
			if !strings.HasPrefix(k, "meta.") {
				continue
			}
			if _, ok := v.(string); !ok {
				return nil, fmt.Errorf(`unexpected type for metadata value %s in core %s: %T`, k, c.UID, v)
			}
			c.Metadata[strings.TrimPrefix(k, "meta.")] = v.(string)
		}

		cores = append(cores, c)
	}

	return cores, nil
}
