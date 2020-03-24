package rcoredump

import (
	"time"
)

// IndexRequest is the struct expected by the index endpoint.
type IndexRequest struct {
	// Date the core dump was generated.
	Date time.Time `json:"date"`
	// Hostname of the origin host.
	Hostname string `json:"hostname"`
	// Does the request body include the executable?
	IncludeExecutable bool `json:"include_executable,omitempty"`
	// Hash of the executable that generated the core dump.
	ExecutableHash string `json:"executable_hash,omitempty"`
	// Path to the executable on the origin host.
	ExecutablePath string `json:"executable_path"`
	// Metadata set by the forwarder configuration.
	Metadata map[string]string `json:"metadata"`
	// Version of the forwarder that sent the coredump.
	ForwarderVersion string `json:"forwarder_version"`
}

// Coredump as indexed by the server.
type Coredump struct {
	Analyzed         bool              `json:"analyzed"`
	Date             time.Time         `json:"date"`
	ExecutableHash   string            `json:"executable_hash"`
	ExecutablePath   string            `json:"executable_path"`
	ExecutableSize   int64             `json:"executable_size"`
	ForwarderVersion string            `json:"forwarder_version"`
	Hostname         string            `json:"hostname"`
	IndexerVersion   string            `json:"indexer_version"`
	Lang             string            `json:"lang"`
	Metadata         map[string]string `json:"metadata"`
	Size             int64             `json:"size"`
	Trace            string            `json:"trace"`
	UID              string            `json:"uid"`
}

// Error type for API return values.
type Error struct {
	Err string `json:"error"`
}

const (
	LangC  = "C"
	LangGo = "Go"
)
