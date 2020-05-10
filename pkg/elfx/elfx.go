package elfx

import (
	"bytes"
	"debug/elf"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
//
// NOTE It currently doesn't check the DT_RUNPATH dynamic section of the
// binary, mainly because it seems so very infrequently used that I can't be
// bothered right now.
func (f File) ResolveImportedLibrary(library string) (path string, ok bool, err error) {
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
		LibraryPathDirs,
		DefaultDirs,
	} {
		for _, dir := range dirs {
			path = filepath.Join(f.Expand(dir), library)
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
//
// BUG The PLATFORM replacement is inherently wrong, and should probably not be
// relied upon.
func (f File) Expand(path string) string {
	return expand(path, func(name string) (value string, ok bool) {
		switch string(name) {
		case "ORIGIN":
			return filepath.Dir(f.Path), true

		case "LIB":
			if f.Class == elf.ELFCLASS64 {
				return "lib64", true
			}
			return "lib", true

		// This is a best attempt at something that is probably
		// fundamentaly wrong or impossible in the context: the
		// platform string is given by the kernel to the program in the
		// auxilliary vector (see getauxval(3)), which we can't get
		// afterwards. Something more complex could probably be done,
		// but right now this will do.
		case "PLATFORM":
			switch f.Class {
			case elf.ELFCLASS64:
				return "x86_64", true
			case elf.ELFCLASS32:
				return "i386", true
			default:
				return "unhandled_arch", true
			}

		default:
			return "", false
		}
	})
}

// expand a string by using a translation function for tokens like $NAME or
// ${NAME}. The functor takes the name of the token and returns the replacement
// string and a boolean indicating if the token should be replaced or not.
func expand(s string, f func(string) (string, bool)) string {
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
		value, ok := f(name)
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

	return buf.String()
}

// isAlphaNum reports whether the byte is an ASCII letter, number, or underscore
func isAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}
