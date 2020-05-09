package main

import (
	"flag"
	"fmt"

	"github.com/elwinar/rcoredump/pkg/elfx"
)

func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Println("invalid arguments: expected 1, got", flag.NArg())
		flag.PrintDefaults()
		return
	}

	executable := flag.Args()[0]
	file, err := elfx.Open(executable)
	if err != nil {
		fmt.Println("opening binary file:", err)
		return
	}

	libraries, err := file.ImportedLibraries()
	if err != nil {
		fmt.Println("parsing imported libraries:", err)
		return
	}

	for _, library := range libraries {
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
	}
}
