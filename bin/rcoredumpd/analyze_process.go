package main

import (
	"bytes"
	"debug/elf"
	"html/template"
	"os"
	"os/exec"
	"strings"

	"github.com/elwinar/rcoredump"
	"github.com/inconshreveable/log15"
)

type analyzeProcess struct {
	analyzers map[string]*template.Template
	dataDir   string
	index     Index
	log       log15.Logger
	store     Store
	uid       string

	core       Coredump
	err        error
	file       *os.File
	executable *os.File
}

// init the process by finding the index core and the associated files.
func (p *analyzeProcess) init() {
	if p.err != nil {
		return
	}

	var err error
	p.core, err = p.index.Find(p.uid)
	if err != nil {
		p.err = wrap(err, "finding indexed core")
		return
	}

	p.executable, err = p.store.Executable(p.core.ExecutableHash)
	if err != nil {
		p.err = wrap(err, `opening core file`)
		return
	}

	p.file, err = p.store.Core(p.uid)
	if err != nil {
		p.err = wrap(err, `opening executable file`)
	}
}

func (p *analyzeProcess) cleanup() {
	if p.executable != nil {
		p.executable.Close()
	}

	if p.file != nil {
		p.file.Close()
	}
}

// computeSizes retrieve the size of both the executable and the core file.
func (p *analyzeProcess) computeSizes() {
	if p.err != nil {
		return
	}

	cinfo, err := os.Stat(p.file.Name())
	if err != nil {
		p.err = wrap(err, `sizing core file`)
		return
	}
	p.core.Size = cinfo.Size()

	einfo, err := os.Stat(p.executable.Name())
	if err != nil {
		p.err = wrap(err, `sizing executable file`)
		return
	}
	p.core.ExecutableSize = einfo.Size()
}

// detectLanguage looks at an executable file's sections to guess which
// language did generate the executable.
//
// Note: the feature is rough, and probably simplist. I don't really care for
// now, because we only want to distinguish C from Go, and this is enough for
// this (Go's routines makes stack traces a little different). This could
// change any moment when we need something more complex.
func (p *analyzeProcess) detectLanguage() {
	if p.err != nil {
		return
	}

	p.log.Debug("loading executable", "path", p.executable.Name())
	file, err := elf.NewFile(p.executable)
	if err != nil {
		p.err = wrap(err, `opening executable file`)
		return
	}
	defer file.Close()

	p.log.Debug("detecting language")
	p.core.Lang = rcoredump.LangC
	for _, section := range file.Sections {
		if section.Name == ".go.buildinfo" {
			p.core.Lang = rcoredump.LangGo
			break
		}
	}
	p.log.Debug("detected language", "lang", p.core.Lang)
}

// extractStackTrace shell out to configuration-defined command to delegate the
// task of extracting the stack trace itself and any information judged
// interesting to index.
func (p *analyzeProcess) extractStackTrace() {
	if p.err != nil {
		return
	}

	tpl, ok := p.analyzers[p.core.Lang]
	if !ok {
		p.log.Warn("no trace analyzer for language", "lang", p.core.Lang)
		return
	}

	var buf bytes.Buffer
	err := tpl.Execute(&buf, map[string]string{
		"Core":       p.file.Name(),
		"DataDir":    p.dataDir,
		"Executable": p.executable.Name(),
	})
	p.log.Debug("extracting stack trace")

	cmd := strings.Split(buf.String(), " ")
	out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		p.err = wrap(err, "extracting stack trace: %s", string(out))
		return
	}

	p.core.Trace = string(out)
	p.log.Debug("extracted stack trace")
}

func (p *analyzeProcess) indexResults() {
	if p.err != nil {
		return
	}

	p.core.Analyzed = true
	p.log.Debug("indexing analysis result")
	err := p.index.Index(p.core)
	if err != nil {
		p.err = wrap(err, "indexing results")
		return
	}
}
