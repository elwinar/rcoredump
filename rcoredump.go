package rcoredump

import "time"

type Header struct {
	Executable string    `json:"executable"`
	Date       time.Time `json:"date"`
	Hostname   string    `json:"hostname"`
}
