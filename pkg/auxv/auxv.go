package auxv

import (
	"bytes"
	"fmt"
	"io"
	"unsafe"
)

// Type is the key of the auxilliary vector entries. See
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/auxvec.h
// for the complete list of accepted values.
type Type Word

// ReadFrom reads an auxilliary vector key from r.
func (t *Type) ReadFrom(r io.Reader) error {
	var w Word
	err := w.ReadFrom(r)
	if err != nil {
		return err
	}
	*t = Type(w)
	return nil
}

//go:generate stringer -type Type
const (
	TypePlatform Type = 15
)

// Vector is an auxilliary vector, i.e the list of key-value pairs provided by
// the kernel about the environment in which a program is operating.
// See https://www.gnu.org/software/libc/manual/html_node/Auxiliary-Vector.html.
type Vector map[Type]Word

// New initialize a new empty Vector.
func New() Vector {
	return Vector{}
}

// ReadFrom takes an io.Reader and parse the auxilliary vector within it.
func (v Vector) ReadFrom(r io.Reader) (err error) {
	for {
		var t Type
		err = t.ReadFrom(r)
		if err != nil {
			break
		}

		var val Word
		err = val.ReadFrom(r)
		if err != nil {
			return fmt.Errorf(`reading value: %w`, err)
		}

		v[t] = val
	}

	if err == io.EOF {
		return nil
	}

	return err
}

// ReadString takes the address of an in-memory string and read it byte by
// byte, until a termination character is found (the 0 byte).
func ReadString(ptr uintptr) string {
	var buf bytes.Buffer
	for {
		var b = *(*byte)(unsafe.Pointer(ptr))
		if b == 0 {
			break
		}
		buf.WriteByte(b)
		ptr += 1
	}
	return buf.String()
}
