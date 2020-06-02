package testingx

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var Update bool

func init() {
	flag.BoolVar(&Update, "update", false, "update the golden files")
}

func Open(t *testing.T, path string) *os.File {
	f, err := os.Open(filepath.Join(`testdata`, path))
	if err != nil {
		t.Fatalf(`opening file %q: %s`, path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func ReadFile(t *testing.T, path string) []byte {
	raw, err := ioutil.ReadFile(filepath.Join(`testdata`, path))
	if err != nil {
		t.Fatalf(`reading file %q: %s`, path, err)
	}
	return raw
}

func WriteFile(t *testing.T, path string, raw []byte) {
	err := ioutil.WriteFile(filepath.Join(`testdata`, path), raw, os.ModePerm)
	if err != nil {
		t.Fatalf(`writing file %q: %s`, path, err)
	}
}

func UnmarshalJSON(t *testing.T, raw []byte, dest interface{}) {
	err := json.Unmarshal(raw, dest)
	if err != nil {
		t.Fatalf(`unmarshaling: %s`, err)
	}
}

func MarshalJSON(t *testing.T, src interface{}) []byte {
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf(`marshaling: %s`, src)
	}
	return raw
}

func Golden(t *testing.T, path string, out []byte) []byte {
	if Update {
		WriteFile(t, path, out)
	}
	return ReadFile(t, path)
}

func GoldenJSON(t *testing.T, path string, out, dest interface{}) {
	if Update {
		WriteFile(t, path, MarshalJSON(t, out))
	}
	UnmarshalJSON(t, ReadFile(t, path), dest)
}
