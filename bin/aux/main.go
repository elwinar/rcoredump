package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"
)

func main() {
	f, err := os.Open("/proc/self/auxv")
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		var id AuxType
		err = binary.Read(f, binary.LittleEndian, &id)
		if err != nil {
			break
		}

		if id != AuxTypePlatform {
			f.Seek(4, 1)
			continue
		}

		platform, err := readAuxString(f, binary.LittleEndian)
		if err != nil {
			fmt.Println("reading platform:", err)
			return
		}

		fmt.Println(id.String(), platform)
		break
	}
}

//go:generate stringer -type AuxType
type AuxType uint64

const (
	// AuxTypePlatform marks a string identifying the platform.
	AuxTypePlatform AuxType = 15
)

func readAuxString(r io.Reader, order binary.ByteOrder) (string, error) {
	var raw uint64
	err := binary.Read(r, order, &raw)
	if err != nil {
		return "", fmt.Errorf(`reading string pointer: %w`, err)
	}

	var buf bytes.Buffer
	var ptr = uintptr(raw)
	for {
		var b = *(*byte)(unsafe.Pointer(ptr))
		if b == 0 {
			break
		}
		buf.WriteByte(b)
		ptr += 1
	}

	return buf.String(), nil
}
