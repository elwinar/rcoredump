package rcoredump

import (
	"time"
)

// IndexRequest is the struct expected by the index endpoint.
type IndexRequest struct {
	// Path to the executable on the origin host.
	ExecutablePath string `json:"executable_path"`

	// Date the core dump was generated.
	Date time.Time `json:"date"`

	// Hostname of the origin host.
	Hostname string `json:"hostname"`

	// Hash of the binary that generated the core dump.
	BinaryHash string `json:"binary_hash,omitempty"`

	// Does the request body include the binary?
	IncludeBinary bool `json:"include_binary,omitempty"`
}

// Coredump as indexed by the server.
type Coredump struct {
	UID            string    `json:"uid"`
	Date           time.Time `json:"date"`
	Hostname       string    `json:"hostname"`
	ExecutablePath string    `json:"executable_path"`
	BinaryHash     string    `json:"binary_hash"`
	Analyzed       bool      `json:"analyzed"`
}

// Error type for API return values.
type Error struct {
	Err string `json:"error"`
}
