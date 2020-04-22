package main

import "runtime/debug"

func main() {
	debug.SetTraceback("crash")
	panic("panic")
}
