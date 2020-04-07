package main

import (
	"fmt"

	"github.com/inconshreveable/log15"
)

type cleanupProcess struct {
	index Index
	log   log15.Logger
	store Store
	core  Coredump

	err error
}

func (p *cleanupProcess) cleanIndex() {
	if p.err != nil {
		return
	}

	p.log.Debug("cleaning index")
	err := p.index.Delete(p.core.UID)
	if err != nil {
		p.err = wrap(err, `removing indexed document`)
		return
	}
}

func (p *cleanupProcess) cleanStore() {
	if p.err != nil {
		return
	}

	p.log.Debug("cleaning store")
	err := p.store.DeleteCore(p.core.UID)
	if err != nil {
		p.err = wrap(err, `removing coredump file`)
		return
	}
}

func (p *cleanupProcess) cleanExecutable() {
	if p.err != nil {
		return
	}

	p.log.Debug("cleaning executable")
	err := p.store.DeleteExecutable(p.core.ExecutableHash)
	if err != nil {
		p.err = wrap(err, `removing executable file`)
		return
	}
}

func (p *cleanupProcess) canCleanExecutable() bool {
	if p.err != nil {
		return false
	}

	_, total, err := p.index.Search(fmt.Sprintf(`executable_hash:"%s"`, p.core.ExecutableHash), "date", "asc", 0, 0)
	if err != nil {
		p.err = wrap(err, `searching for executable's coredumps`)
		return false
	}

	return total == 0
}
