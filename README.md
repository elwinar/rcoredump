# rcoredump

_rcoredump_ is meant to be a toolbox for aggregating, indexing, and searching
core dumps. Think ELK for core dumps.

## Usage

### `rcoredumpd`

_rcoredumpd_ is the indexation service. It listen on a TCP connection for
incoming files and process them.

```
Usage of rcoredumpd:
  -bind string
        address to listen to (default "localhost:1105")
  -conf string
        configuration file to load (default "/etc/rcoredump/rcoredumpd.conf")
  -dir string
        path of the directory to store the coredumps into (default "/var/lib/rcoredumpd/")
```

Each connection yield a header file, a core dump and the binary that crashed.
All three are saved in the data directory as `<id>.<type>`.

### `rcoredump`

_rcoredump_ is the forwarding tool. It sends the core dump, the binary and a
header with some additional informations to the indexing service.

```
Usage of rcoredump: rcoredump [options] <executable path> <timestamp of dump>
  -conf string
        configuration file to load (default "/etc/rcoredump/rcoredump.conf")
  -dest string
        address of the destination host (default "localhost:1105")
  -log string
        path of the log file for rcoredump (default "/var/log/rcoredump.log")
  -src string
        path of the coredump to send to the host ('-' for stdin) (default "-")
```

On linux, you can use sysctl's `kernel.core_pattern` tunable to have the kernel
invoke _rcoredump_ everytime a dump is generated. For example:
`kernel.core_pattern=|rcoredump %E %t`.

