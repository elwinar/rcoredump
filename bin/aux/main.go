package main

import (
	"fmt"
	"os"

	"github.com/elwinar/rcoredump/pkg/auxv"
)

func main() {
	f, err := os.Open("/proc/self/auxv")
	if err != nil {
		fmt.Println(err)
		return
	}

	v := auxv.New()
	err = v.ReadFrom(f)
	if err != nil {
		fmt.Println(err)
		return
	}

	if val, ok := v[auxv.TypePlatform]; ok {
		fmt.Println(auxv.ReadString(uintptr(val)))
	}
}
