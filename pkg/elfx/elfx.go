package elfx

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/elwinar/rcoredump/pkg/auxv"
)

func init() {
	SetLibraryPath(os.Getenv("LD_LIBRARY_PATH"))
}

var (
	// LibraryPathDirs contains the directories in LD_LIBRARY_PATH.
	LibraryPathDirs []string

	// DefaultDirs contains the list of default directories to look for
	// shared object files. It is set at compile-time depending on the
	// target architecture.
	DefaultDirs []string
)

// SetLibraryPath parse a list of directories in PATH-like format and updates
// the LibraryPathDirs variable. This method is run when the package is
// initialized with whatever the LD_LIBRARY_PATH env var contains.
//
// NOTE The expansion of the $ORIGIN, $PLATFORM and $LIB variables aren't
// performed here, as the first one can only be done in the context of a file.
// This is done in File.Expand.
func SetLibraryPath(path string) {
	if len(path) == 0 {
		LibraryPathDirs = nil
		return
	}

	// The POSIX standard doesn't define an escape char for PATH-like
	// environment vars, and the glibc implements it with
	// `subp = __strchrnul (p, ':');`. v0v.
	dirs := strings.Split(path, ":")

	// Fill the LibraryPathDirs and ensure we have no duplicates. An empty
	// entry is considered as `.`.
	LibraryPathDirs = make([]string, 0, len(dirs))
	met := make(map[string]struct{}, len(dirs))
	for _, d := range dirs {
		if len(d) == 0 {
			d = `.`
		}
		if _, ok := met[d]; ok {
			continue
		}
		LibraryPathDirs = append(LibraryPathDirs, d)
	}
}

// File wraps an elf.File to add additional utility methods on it.
type File struct {
	Path string
	*elf.File
}

func Open(path string) (File, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return File{}, err
	}

	f, err := elf.Open(path)
	if err != nil {
		return File{}, err
	}

	return File{Path: path, File: f}, err
}

// ResolveImportedLibrary return the path of the given library following the
// rules of Linux's dynamic linker and a boolean indicating if the designated
// file exists on the system.
//
// NOTE The rules are described in the manual for ld-linux.so.
func (f File) ResolveImportedLibrary(library string) (path string, ok bool, err error) {
	// We get the DT_RUNPATH section content, and if empty the deprecated
	// DT_RPATH one. The first one only applies to the current file's
	// DT_NEEDED libraries (returned by elf.File.ImportedLibraries()),
	// while the second should be applied transitively.
	var runpath, rpath []string
	runpath, err = f.DynString(elf.DT_RUNPATH)
	if err != nil {
		return library, false, err
	}
	if len(runpath) == 0 {
		rpath, err = f.DynString(elf.DT_RPATH)
		if err != nil {
			return library, false, err
		}
	}

	// We check first if the library is a path, then in the configured and
	// standard directories.
	if strings.Contains(library, "/") {
		if filepath.IsAbs(library) {
			_, err = os.Stat(library)
			if errors.Is(err, os.ErrNotExist) {
				return library, false, nil
			}
			if err != nil {
				return library, false, err
			}
			return library, true, nil
		}

		// filepath.Join does apply filepath.Clean, which has the
		// effect of removing the leading ./ from the path. We want to
		// keep it here to distinguish between relative paths and found
		// paths.
		path = filepath.Join(filepath.Dir(f.Path), library)
		_, err = os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		if err != nil {
			return path, false, err
		}
		return path, true, nil
	}

	for _, dirs := range [][]string{
		rpath,
		LibraryPathDirs,
		runpath,
		DefaultDirs,
	} {
		for _, dir := range dirs {
			dir, err = f.Expand(dir)
			if err != nil {
				return library, false, fmt.Errorf(`expanding path for library %q: %w`, library, err)
			}

			path = filepath.Join(dir, library)
			_, err = os.Stat(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return path, false, err
			}
			return path, true, nil
		}
	}

	return library, false, nil
}

