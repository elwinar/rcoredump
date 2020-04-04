# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## [v0.8.0](https://github.com/elwinar/rcoredump/releases/tag/v0.8.0) - 2020-04-04
###
- Pagination in the API
- Pagination of the results in the webapp
- Add a special display for empty results
- Permalinks to coredump and executable searchs
### Changed
- Design improvements:
	- Hover color for result rows
	- Lighter separators in table for better readability

## [v0.7.0](https://github.com/elwinar/rcoredump/releases/tag/v0.7.0) - 2020-03-29
### Added
- /about endpoint with build information
- Version of the webapp in the webapp's header
- ForwarderVersion and IndexerVersion in the indexed fields
- Metric for the size of the received coredumps, and options to control the exposed buckets
- Coredump.AnalyzedAt field with the date the coredump was anayzed (updated upon analysis)
- Coredump.Executable field with the name of the executable
### Changed
- The `c.analyzer` and `go.analyzer` options now take the gdb or delve commands to execute on the coredump, instead of full shell commands (#27)
- Design & Interface overhaul:
	- Better table readability and usage by changing padding and making the whole row clickable
	- Rework the searchbar to remove useless options and optimize the sorting
	- Reorganization of the detail view and table heading for better readability
	- Design adjustments for better compliance with [WCAG 2.0](https://www.w3.org/TR/WCAG20/)
- The sort and sort order parameters are now separate options because it's simpler
- Limit the sort options to date and hostname because the executable one isn't working as intended anymore
- Compute the core and executable sizes during indexation instead of analysis
- Rename Coredump.Date to Coredump.DumpedAt for consistency with Coredump.AnalyzedAt
### Fixed
- Case of the fields for sorting is back to lowercase

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
