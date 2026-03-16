# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**srrb** — Static RSS Reader Backend. A Go CLI tool (`srrb`) that fetches, parses, and stores RSS/Atom/RDF feed articles into gzip-compressed packs. Supports local filesystem, S3, and SFTP storage backends.

## Commands

```bash
go build -o srrb .           # Build
go test ./...               # Run all tests
go test -run TestName .     # Run a single test
go test -v ./backend/       # Test a specific package
```

No Makefile, linter config, or Dockerfile exists. Release builds use `CGO_ENABLED=0 go build -ldflags "-s -w"`.

## Architecture

**CLI entry point** (`main.go`): Uses `alecthomas/kong` with YAML config file support. Global flags control concurrency, package size, output path, etc. via `Globals` struct.

**Command files** (`cmd_*.go`): Each CLI subcommand (add, rm, ls, fetch, import, extern) lives in its own file at the root package.

**Feed parsing** (`feed.go`): Unified streaming XML parser that auto-detects RSS, Atom, and RDF formats. Parses items one-by-one with fallback logic for fields like GUID and dates.

**Storage layer** — two abstractions:
- `backend/` — Low-level storage interface (Get/Put/AtomicPut/Rm). Implementations: local filesystem, S3, SFTP. Backend selected by URL scheme of output path.
- `store.go` — Higher-level pack management. Articles stored in gzip-compressed packs (~200KB target). Uses boolean Latest flag toggling for atomic updates.

**Module system** (`mod/`): Processing pipeline applied per-subscription during fetch. Built-in: `#sanitize` (bluemonday), `#minify` (tdewolff/minify). External modules invoked as shell commands with JSON I/O.

**Subscription model** (`subscription.go`): Tracks feed metadata (ETag, Last-Modified, LastGUID) for conditional fetching. Parallel fetch with configurable worker pool.

**Database** (`db.go` or `db/`): Manages subscriptions and article packs with file-based locking to prevent concurrent writes.

## Key Patterns

- Atomic writes via temp-then-rename (local) or conditional puts (S3)
- File-based DB locking (`.locked` key) with `--force` override
- Environment variables prefixed with `SRR_` (e.g., `SRR_JOBS`, `SRR_OUTPUT_PATH`)
- Error wrapping with `fmt.Errorf("%w", err)` for context propagation
- `ErrStopFeed` sentinel to halt feed processing early
