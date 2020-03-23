# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased
### Added
- /about endpoint with build information

## [v0.6.1](https://github.com/elwinar/rcoredump/releases/tag/v0.6.1) - 2020-03-23
### Internal
- Cleanup `go.mod` dependencies for unused packages

## [v0.6.0](https://github.com/elwinar/rcoredump/releases/tag/v0.6.0) - 2020-03-22
### Added
- `metadata` options accept a list of key-value pairs to send alongside the coredump to the indexer
- Travis-CI integration is setup [here](https://travis-ci.org/github/elwinar/rcoredump)
### Changed
- the `dir` option becomes `data-dir`
### Removed
- `bleve.path` configuration option: it is now deduced from the `data-dir` option
