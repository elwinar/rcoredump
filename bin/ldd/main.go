package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/elwinar/rcoredump/pkg/elfx"
)

func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Println("invalid arguments: expected 1, got", flag.NArg())
		flag.PrintDefaults()
		os.Exit(1)
	}

	executable := flag.Arg(0)
	file, err := elfx.Open(executable)
	defer file.Close()
	if err != nil {
		fmt.Println("opening binary file:", err)
		os.Exit(1)
	}

	stack, err := file.ImportedLibraries()
	if err != nil {
		fmt.Println("parsing imported libraries:", err)
		os.Exit(1)
	}

	var known = make(map[string]struct{})
	for _, l := range stack {
		known[l] = struct{}{}
	}

	for len(stack) != 0 {
		var library string
		library, stack = stack[0], stack[1:]

		path, ok, err := file.ResolveImportedLibrary(library)
		if err != nil {
			fmt.Printf("%s: error while resolving: %s\n", library, err)
			continue
		}

		if !ok {
			fmt.Printf("%s: not found\n", library)
			continue
		}

		fmt.Printf("%s => %s\n", library, path)

		libfile, err := elfx.Open(path)
		if err != nil {
			fmt.Println("opening library", library, "file:", err)
			os.Exit(1)
		}

		libparents, err := libfile.ImportedLibraries()
		if err != nil {
			fmt.Println("parsing library", library, "parents:", err)
		}
		for _, l := range libparents {
			if _, ok := known[l]; ok {
				continue
			}
			stack = append(stack, l)
			known[l] = struct{}{}
		}
	}
}