// Expand a rpath specification for tokens like $ORIGIN, $LIB, $PLATFORM.
// Versions with curly braces (${ORIGIN}) are also handled.
//
// NOTE This intentionally doesn't use a more elaborate function like os.Expand
// or github.com/mvdan/sh because both of those have much more features than
// necessary, and variable expansion is a very sensible subject.
func (f File) Expand(path string) (string, error) {
	return expand(path, func(name string) (string, bool, error) {
		switch string(name) {
		case "ORIGIN":
			return filepath.Dir(f.Path), true, nil

		case "LIB":
			if f.Class == elf.ELFCLASS64 {
				return "lib64", true, nil
			}
			return "lib", true, nil

		// Here we assume that the platform value for the running
		// binary and the openned file are the same. This is not
		// necessarily true, but it will do for now.
		//
		// TODO Check if the auxilliary vector can be retrieved from
		// the file itself.
		case "PLATFORM":
			err := parseAux()
			if err != nil {
				return "", false, fmt.Errorf(`parsing auxilliary vector: %w`, err)
			}

			val, ok := aux[auxv.TypePlatform]
			if !ok {
				return "", false, fmt.Errorf(`missing platform entry in auxilliary vector`)
			}

			return val.ReadString(), true, nil

		default:
			return "", false, nil
		}
	})
}

// expand a string by using a translation function for tokens like $NAME or
// ${NAME}. The functor takes the name of the token and returns the replacement
// string and a boolean indicating if the token should not be replaced.
func expand(s string, f func(string) (string, bool, error)) (string, error) {
	var buf bytes.Buffer

	// Read byte by byte. As $, { and } are all ASCII, this is enough.
	for i := 0; i < len(s); i++ {
		// Put all non-token chars into the buffer.
		if s[i] != '$' {
			buf.WriteByte(s[i])
			continue
		}

		// If the $ was the last char, put it into the buffer and
		// exits.
		j := i + 1
		if j >= len(s) {
			buf.WriteByte(s[i])
			break
		}

		// Ignore an eventual opening brace.
		if s[j] == '{' {
			j += 1
		}

		// Continue while we find allowed characters (alphanum and
		// underscores).
		for ; j < len(s) && isAlphaNum(s[j]); j++ {
		}

		// Extract the name of the token, ignoring opening brace.
		name := s[i+1 : j]
		if name[0] == '{' {
			name = name[1:]
		}

		// Translate the token and either add the translation or the
		// token into the buffer.
		value, ok, err := f(name)
		if err != nil {
			return "", fmt.Errorf(`replacing token %q: %w`, name, err)
		}

		if ok {
			buf.WriteString(value)
		} else {
			buf.WriteString(s[i:j])
		}

		// If we didn't start with a brace, the current char must be
		// added to the buffer.
		if j < len(s) && (s[i+1] != '{' || s[j] != '}') {
			buf.WriteByte(s[j])
		}

		// Update the pointer and continue.
		i = j
		continue
	}

	return buf.String(), nil
}

// isAlphaNum reports whether the byte is an ASCII letter, number, or underscore
func isAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

var (
	// Aux is the auxilliary vector parsed at runtime. It is initialized
	// when resolving external libraries.
	aux auxv.Vector
	// auxvOnce is used to ensure that the parsing of the auxilliary vector
	// is done only once independently of what required it.
	auxOnce sync.Once
)

// parseAuxv initialize the auxilliary vector and return the eventual error. It
// must be called before using auxv, and is a no-op if already called once.
func parseAux() error {
	var err error
	auxOnce.Do(func() {
		var f io.Reader
		f, err = os.Open("/proc/self/auxv")
		if err != nil {
			err = fmt.Errorf(`opening auxilliary vector: %w`, err)
			return
		}

		aux = auxv.New()
		err = aux.ReadFrom(f)
		if err != nil {
			err = fmt.Errorf(`reading auxilliary vector: %w`, err)
			return
		}
	})
	return err
}
