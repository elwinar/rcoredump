package elfx

import (
	"debug/elf"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/elwinar/rcoredump/pkg/auxv"
)

func TestSetLibraryPath(t *testing.T) {
	type testcase struct {
		input string
		want  []string
	}

	for n, c := range map[string]testcase{
		"empty": testcase{
			input: "",
			want:  nil,
		},
		"one": testcase{
			input: "one",
			want:  []string{"one"},
		},
		"two": testcase{
			input: "one:two",
			want:  []string{"one", "two"},
		},
		"three": testcase{
			input: "one:two:three",
			want:  []string{"one", "two", "three"},
		},
		"dot": testcase{
			input: ".:one",
			want:  []string{".", "one"},
		},
		"empty path": testcase{
			input: ":one",
			want:  []string{".", "one"},
		},
	} {
		t.Run(n, func(t *testing.T) {
			SetLibraryPath(c.input)

			if !reflect.DeepEqual(LibraryPathDirs, c.want) {
				t.Errorf(`SetLibraryPath(%q): wanted %#v, got %#v`, c.input, c.want, LibraryPathDirs)
			}
		})
	}
}

func TestFile_ResolveImportedLibrary(t *testing.T) {
	type testcase struct {
		executable  string
		libraryDirs []string
		defaultDirs []string
		input       string
		wantPath    string
		wantOK      bool
	}

	for n, c := range map[string]testcase{
		"relative": testcase{
			input:    "./relative.so",
			wantPath: AbsT(t, "testdata/relative.so"),
			wantOK:   true,
		},
		"missing relative": testcase{
			input:    "./missing_relative.so",
			wantPath: AbsT(t, "testdata/missing_relative.so"),
			wantOK:   false,
		},
		"absolute": testcase{
			input:    AbsT(t, "./testdata/absolute.so"),
			wantPath: AbsT(t, "./testdata/absolute.so"),
			wantOK:   true,
		},
		"missing absolute": testcase{
			input:    AbsT(t, "./testdata/missing_absolute.so"),
			wantPath: AbsT(t, "./testdata/missing_absolute.so"),
			wantOK:   false,
		},
		"library in lib": testcase{
			input:    "library_in_lib.so",
			wantPath: AbsT(t, "./testdata/lib/library_in_lib.so"),
			wantOK:   true,
		},
		"library in ld_path": testcase{
			input:    "library_in_ld_path.so",
			wantPath: AbsT(t, "./testdata/ld_library_path/library_in_ld_path.so"),
			wantOK:   true,
		},
		"library in lib64": testcase{
			defaultDirs: []string{"./testdata/$LIB"},
			input:       "library_in_lib64.so",
			wantPath:    "testdata/lib64/library_in_lib64.so",
			wantOK:      true,
		},
		"library in rpath": testcase{
			executable: "./testdata/executable_rpath",
			input:      "library_in_rpath.so",
			wantPath:   "testdata/rpath/library_in_rpath.so",
			wantOK:     true,
		},
		"library in runpath": testcase{
			executable: "./testdata/executable_runpath",
			input:      "library_in_runpath.so",
			wantPath:   "testdata/runpath/library_in_runpath.so",
			wantOK:     true,
		},
		"not found": testcase{
			input:    "missing_library.so",
			wantPath: "missing_library.so",
			wantOK:   false,
		},
	} {
		t.Run(n, func(t *testing.T) {
			// Set some defaults to avoid writing repetitive lines.
			if c.libraryDirs == nil {
				c.libraryDirs = []string{
					AbsT(t, "./testdata/ld_library_path"),
				}
			}
			LibraryPathDirs = c.libraryDirs

			if c.defaultDirs == nil {
				c.defaultDirs = []string{
					AbsT(t, "./testdata/lib"),
					AbsT(t, "./testdata/usr/lib"),
				}
			}
			DefaultDirs = c.defaultDirs

			if c.executable == "" {
				c.executable = "./testdata/executable"
			}

			file, err := Open(c.executable)
			if err != nil {
				t.Fatalf(`ResolveImportedLibrary(%q, %q): opening executable: %s`, c.executable, c.input, err)
			}

			path, ok, err := file.ResolveImportedLibrary(c.input)
			if err != nil {
				t.Fatalf(`ResolveImportedLibrary(%q, %q): unexpected error: %s`, c.executable, c.input, err)
			}

			if path != c.wantPath || ok != c.wantOK {
				t.Errorf(`ResolveImportedLibrary(%q, %q): wanted %q, %t, got %q, %t`, c.executable, c.input, c.wantPath, c.wantOK, path, ok)
			}
		})
	}
}

// AbsT returns an absolute path equivalent to the given path, and fail the
// test in case of error.
func AbsT(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf(`filepath.Abs(%q): %s`, path, err)
	}
	return p
}

func TestFile_Expand(t *testing.T) {
	// We retrieve the auxilliary vector by hand here to ensure we can test
	// with the right value for the platform expansion.
	err := parseAux()
	if err != nil {
		t.Fatalf(`parsing auxilliary vector`)
	}
	v, ok := aux[auxv.TypePlatform]
	if !ok {
		t.Fatalf(`missing platform entry in auxilliary vector`)
	}
	platform := v.ReadString()

	type testcase struct {
		input string
		want  string
	}

	for n, c := range map[string]testcase{
		"origin": testcase{
			input: "$ORIGIN/foo",
			want:  "testdata/foo",
		},
		"curly_origin": testcase{
			input: "${ORIGIN}/foo",
			want:  "testdata/foo",
		},
		"lib": testcase{
			input: "foo/$LIB/bar",
			want:  "foo/lib64/bar",
		},
		"lib_end": testcase{
			input: "foo/$LIB",
			want:  "foo/lib64",
		},
		"curly_lib_end": testcase{
			input: "foo/${LIB}",
			want:  "foo/lib64",
		},
		"platform": testcase{
			input: "foo/$PLATFORM",
			want:  fmt.Sprintf("foo/%s", platform),
		},
	} {
		t.Run(n, func(t *testing.T) {
			file := File{
				Path: "./testdata/executable",
				File: &elf.File{
					FileHeader: elf.FileHeader{
						Class: elf.ELFCLASS64,
					},
				},
			}

			got, err := file.Expand(c.input)
			if err != nil {
				t.Fatalf(`File.Expand(%q): unexpected error %q`, c.input, err)
			}
			if got != c.want {
				t.Errorf(`File.Expand(%q): wanted %q, got %q`, c.input, c.want, got)
			}
		})
	}
}
