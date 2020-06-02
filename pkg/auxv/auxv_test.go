package auxv

import (
	"testing"

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
