package auxv

import (
	"encoding/binary"
	"io"
)

// Word is the type used by the auxilliary vector for both the key and values
// of the vector's pairs.
type Word uint64

// ReadFrom reads an auxilliary vector value from r.
func (w *Word) ReadFrom(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, w)
}
