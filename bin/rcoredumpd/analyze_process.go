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
	log       log15.Logger
	uid       string
	index     Index
	dir       string
	analyzers map[string]*template.Template

	core rcoredump.Coredump
	err  error
}

func (p *analyzeProcess) init() {
	p.log = p.log.New("uid", p.uid)
}

// findCore do a search on the coredump index so we can get additionnal info on
// the executable and have a document to update.
func (p *analyzeProcess) findCore() {
	var err error
	p.core, err = p.index.Find(p.uid)
	if err != nil {
		p.err = wrap(err, "finding indexed core")
		return
	}
}

// computeSizes retrieve the size of both the executable and the core file.
func (p *analyzeProcess) computeSizes() {
	if p.err != nil {
		return
	}

	corestat, err := os.Stat(corepath(p.dir, p.core.UID))
	if err != nil {
		p.err = wrap(err, `sizing core file`)
		return
	}
	p.core.Size = corestat.Size()

	exestat, err := os.Stat(exepath(p.dir, p.core.ExecutableHash))
	if err != nil {
		p.err = wrap(err, `sizing executable file`)
		return
	}
	p.core.ExecutableSize = exestat.Size()
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

	exefile := exepath(p.dir, p.core.ExecutableHash)
	p.log.Debug("loading executable", "exefile", exefile)
	file, err := elf.Open(exepath(p.dir, p.core.ExecutableHash))
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
		"Executable": exepath(p.dir, p.core.ExecutableHash),
		"Core":       corepath(p.dir, p.core.UID),
		"Dir":        p.dir,
	})
	p.log.Debug("extracting stack trace", "cmd", buf.String())

	cmd := strings.Split(buf.String(), " ")
	out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		p.err = wrap(err, "extracting stack trace: %s", string(out))
		return
	}

	p.core.Trace = string(out)
	p.log.Debug("extracted stack trace", "stack", p.core.Trace)
}

func (p *analyzeProcess) indexResults() {
	if p.err != nil {
		return
	}

	p.core.Analyzed = true
	p.log.Debug("indexing analysis result", "result", p.core)
	err := p.index.Index(p.core)
	if err != nil {
		p.err = wrap(err, "indexing results")
		return
	}
}
