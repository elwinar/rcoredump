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
  -dir string
        path of the directory to store the coredumps into (default "/var/lib/rcoredumpd/")
```

### `rcoredump`

_rcoredump_ is the forwarder binary. It compress dumps and send them to the
indexing service.

```
Usage of rcoredump:
  -dest string
        address of the destination host (default "localhost:1105")
  -src string
        path of the coredump to send to the host ('-' for stdin) (default "-")
```

On linux, you can use sysctl's `kernel.core_pattern` tunable to have the kernel
invoke _rcoredump_ everytime a dump is generated. For example:
`kernel.core_pattern=|rcoredump`.

