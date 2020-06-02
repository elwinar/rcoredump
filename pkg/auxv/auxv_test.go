package auxv

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/elwinar/rcoredump/pkg/testingx"
	"github.com/google/go-cmp/cmp"
)

func TestVector_ReadFrom(t *testing.T) {
	const path = `testdata/auxv`

	vector := New()
	err := vector.ReadFrom(testingx.Open(t, `auxv`))
	if err != nil {
		t.Errorf(`Vector.ReadFrom(%q): unexpected error: %s`, path, err)
		return
	}

	var expected Vector
	testingx.GoldenJSON(t, `auxv.golden.json`, vector, &expected)

	if !cmp.Equal(vector, expected) {
		t.Errorf(`Vector.ReadFrom(%q): unexpected result`, path)
		t.Log(cmp.Diff(vector, expected))
	}
}

func TestReadString(t *testing.T) {
	type testcase struct {
		str  []byte
		want string
	}

	for n, c := range map[string]testcase{
		"nominal":            testcase{str: []byte("hello world\000"), want: "hello world"},
		"unterminated":       testcase{str: []byte("hello world"), want: "hello world"},
		"empty":              testcase{str: []byte{0}, want: ""},
		"unterminated empty": testcase{str: []byte{}, want: ""},
	} {
		t.Run(n, func(t *testing.T) {
			hdr := (*reflect.StringHeader)(unsafe.Pointer(&c.str))
			out := ReadString(hdr.Data)

			if out != c.want {
				t.Errorf(`ReadString(%p): wanted %q, got %q`, c.str, c.want, out)
			}
		})
	}
}
