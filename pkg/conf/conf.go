package conf

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Parse the given FlagSet using the command line and the file pointed by the
// conf flag value.
//
// The parser ignore empty lines and lines that start with a #.
// Lines without a = sign will be considered as a boolean flag and the value
// will default to true.
// The priority order is command line, conf file, then default value.
func Parse(fs *flag.FlagSet, conf string) {
	err := parse(fs, os.Args[1:], conf)
	if err == nil {
		return
	}

	fmt.Fprintln(fs.Output(), err)
	fs.Usage()
	switch fs.ErrorHandling() {
	case flag.ContinueOnError:
		return
	case flag.ExitOnError:
		os.Exit(2)
	case flag.PanicOnError:
		panic(err)
	}
}

func parse(fs *flag.FlagSet, args []string, conf string) error {
	// Can't work on an empty flagset.
	if fs == nil {
		return errors.New(`nil flagset`)
	}

	fs.Parse(args)

	// If there is no configuration flag given, there is nothing to do.
	if conf == "" {
		return nil
	}

	// The flag package doesn't provide a view of which flags have been
	// set. The Visit method, however, is iterating on the
	// flag.FlagSet.actual map, which allow us to get this information.
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	f := fs.Lookup(conf)
	if f == nil {
		return fmt.Errorf("configuration flag %q not found", conf)
	}

	path, ok := f.Value.(flag.Getter).Get().(string)
	if !ok {
		return fmt.Errorf("non-string configuration flag %q given", conf)
	}

	file, err := os.Open(path)

	// If the conf flag wasn't set by hand and it doesn't exist, ignore the
	// error.
	if errors.Is(err, os.ErrNotExist) && !set[conf] {
		return nil
	}

	if err != nil {
		return fmt.Errorf("opening configuration file: %w", err)
	}

	// Parse the configuration file line by line. Ignore empty line, lines
	// that start with a #. Lines without a = sign will be considered as a
	// boolean flag and the value will default to true. Only set the flags
	// that weren't encountered on the command line.
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		// Ignore dashes at the start of lines.
		line = strings.TrimLeft(line, "-")

		chunks := strings.SplitN(line, "=", 2)
		if len(chunks) == 1 {
			chunks = append(chunks, "true")
		}
		key, val := chunks[0], chunks[1]

		key = strings.TrimSpace(key)
		if set[key] {
			continue
		}

		val = strings.TrimSpace(val)
		if len(val) != 0 && val[0] == '"' {
			val, err = strconv.Unquote(val)
			if err != nil {
				return fmt.Errorf("unquoting value %q for key %q: %w", val, key, err)
			}
		}

		err := fs.Set(key, val)
		if err != nil {
			return fmt.Errorf("setting flag %q to %q: %w", key, val, err)
		}
	}

	return nil
}

// MapFlag returns a flag.Value that will be parsed into the given map.  Raw
// flag value is split on ';' to separate multiple key-value pairs, and on the
// first '=' to separate the key from the value. Quoted values aren't handled
// specifically because there is already a layer of unquoting done either by
// the command-line or the configuration file parsing. The parsing doesn't
// support any kind of escaping either for simplicity reasons.
func MapFlag(m *map[string]string) *mapFlag {
	if *m == nil {
		*m = make(map[string]string)
	}
	return &mapFlag{
		m: *m,
	}
}

type mapFlag struct {
	m map[string]string
}

// String return the textual representation for this map's content.
func (f *mapFlag) String() string {
	if len(f.m) == 0 {
		return ""
	}

	var keys []string
	for k, _ := range f.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	buf.WriteString(keys[0])
	buf.WriteByte('=')
	buf.WriteString(f.m[keys[0]])
	for _, k := range keys[1:] {
		buf.WriteByte(';')
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(f.m[k])
	}
	return buf.String()
}

// Set values in the map from the raw string given.
func (f *mapFlag) Set(raw string) error {
	for _, value := range strings.Split(raw, ";") {
		chunks := strings.SplitN(value, "=", 2)
		if len(chunks) == 1 {
			f.m[chunks[0]] = ""
			continue
		}

		key, val := chunks[0], chunks[1]
		f.m[key] = val
	}
	return nil
}
