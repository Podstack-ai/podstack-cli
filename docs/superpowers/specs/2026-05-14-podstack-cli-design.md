# Podstack CLI — Design

**Date:** 2026-05-14
**Status:** Approved (pending user review of written spec)

## 1. Purpose

A thin, polished CLI named `podstack` for transferring large files between machines
using [croc](https://github.com/schollz/croc) as the underlying transfer engine.
Croc is bundled into the binary as a Go library import — the user does not need
to install croc separately.

The CLI defaults to a Podstack-operated relay (`relay.cloud.podstack.ai`) but
allows callers to override the relay or fall back to croc's public relay.

## 2. Goals & non-goals

**Goals**

- Single static binary, no external runtime or croc binary required.
- Linux and macOS support, on both `amd64` and `arm64`.
- Support every transfer type croc supports: single file, multiple files,
  directories, mixed, and text mode.
- Resume interrupted transfers automatically (croc's native behaviour).
- Distribution via GitHub Releases, Homebrew tap, and a `curl | sh` installer.

**Non-goals (v1)**

- Windows support.
- Podcast-specific metadata, episode naming, or upload-to-Podstack-backend flow.
- Team features (named rooms, transfer history).
- Exposing every croc flag (`--curve`, `--hash`, etc.). These can be added later.

## 3. Architecture

- Language: **Go**.
- CLI framework: **`github.com/spf13/cobra`**.
- Croc integration: import `github.com/schollz/croc/v10/src/croc` and call its
  `Client` API directly. No subprocess, no embedded binary extraction.
- Cross-compile targets: `darwin-arm64`, `darwin-amd64`, `linux-amd64`,
  `linux-arm64`.
- Default relay baked into the binary: **`relay.cloud.podstack.ai`**.

**Why import as a library rather than embed the binary?**
A library import gives us a single self-contained binary, reproducible builds,
no temp-file extraction at runtime, and proper error propagation. Croc is
MIT-licensed and exposes a stable `Client` API.

## 4. Command surface

```
podstack send [files-or-dirs...]
  --code <phrase>          custom code phrase (default: random 3-word)
  --relay <host|ip>        override relay for this transfer
  --relay-default          fall back to croc's public relay (croc.schollz.com)
  --text <msg>             send a text string instead of a file
  --zip                    zip before sending
  --no-compress            disable compression

podstack receive <code>
  --out <dir>              output directory (default: cwd)
  --relay <host|ip>        override relay
  --relay-default          fall back to croc's public relay
  --yes                    auto-accept the incoming transfer

podstack version
podstack --help
```

### Relay precedence (highest wins)

1. `--relay <host>` flag
2. `--relay-default` flag (selects croc's public relay, `croc.schollz.com`)
3. `PODSTACK_RELAY` environment variable
4. Built-in default: `relay.cloud.podstack.ai`

`--relay` and `--relay-default` are mutually exclusive; passing both is an error.

### Resume

Croc resumes interrupted transfers automatically when a partial file with the
matching name and code exists in the receive directory. We do not expose a flag
for this — it's always on. Documented in `--help`.

## 5. Project layout

```
podstack-cli/
├── go.mod
├── go.sum
├── main.go
├── cmd/
│   ├── root.go         # cobra root + --version
│   ├── send.go         # send subcommand
│   └── receive.go      # receive subcommand
├── internal/
│   ├── relay/relay.go  # relay precedence resolver
│   └── transfer/       # thin wrappers over croc.Client (Send, Receive)
├── scripts/
│   └── install.sh      # curl|sh installer for mac+linux
├── .github/workflows/release.yml   # goreleaser on tag push
├── .goreleaser.yaml
├── Formula/podstack.rb             # mirrored Homebrew formula (tap repo owns source)
├── README.md
└── LICENSE
```

### Module boundaries

- `cmd/` — argument parsing and user-facing I/O only. No transfer logic.
- `internal/relay` — pure function: resolve the relay host from flags, env,
  and defaults. Table-driven tests.
- `internal/transfer` — wraps `croc.Client`. Takes a resolved config, returns
  results or errors. Knows nothing about cobra flags.
- `main.go` — wires cobra root and exits with the appropriate code.

## 6. Build & release

- **goreleaser** builds the four target binaries on a git tag push (`v*`).
- Publishes a GitHub Release with tarballs and a `checksums.txt`.
- goreleaser's `brews:` block auto-updates the formula in the
  `podstack/homebrew-tap` repo.
- Version is injected at build time via
  `-ldflags "-X main.version=$VERSION"` and surfaced by `podstack version`.

## 7. install.sh

A POSIX-compliant script (no bash-only features) that:

1. Detects OS via `uname -s` (`Darwin`, `Linux`) and arch via `uname -m`
   (`x86_64` → `amd64`, `arm64`/`aarch64` → `arm64`).
2. Resolves the latest release tag via the GitHub API.
3. Downloads the matching tarball and verifies its sha256 against
   `checksums.txt` from the release.
4. Extracts the `podstack` binary to `/usr/local/bin` if writable, otherwise
   `$HOME/.local/bin` (and prints a note about adding it to `PATH`).
5. `chmod +x` and prints the installed version.

Intended usage: `curl -fsSL https://podstack.ai/install.sh | sh`.

The script must fail loudly (set `-eu`) on any error, and exit non-zero if the
checksum doesn't match.

## 8. Testing

- **Unit tests** for `internal/relay` covering every branch of the precedence
  rules (table-driven).
- **Integration test** that spins up a local croc relay in a goroutine, runs
  `send` and `receive` against it, and asserts the received file matches the
  source byte-for-byte. A second variant kills the receive mid-transfer,
  restarts it, and asserts the resume completes the file.
- **CI** on every PR: `go test ./...`, `go vet ./...`,
  `golangci-lint run`.

## 9. Error handling

- Invalid flag combinations (e.g. `--relay` and `--relay-default` together) exit
  with code 2 and a one-line message.
- Transfer failures from croc are surfaced with the underlying error message
  and exit code 1.
- `Ctrl-C` cancels cleanly via context cancellation propagated to the croc
  Client.

## 10. Open questions

None at design lock-in. Any flags currently omitted (curve, hash, multi-port,
etc.) can be added in a follow-up without breaking the v1 surface.
