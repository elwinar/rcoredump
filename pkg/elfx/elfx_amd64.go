package elfx

func init() {
	// This is probably naive, but it will do until we need something more complex.
	DefaultDirs = []string{"/lib", "/usr/lib", "/lib64", "/usr/lib64"}
}
