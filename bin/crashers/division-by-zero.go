package main

import "runtime/debug"

func main() {
	debug.SetTraceback("crash")
	var i = 0
	_ = 1 / i
}
