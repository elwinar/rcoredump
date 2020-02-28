# rcoredump

_rcoredump_ is meant to be a toolbox for aggregating, indexing, and searching
core dumps. Think ELK for core dumps.

## Usage

### `rcoredumpd`

_rcoredumpd_ is the indexation service. It listen on a TCP connection for
incoming files and process them.

```
Usage of rcoredumpd: rcoredumpd [options]
  -bind string
    	address to listen to (default "localhost:1105")
  -conf string
    	configuration file to load (default "/etc/rcoredump/rcoredumpd.conf")
  -dir string
    	path of the directory to store the coredumps into (default "/var/lib/rcoredumpd/")
  -version
    	print the version of rcoredumpd
```

### `rcoredump`

_rcoredump_ is the forwarding tool. It sends the core dump, the binary and a
header with some additional informations to the indexing service.

```
Usage of rcoredump: rcoredump [options] <executable path> <timestamp of dump>
  -conf string
    	configuration file to load (default "/etc/rcoredump/rcoredump.conf")
  -dest string
    	address of the destination host (default "http://localhost:1105")
  -filelog string
    	path of the file to log into ('-' for stdout) (default "-")
  -send-binary
    	send the binary along with the dump (default true)
  -src string
    	path of the coredump to send to the host ('-' for stdin) (default "-")
  -syslog
    	output logs to syslog
  -version
    	print the version of rcoredum
```

On linux, you can use sysctl's `kernel.core_pattern` tunable to have the kernel
invoke _rcoredump_ everytime a dump is generated. For example:
`kernel.core_pattern=|/path/to/rcoredump %E %t`.

## Logging

All logging is done on stdout using the _logfmt_ format. This output can be
redirected easily enough using various utilities, like `logger` for syslog. For
convenience, the forwarder also accept a `syslog` flag to log using syslog, and
a `filelog` flag to log to a file.
