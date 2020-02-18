package main

import (
	"debug/elf"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/blevesearch/bleve"
	"github.com/elwinar/rcoredump"
	"github.com/inconshreveable/log15"
)

type analyzeProcess struct {
	log log15.Logger
	uid string

	core rcoredump.Coredump
	elf  *elf.File
	err  error
}

func newAnalyzeProcess(uid string, log log15.Logger) *analyzeProcess {
	return &analyzeProcess{
		uid: uid,
		log: log.New("uid", uid),
	}
}

// findCore do a search on the coredump index so we can get additionnal info on
// the binary and have a document to complete afterwards. (Bleve doesn't do
// partial updates.)
func (p *analyzeProcess) findCore(i bleve.Index) {
	req := bleve.NewSearchRequest(bleve.NewDocIDQuery([]string{p.uid}))
	req.Fields = []string{"*"}
	res, err := i.Search(req)
	if err != nil {
		p.err = wrap(err, "finding indexed core")
		return
	}

	if len(res.Hits) == 0 {
		p.err = wrap(errors.New(`not found`), "finding indexed core")
		return
	}

	raw, err := json.Marshal(res.Hits[0].Fields)
	if err != nil {
		p.err = wrap(err, "parsing indexed core")
		return
	}

	err = json.Unmarshal(raw, &p.core)
	if err != nil {
		p.err = wrap(err, "parsing indexed core")
		return
	}
}

func (p *analyzeProcess) clean() {
	if p.elf != nil {
		p.elf.Close()
	}
}

func (p *analyzeProcess) loadELF(path string) {
	if p.err != nil {
		return
	}

	p.log.Debug("loading binary", "dir", path)
	file, err := elf.Open(filepath.Join(path, p.core.BinaryHash))
	if err != nil {
		p.err = wrap(err, `opening core file: %w`)
		return
	}
	p.elf = file
}

// detectLanguage looks at a binary file's sections to guess which language did
// generate the binary.
//
// Note: the feature is rough, and probably simplist. I don't really care for
// now, because we only want to distinguish C from Go, and this is enough for
// this (Go's routines makes stack traces a little different). This could
// change any moment when we need something more complex.
func (p *analyzeProcess) detectLanguage() {
	if p.err != nil {
		return
	}

	p.log.Debug("detecting language")
	p.core.Lang = rcoredump.LangC
	for _, section := range p.elf.Sections {
		if section.Name == ".go.buildinfo" {
			p.core.Lang = rcoredump.LangGo
			break
		}
	}
	p.log.Debug("detected language", "lang", p.core.Lang)
}

func (p *analyzeProcess) indexResults(i bleve.Index) {
	if p.err != nil {
		return
	}

	p.core.Analyzed = true
	p.log.Debug("indexing analysis result", "result", p.core)
	err := i.Index(p.uid, p.core)
	if err != nil {
		p.err = wrap(err, "indexing results")
		return
	}
}
