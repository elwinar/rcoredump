// testingx contains testing helpers meant to simplify unit testing. Most of
// the helpers are simple wrapper for other libraries functions with a few
// tweaks meant to simplify the unit tests:
// - they don't return an error and instead fail the test,
// - relative filepath are prefixed by testdata/,
// - resources like file handles are closed during test cleanup;
package testingx

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// updateGoldenFlag indicates tests to udpate their golden files with the
// expected output. This flag is controled by the -updategolden flag and will
// apply to every call to GoldenXXX, one is expected to use the -run flag to
// limit to specific tests.
var updateGolden bool

func init() {
	flag.BoolVar(&updateGolden, "updategolden", false, "update the golden files")
}

// Open the file at path and return the handle.
func Open(t *testing.T, path string) *os.File {
	t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(`testdata`, path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf(`opening file %q: %s`, path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// ReadFile returns the content of the file at path. See ioutil.ReadFile.
func ReadFile(t *testing.T, path string) []byte {
	t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(`testdata`, path)
	}
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf(`reading file %q: %s`, path, err)
	}
	return raw
}

// WriteFile set the content of the file at path. See ioutil.WriteFile.
func WriteFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(`testdata`, path)
	}
	err := ioutil.WriteFile(path, raw, os.ModePerm)
	if err != nil {
		t.Fatalf(`writing file %q: %s`, path, err)
	}
}

// UnmarshalJSON parse the JSON raw string into dest. See json.Unmarshal.
func UnmarshalJSON(t *testing.T, raw []byte, dest interface{}) {
	err := json.Unmarshal(raw, dest)
	if err != nil {
		t.Fatalf(`unmarshaling: %s`, err)
	}
}

// MarshalJSON encode src into a JSON string. See json.Marshal.
func MarshalJSON(t *testing.T, src interface{}) []byte {
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf(`marshaling: %s`, src)
	}
	return raw
}

// Golden returns the content of the file at path, eventually changing them to
// out beforehand if the -goldenupdate flag was set to true on the command
// line. See ReadFile and WriteFile.
func Golden(t *testing.T, path string, out []byte) []byte {
	if updateGolden {
		WriteFile(t, path, out)
	}
	return ReadFile(t, path)
}

// GoldenJSON is like Golden, but keep a JSON representation of the given
// structs into the file at path.
func GoldenJSON(t *testing.T, path string, out, dest interface{}) {
	if updateGolden {
		WriteFile(t, path, MarshalJSON(t, out))
	}
	UnmarshalJSON(t, ReadFile(t, path), dest)
}
