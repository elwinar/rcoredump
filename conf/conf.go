package conf

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
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
	err := parse(fs, conf)
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

func parse(fs *flag.FlagSet, conf string) error {
	fs.Parse(os.Args[1:])

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

		chunks := strings.SplitN(line, "=", 2)
		if len(chunks) == 1 {
			chunks = append(chunks, "true")
		}
		key, val := strings.TrimSpace(chunks[0]), strings.TrimSpace(chunks[1])

		if set[key] {
			continue
		}

		err := fs.Set(key, val)
		if err != nil {
			return fmt.Errorf("setting flag %q to %q: %w", key, val, err)
		}
	}

	return nil
}
