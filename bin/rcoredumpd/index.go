package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	. "github.com/elwinar/rcoredump/pkg/rcoredump"

	"github.com/blevesearch/bleve"
	structmapper "gopkg.in/anexia-it/go-structmapper.v1"
)

// An Index is a full-text search engine for coredumps.
type Index interface {
	Index(Coredump) error
	Find(string) (Coredump, error)
	Delete(string) error
	Search(string, string, string, int, int) ([]Coredump, uint64, error)
}

var (
	// ErrNotFound is returned when the index can't find the requested
	// coredump.
	ErrNotFound = errors.New(`not found`)
)

// BleveIndex uses the Bleve engine for searching coredumps.
type BleveIndex struct {
	// the index is the actual struct we are interfacing with.
	index bleve.Index

	// the mapper is used to convert between the Coredump struct itself and
	// the map[string]interface{} used internally by the bleve index. The
	// issue is that the types of fields allowed by bleve are quite
	// limited, so we need to fake fields like Metadata using meta.x
	// fields. In addition, this allows searching on those fields, which
	// isn't possible by default.
	mapper *structmapper.Mapper
}

// compile-time check that the BleveIndex actually implements the Index
// interface.
var _ Index = new(BleveIndex)

// NewBleveIndex load an Index using the Bleve engine. The index is created and
// its mapping initialized if needed.
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

func (i BleveIndex) Delete(uid string) error {
	return i.index.Delete(uid)
}

func (i BleveIndex) Search(q, sort, order string, size, from int) (cores []Coredump, total uint64, err error) {
	req := bleve.NewSearchRequest(bleve.NewQueryStringQuery(q))
	req.Fields = []string{"*"}
	req.From = from
	req.Size = size

	if order == "desc" {
		sort = "-" + sort
	}
	req.SortBy([]string{sort})

	res, err := i.index.Search(req)
	if err != nil {
		return nil, 0, wrap(err, `searching for coredumps`)
	}

	for _, d := range res.Hits {
		var c Coredump

		err := i.mapper.ToStruct(d.Fields, &c)
		if err != nil {
			return nil, 0, wrap(err, `mapping to coredump`)
		}

		c.Metadata = make(map[string]string)
		for k, v := range d.Fields {
			if !strings.HasPrefix(k, "meta.") {
				continue
			}
			if _, ok := v.(string); !ok {
				return nil, 0, fmt.Errorf(`unexpected type for metadata value %s in core %s: %T`, k, c.UID, v)
			}
			c.Metadata[strings.TrimPrefix(k, "meta.")] = v.(string)
		}

		cores = append(cores, c)
	}

	return cores, res.Total, nil
}
