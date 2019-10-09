package main

import "runtime/debug"

func main() {
	debug.SetTraceback("crash")
	var i *int
	_ = *i
}
