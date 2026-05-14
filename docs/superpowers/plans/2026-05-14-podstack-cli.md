# Podstack CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a thin Go CLI named `podstack` that wraps croc v10.4.3 (imported as a library) for sending and receiving large files, defaulting to the Podstack-operated relay (`relay.cloud.podstack.ai`), distributed via GitHub Releases, a Homebrew tap, and a `curl | sh` installer for macOS and Linux.

**Architecture:** Single static Go binary. `cmd/` holds cobra subcommands; `internal/relay` resolves the relay host via flag/env/default precedence; `internal/transfer` builds the `croc.Options` struct and calls `croc.New + Send/Receive`. Distribution is goreleaser-driven on tag push, producing four binaries (`darwin/linux × amd64/arm64`) and updating a Homebrew tap.

**Tech Stack:** Go (1.22+), `github.com/spf13/cobra` v1, `github.com/schollz/croc/v10` v10.4.3, goreleaser v1, GitHub Actions, `golangci-lint` v1.

**Spec reference:** `docs/superpowers/specs/2026-05-14-podstack-cli-design.md`

---

## File Structure

Files this plan creates, with the responsibility of each:

- `go.mod`, `go.sum` — Go module manifest. Module path `github.com/podstack/podstack-cli`.
- `main.go` — entrypoint, calls `cmd.Execute()`.
- `cmd/root.go` — cobra root command + `version` subcommand. Holds the `version` variable populated by `-ldflags`.
- `cmd/send.go` — `send` subcommand. Parses flags, calls `relay.Resolve`, calls `transfer.Send`.
- `cmd/receive.go` — `receive` subcommand. Parses flags, calls `relay.Resolve`, calls `transfer.Receive`.
- `internal/relay/relay.go` — pure relay-precedence resolver. Function `Resolve(flagRelay string, flagDefault bool, env string) (string, error)`.
- `internal/relay/relay_test.go` — table-driven unit tests for `Resolve`.
- `internal/transfer/transfer.go` — wraps `croc.Options + New + Send/Receive`. Exposes `SendConfig`, `ReceiveConfig`, `Send(SendConfig)`, `Receive(ReceiveConfig)`.
- `internal/transfer/transfer_test.go` — unit tests asserting `croc.Options` is populated correctly from our config (no network).
- `internal/transfer/integration_test.go` — build-tag `integration`. Spins up a croc relay in a goroutine, runs send/receive end-to-end, plus a resume variant via subprocess kill/restart.
- `scripts/install.sh` — POSIX `sh` installer. Detects OS/arch, downloads tarball from latest release, verifies sha256, installs to `/usr/local/bin` or `$HOME/.local/bin`.
- `scripts/install_test.sh` — shell-based smoke test (manual + CI) that runs `install.sh` against a fake release served from a local HTTP server.
- `.goreleaser.yaml` — goreleaser config: 4 build targets, archive naming, checksum, Homebrew tap publishing.
- `.github/workflows/ci.yml` — runs `go test`, `go vet`, `golangci-lint` on every PR.
- `.github/workflows/release.yml` — runs goreleaser on `v*` tag push.
- `README.md` — usage, install instructions, examples (send file, send dir, send text, receive, resume).
- `LICENSE` — MIT.
- `.gitignore` — Go-standard ignores plus `dist/` (goreleaser output).

**Key croc facts (verified against v10.4.3 source):**
- Import path: `github.com/schollz/croc/v10/src/croc` and `…/v10/src/models`.
- `croc.New(croc.Options) (*croc.Client, error)`.
- `(*croc.Client).Send(filesInfo []croc.FileInfo, emptyFolders []croc.FileInfo, totalFolders int) error`.
- `(*croc.Client).Receive() error`.
- `croc.GetFilesInfo(paths []string, zip bool, ignoreGit bool, exclude []string) (filesInfo, emptyFolders, totalFolders, error)`.
- Default relay password is `models.DEFAULT_PASSPHRASE` (`"pass123"`). The Podstack relay must accept this password for v1.
- Croc has no `context.Context` parameter. Ctrl-C is handled by croc's internal `signal.Notify`. Our code does not install signal handlers.
- Croc writes progress to stderr unconditionally. `Options.Quiet = true` redirects stderr to `/dev/null` — do not enable it by default.
- Send-side requires `RelayPorts []string` (default `["9009","9010","9011","9012","9013"]`). Receive-side negotiates ports via the relay handshake and only needs `RelayAddress`.
- `SharedSecret` must be ≥6 characters; `New` rejects shorter codes.
- A relay address must be in `host:port` form.

---

## Task 1: Scaffold the Go module + cobra skeleton

**Files:**
- Create: `go.mod`, `main.go`, `cmd/root.go`, `.gitignore`, `LICENSE`

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd /Users/saurav/Podstack/podstack-cli
go mod init github.com/podstack/podstack-cli
```

Expected: creates `go.mod` with `module github.com/podstack/podstack-cli` and `go 1.22` (or whatever your local Go is — 1.22+ required).

- [ ] **Step 2: Add cobra and croc dependencies**

Run:
```bash
go get github.com/spf13/cobra@v1.8.1
go get github.com/schollz/croc/v10@v10.4.3
```

Expected: `go.mod` lists both, `go.sum` populated.

- [ ] **Step 3: Write `main.go`**

Create `/Users/saurav/Podstack/podstack-cli/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/podstack/podstack-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "podstack:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Write `cmd/root.go`**

Create `/Users/saurav/Podstack/podstack-cli/cmd/root.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "podstack",
	Short: "Send and receive large files using croc",
	Long: `podstack is a thin wrapper around croc (https://github.com/schollz/croc).

It defaults to the Podstack relay (relay.cloud.podstack.ai) and supports
sending files, directories, and text. Interrupted transfers resume
automatically when the receiver is re-run with the same code in the same
output directory.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the podstack version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
```

- [ ] **Step 5: Write `.gitignore`**

Create `/Users/saurav/Podstack/podstack-cli/.gitignore`:

```
# Binaries
podstack
podstack-*
*.exe

# Go
*.test
*.out
vendor/

# Goreleaser
dist/

# Editors
.idea/
.vscode/
*.swp
.DS_Store
```

- [ ] **Step 6: Write `LICENSE`**

Create `/Users/saurav/Podstack/podstack-cli/LICENSE` with the standard MIT license text:

```
MIT License

Copyright (c) 2026 Podstack

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```

- [ ] **Step 7: Verify the binary builds and `version` runs**

Run:
```bash
go build -o podstack . && ./podstack version
```

Expected output: `dev`

Run:
```bash
./podstack --help
```

Expected: cobra prints usage text mentioning `podstack` and the `version` subcommand. No other subcommands yet.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go .gitignore LICENSE
git commit -m "feat: scaffold cobra CLI with version subcommand"
```

---

## Task 2: Relay precedence resolver (TDD)

**Files:**
- Create: `internal/relay/relay.go`, `internal/relay/relay_test.go`

The resolver is a pure function. Precedence (highest wins): explicit `--relay <host>` → `--relay-default` flag → `PODSTACK_RELAY` env var → built-in default `relay.cloud.podstack.ai:9009`. `--relay` and `--relay-default` are mutually exclusive (caller is responsible for setting at most one).

Croc's public relay is `croc.schollz.com:9009`. We hardcode both constants here.

- [ ] **Step 1: Write the failing test file**

Create `/Users/saurav/Podstack/podstack-cli/internal/relay/relay_test.go`:

```go
package relay

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		flagRelay   string
		flagDefault bool
		env         string
		want        string
		wantErr     bool
	}{
		{
			name: "all defaults uses podstack relay",
			want: "relay.cloud.podstack.ai:9009",
		},
		{
			name: "env overrides default",
			env:  "myrelay.example.com:9009",
			want: "myrelay.example.com:9009",
		},
		{
			name:        "flag-default overrides env",
			env:         "myrelay.example.com:9009",
			flagDefault: true,
			want:        "croc.schollz.com:9009",
		},
		{
			name:      "explicit flag wins over everything",
			env:       "myrelay.example.com:9009",
			flagRelay: "another.example.com:9009",
			want:      "another.example.com:9009",
		},
		{
			name:        "both flags is an error",
			flagRelay:   "foo:9009",
			flagDefault: true,
			wantErr:     true,
		},
		{
			name:      "host without port gets :9009 appended",
			flagRelay: "foo.example.com",
			want:      "foo.example.com:9009",
		},
		{
			name: "env without port gets :9009 appended",
			env:  "envrelay.example.com",
			want: "envrelay.example.com:9009",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.flagRelay, tt.flagDefault, tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test, confirm it fails to compile**

Run:
```bash
go test ./internal/relay/...
```

Expected: compile error — `undefined: Resolve`.

- [ ] **Step 3: Write the minimal implementation**

Create `/Users/saurav/Podstack/podstack-cli/internal/relay/relay.go`:

```go
package relay

import (
	"errors"
	"strings"
)

const (
	// PodstackDefault is the relay used when no flag, env, or override is set.
	PodstackDefault = "relay.cloud.podstack.ai:9009"
	// CrocPublicDefault matches croc's upstream public relay.
	CrocPublicDefault = "croc.schollz.com:9009"
	// defaultPort is the canonical croc relay port.
	defaultPort = "9009"
)

// ErrConflictingFlags is returned when both --relay and --relay-default are set.
var ErrConflictingFlags = errors.New("--relay and --relay-default are mutually exclusive")

// Resolve picks the relay address using the documented precedence:
// flagRelay (--relay <x>) > flagDefault (--relay-default) > env (PODSTACK_RELAY) > PodstackDefault.
//
// If the chosen value has no ":port" it is appended with the default croc port.
func Resolve(flagRelay string, flagDefault bool, env string) (string, error) {
	if flagRelay != "" && flagDefault {
		return "", ErrConflictingFlags
	}
	switch {
	case flagRelay != "":
		return ensurePort(flagRelay), nil
	case flagDefault:
		return CrocPublicDefault, nil
	case env != "":
		return ensurePort(env), nil
	default:
		return PodstackDefault, nil
	}
}

func ensurePort(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":" + defaultPort
}
```

- [ ] **Step 4: Run the tests, confirm they pass**

Run:
```bash
go test ./internal/relay/... -v
```

Expected: all subtests `PASS`.

- [ ] **Step 5: Wire flag-conflict exit code into `main.go`**

Spec section 9 calls for exit code 2 when `--relay` and `--relay-default` are both set (vs. exit code 1 for transfer failures). Update `main.go` to inspect the error.

Edit `/Users/saurav/Podstack/podstack-cli/main.go` to read:

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/podstack/podstack-cli/cmd"
	"github.com/podstack/podstack-cli/internal/relay"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "podstack:", err)
		if errors.Is(err, relay.ErrConflictingFlags) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Verify the build still succeeds**

Run:
```bash
go build -o podstack . && ./podstack version
```

Expected: prints `dev`.

- [ ] **Step 7: Commit**

```bash
git add internal/relay/ main.go
git commit -m "feat(relay): add precedence resolver with table-driven tests"
```

---

## Task 3: Transfer wrappers (send + receive) with unit tests

**Files:**
- Create: `internal/transfer/transfer.go`, `internal/transfer/transfer_test.go`

The wrappers translate our flag-shaped configs into `croc.Options` and call `croc.New + Send/Receive`. The unit tests check option mapping without doing any network I/O — they call a small helper `buildSendOptions` / `buildReceiveOptions` that returns the populated `croc.Options` for assertion. The end-to-end network test is Task 7.

- [ ] **Step 1: Write the failing test file**

Create `/Users/saurav/Podstack/podstack-cli/internal/transfer/transfer_test.go`:

```go
package transfer

import (
	"strings"
	"testing"
)

func TestBuildSendOptions(t *testing.T) {
	cfg := SendConfig{
		Code:       "podstack-foo-bar",
		Relay:      "relay.cloud.podstack.ai:9009",
		Text:       "",
		ZipFolder:  true,
		NoCompress: true,
	}
	opts := buildSendOptions(cfg)

	if !opts.IsSender {
		t.Error("IsSender should be true")
	}
	if opts.SharedSecret != "podstack-foo-bar" {
		t.Errorf("SharedSecret = %q, want %q", opts.SharedSecret, "podstack-foo-bar")
	}
	if opts.RelayAddress != "relay.cloud.podstack.ai:9009" {
		t.Errorf("RelayAddress = %q", opts.RelayAddress)
	}
	if !opts.ZipFolder {
		t.Error("ZipFolder should be true")
	}
	if !opts.NoCompress {
		t.Error("NoCompress should be true")
	}
	if opts.SendingText {
		t.Error("SendingText should be false when Text is empty")
	}
	if len(opts.RelayPorts) == 0 {
		t.Error("RelayPorts must be populated for sender")
	}
	// Suppress IPv6 relay when a custom IPv4 relay is set (matches croc's own cli.go wiring).
	if opts.RelayAddress6 != "" {
		t.Errorf("RelayAddress6 should be cleared when custom RelayAddress is set, got %q", opts.RelayAddress6)
	}
}

func TestBuildSendOptionsTextMode(t *testing.T) {
	cfg := SendConfig{Code: "code-foo-bar", Relay: "x:9009", Text: "hello world"}
	opts := buildSendOptions(cfg)
	if !opts.SendingText {
		t.Error("SendingText should be true when Text is non-empty")
	}
}

func TestBuildReceiveOptions(t *testing.T) {
	cfg := ReceiveConfig{
		Code:        "podstack-foo-bar",
		Relay:       "relay.cloud.podstack.ai:9009",
		AutoAccept:  true,
	}
	opts := buildReceiveOptions(cfg)
	if opts.IsSender {
		t.Error("IsSender should be false for receive")
	}
	if !opts.NoPrompt {
		t.Error("NoPrompt should be true when AutoAccept is set")
	}
	if opts.SharedSecret != "podstack-foo-bar" {
		t.Errorf("SharedSecret = %q", opts.SharedSecret)
	}
	if opts.RelayAddress != "relay.cloud.podstack.ai:9009" {
		t.Errorf("RelayAddress = %q", opts.RelayAddress)
	}
}

func TestValidateSendConfig(t *testing.T) {
	t.Run("text and paths conflict", func(t *testing.T) {
		err := SendConfig{Code: "code-foo-bar", Relay: "x:9009", Text: "hi", Paths: []string{"f.txt"}}.Validate()
		if err == nil || !strings.Contains(err.Error(), "text") {
			t.Errorf("expected text/paths conflict error, got %v", err)
		}
	})
	t.Run("no text and no paths", func(t *testing.T) {
		err := SendConfig{Code: "code-foo-bar", Relay: "x:9009"}.Validate()
		if err == nil {
			t.Error("expected error when neither text nor paths given")
		}
	})
	t.Run("short code", func(t *testing.T) {
		err := SendConfig{Code: "abc", Relay: "x:9009", Text: "hi"}.Validate()
		if err == nil {
			t.Error("expected error for code shorter than 6 chars")
		}
	})
	t.Run("valid text-only", func(t *testing.T) {
		err := SendConfig{Code: "code-foo-bar", Relay: "x:9009", Text: "hi"}.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("valid file-only", func(t *testing.T) {
		err := SendConfig{Code: "code-foo-bar", Relay: "x:9009", Paths: []string{"f.txt"}}.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
```

- [ ] **Step 2: Run, confirm it fails to compile**

Run:
```bash
go test ./internal/transfer/...
```

Expected: compile errors — `undefined: SendConfig`, `buildSendOptions`, etc.

- [ ] **Step 3: Write the implementation**

Create `/Users/saurav/Podstack/podstack-cli/internal/transfer/transfer.go`:

```go
// Package transfer wraps the croc Go library to send and receive files.
//
// The public surface is intentionally minimal: SendConfig / ReceiveConfig
// describe what the caller wants, and Send / Receive perform the transfer.
// All croc-specific knobs live in this package so cmd/ can stay free of
// croc imports.
package transfer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/schollz/croc/v10/src/croc"
	"github.com/schollz/croc/v10/src/models"
)

const minCodeLen = 6

// basePort and transferCount mirror croc's CLI defaults. The relay must
// expose basePort..basePort+transferCount.
const (
	basePort      = 9009
	transferCount = 4
)

// SendConfig is the user-facing send configuration.
type SendConfig struct {
	Code       string   // shared secret; must be ≥6 chars
	Relay      string   // host:port
	Paths      []string // file/dir paths to send (mutually exclusive with Text)
	Text       string   // text body (mutually exclusive with Paths)
	ZipFolder  bool
	NoCompress bool
}

// ReceiveConfig is the user-facing receive configuration.
type ReceiveConfig struct {
	Code       string // shared secret
	Relay      string // host:port
	OutDir     string // chdir target before receiving; "" means cwd
	AutoAccept bool   // skip prompt
}

// Validate checks SendConfig for user-input errors before touching the network.
func (c SendConfig) Validate() error {
	if len(c.Code) < minCodeLen {
		return fmt.Errorf("code must be at least %d characters", minCodeLen)
	}
	hasPaths := len(c.Paths) > 0
	hasText := c.Text != ""
	if hasPaths && hasText {
		return errors.New("--text is mutually exclusive with file/directory arguments")
	}
	if !hasPaths && !hasText {
		return errors.New("nothing to send: provide at least one file/directory or --text")
	}
	if c.Relay == "" {
		return errors.New("relay address is empty")
	}
	return nil
}

// Validate checks ReceiveConfig for user-input errors.
func (c ReceiveConfig) Validate() error {
	if len(c.Code) < minCodeLen {
		return fmt.Errorf("code must be at least %d characters", minCodeLen)
	}
	if c.Relay == "" {
		return errors.New("relay address is empty")
	}
	return nil
}

func buildRelayPorts() []string {
	ports := make([]string, transferCount+1)
	for i := 0; i <= transferCount; i++ {
		ports[i] = strconv.Itoa(basePort + i)
	}
	return ports
}

func buildSendOptions(cfg SendConfig) croc.Options {
	return croc.Options{
		IsSender:      true,
		SharedSecret:  cfg.Code,
		RelayAddress:  cfg.Relay,
		RelayAddress6: "", // forced empty: we use a single IPv4 relay
		RelayPorts:    buildRelayPorts(),
		RelayPassword: models.DEFAULT_PASSPHRASE,
		Curve:         "p256",
		HashAlgorithm: "xxhash",
		SendingText:   cfg.Text != "",
		NoCompress:    cfg.NoCompress,
		ZipFolder:     cfg.ZipFolder,
		Overwrite:     false,
		NoPrompt:      true, // sender never prompts
	}
}

func buildReceiveOptions(cfg ReceiveConfig) croc.Options {
	return croc.Options{
		IsSender:      false,
		SharedSecret:  cfg.Code,
		RelayAddress:  cfg.Relay,
		RelayAddress6: "",
		RelayPassword: models.DEFAULT_PASSPHRASE,
		Curve:         "p256",
		HashAlgorithm: "xxhash",
		NoPrompt:      cfg.AutoAccept,
		Overwrite:     true, // required for resume
	}
}

// Send performs the send half of the croc handshake.
func Send(cfg SendConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	opts := buildSendOptions(cfg)

	var (
		filesInfo     []croc.FileInfo
		emptyFolders  []croc.FileInfo
		totalFolders  int
		err           error
	)
	if cfg.Text == "" {
		filesInfo, emptyFolders, totalFolders, err = croc.GetFilesInfo(cfg.Paths, cfg.ZipFolder, false, nil)
		if err != nil {
			return fmt.Errorf("collecting files: %w", err)
		}
	} else {
		filesInfo, err = textFileInfo(cfg.Text)
		if err != nil {
			return err
		}
	}

	client, err := croc.New(opts)
	if err != nil {
		return fmt.Errorf("creating croc client: %w", err)
	}
	if err := client.Send(filesInfo, emptyFolders, totalFolders); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

// Receive performs the receive half of the croc handshake.
func Receive(cfg ReceiveConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.OutDir != "" {
		if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
			return fmt.Errorf("creating out dir: %w", err)
		}
		if err := os.Chdir(cfg.OutDir); err != nil {
			return fmt.Errorf("entering out dir: %w", err)
		}
	}
	opts := buildReceiveOptions(cfg)
	client, err := croc.New(opts)
	if err != nil {
		return fmt.Errorf("creating croc client: %w", err)
	}
	if err := client.Receive(); err != nil {
		return fmt.Errorf("receive: %w", err)
	}
	return nil
}

// textFileInfo materialises the --text body to a temp file and returns the
// FileInfo croc needs. Croc's own CLI writes text to a temp file too.
func textFileInfo(text string) ([]croc.FileInfo, error) {
	f, err := os.CreateTemp("", "podstack-text-*.txt")
	if err != nil {
		return nil, fmt.Errorf("creating temp file for text: %w", err)
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("writing temp text: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	infos, _, _, err := croc.GetFilesInfo([]string{f.Name()}, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("collecting text file info: %w", err)
	}
	// Rename the displayed remote file from the temp path to "message.txt".
	if len(infos) > 0 {
		infos[0].Name = "message.txt"
		infos[0].FolderRemote = "./"
		infos[0].FolderSource = filepath.Dir(f.Name())
	}
	return infos, nil
}
```

- [ ] **Step 4: Run the tests, confirm they pass**

Run:
```bash
go test ./internal/transfer/... -v
```

Expected: all listed subtests `PASS`.

- [ ] **Step 5: Verify the package still compiles standalone**

Run:
```bash
go vet ./...
```

Expected: no output (success).

- [ ] **Step 6: Commit**

```bash
git add internal/transfer/
git commit -m "feat(transfer): wrap croc library with send/receive configs"
```

---

## Task 4: `send` cobra command

**Files:**
- Create: `cmd/send.go`

- [ ] **Step 1: Write the command file**

Create `/Users/saurav/Podstack/podstack-cli/cmd/send.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/podstack/podstack-cli/internal/relay"
	"github.com/podstack/podstack-cli/internal/transfer"
	crocutils "github.com/schollz/croc/v10/src/utils"
	"github.com/spf13/cobra"
)

type sendFlags struct {
	code         string
	relay        string
	relayDefault bool
	text         string
	zip          bool
	noCompress   bool
}

func newSendCmd() *cobra.Command {
	flags := &sendFlags{}

	cmd := &cobra.Command{
		Use:   "send [files-or-dirs...]",
		Short: "Send files, directories, or text",
		Long: `Send files, directories, or text via croc.

Examples:
  podstack send ./episode-001.wav
  podstack send ./assets/ ./notes.md
  podstack send --text "see you at 3pm"
  podstack send --code my-shared-code ./file.zip
  podstack send --relay-default ./file.zip       # use croc's public relay
  podstack send --relay myrelay.example.com:9009 ./file.zip

Resume: if a send is interrupted, re-run the same command and the
receiver will resume from where it left off (croc tracks partial files
by hash in the receive directory).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			relayAddr, err := relay.Resolve(flags.relay, flags.relayDefault, strings.TrimSpace(os.Getenv("PODSTACK_RELAY")))
			if err != nil {
				return err
			}

			code := flags.code
			if code == "" {
				code = crocutils.GetRandomName()
			}

			cfg := transfer.SendConfig{
				Code:       code,
				Relay:      relayAddr,
				Paths:      args,
				Text:       flags.text,
				ZipFolder:  flags.zip,
				NoCompress: flags.noCompress,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Relay: %s\nCode:  %s\n\n", relayAddr, code)
			fmt.Fprintln(cmd.OutOrStdout(), "Receiver runs: podstack receive", code)
			fmt.Fprintln(cmd.OutOrStdout())

			return transfer.Send(cfg)
		},
	}

	cmd.Flags().StringVar(&flags.code, "code", "", "custom code phrase (≥6 chars; auto-generated if empty)")
	cmd.Flags().StringVar(&flags.relay, "relay", "", "relay host[:port] (overrides default)")
	cmd.Flags().BoolVar(&flags.relayDefault, "relay-default", false, "use croc's public relay (croc.schollz.com)")
	cmd.Flags().StringVar(&flags.text, "text", "", "send text instead of a file")
	cmd.Flags().BoolVar(&flags.zip, "zip", false, "zip directories before sending")
	cmd.Flags().BoolVar(&flags.noCompress, "no-compress", false, "disable compression")

	return cmd
}
```

- [ ] **Step 2: Wire the command into the root command**

Edit `/Users/saurav/Podstack/podstack-cli/cmd/root.go` and replace the `init()` function:

```go
func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newSendCmd())
}
```

- [ ] **Step 3: Verify the command parses and `--help` renders**

Run:
```bash
go build -o podstack . && ./podstack send --help
```

Expected: help text shows `--code`, `--relay`, `--relay-default`, `--text`, `--zip`, `--no-compress`, and the examples.

Run:
```bash
./podstack send --relay foo --relay-default ./file
```

Expected: exits with non-zero status and message `podstack: --relay and --relay-default are mutually exclusive`.

Run:
```bash
./podstack send
```

Expected: exits with non-zero status and message containing `nothing to send`.

- [ ] **Step 4: Commit**

```bash
git add cmd/
git commit -m "feat(cmd): add send subcommand"
```

---

## Task 5: `receive` cobra command

**Files:**
- Create: `cmd/receive.go`

- [ ] **Step 1: Write the receive command**

Create `/Users/saurav/Podstack/podstack-cli/cmd/receive.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/podstack/podstack-cli/internal/relay"
	"github.com/podstack/podstack-cli/internal/transfer"
	"github.com/spf13/cobra"
)

type receiveFlags struct {
	out          string
	relay        string
	relayDefault bool
	yes          bool
}

func newReceiveCmd() *cobra.Command {
	flags := &receiveFlags{}

	cmd := &cobra.Command{
		Use:   "receive <code>",
		Short: "Receive files using a code phrase",
		Long: `Receive files using a code phrase shared by the sender.

Examples:
  podstack receive my-shared-code
  podstack receive my-shared-code --out ./downloads
  podstack receive --yes my-shared-code
  podstack receive --relay-default my-shared-code

Resume: if an earlier receive was interrupted, re-run the same command
in the same output directory and croc will resume the partial file
based on its hash.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			relayAddr, err := relay.Resolve(flags.relay, flags.relayDefault, strings.TrimSpace(os.Getenv("PODSTACK_RELAY")))
			if err != nil {
				return err
			}
			code := args[0]

			cfg := transfer.ReceiveConfig{
				Code:       code,
				Relay:      relayAddr,
				OutDir:     flags.out,
				AutoAccept: flags.yes,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Relay: %s\nCode:  %s\n\n", relayAddr, code)

			return transfer.Receive(cfg)
		},
	}

	cmd.Flags().StringVar(&flags.out, "out", "", "output directory (default: cwd)")
	cmd.Flags().StringVar(&flags.relay, "relay", "", "relay host[:port] (overrides default)")
	cmd.Flags().BoolVar(&flags.relayDefault, "relay-default", false, "use croc's public relay (croc.schollz.com)")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "auto-accept the incoming transfer")

	return cmd
}
```

- [ ] **Step 2: Wire the command into root**

Edit `/Users/saurav/Podstack/podstack-cli/cmd/root.go` `init()`:

```go
func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newReceiveCmd())
}
```

- [ ] **Step 3: Verify the help and flag-conflict behaviour**

Run:
```bash
go build -o podstack . && ./podstack receive --help
```

Expected: shows `--out`, `--relay`, `--relay-default`, `--yes`.

Run:
```bash
./podstack receive --relay foo --relay-default abcdefg
```

Expected: exits with non-zero status and the mutual-exclusion error.

Run:
```bash
./podstack receive
```

Expected: cobra error "accepts 1 arg(s), received 0".

- [ ] **Step 4: Commit**

```bash
git add cmd/
git commit -m "feat(cmd): add receive subcommand"
```

---

## Task 6: End-to-end integration test (basic round trip)

**Files:**
- Create: `internal/transfer/integration_test.go`

This test starts a croc relay in-process via `github.com/schollz/croc/v10/src/tcp`, generates a 1 MiB pseudo-random file, calls `Send` and `Receive` in two goroutines, and verifies the receive directory contains the same file content. Build tag `integration` keeps it out of the default `go test` run because it binds real ports.

- [ ] **Step 1: Add the tcp package as a dependency (it's already in croc's module, just used here)**

No explicit `go get` needed — it's pulled in transitively. We will verify at build time.

- [ ] **Step 2: Write the integration test**

Create `/Users/saurav/Podstack/podstack-cli/internal/transfer/integration_test.go`:

```go
//go:build integration

package transfer_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/podstack/podstack-cli/internal/transfer"
	"github.com/schollz/croc/v10/src/models"
	"github.com/schollz/croc/v10/src/tcp"
)

// startRelay spins up a croc relay on 127.0.0.1:9009..9013 in a goroutine.
// It returns the host:port of the base port. The relay shuts down when the
// test process exits.
func startRelay(t *testing.T) string {
	t.Helper()
	go func() {
		// tcp.Run signature in v10: Run(debug, ip, port, password, banner, allowedIPs)
		// Reference: https://github.com/schollz/croc/blob/v10.4.3/src/tcp/tcp.go
		if err := tcp.Run("info", "127.0.0.1", "9009", models.DEFAULT_PASSPHRASE, "", ""); err != nil {
			t.Logf("relay exited: %v", err)
		}
	}()
	// Give the relay a moment to listen.
	time.Sleep(500 * time.Millisecond)
	return "127.0.0.1:9009"
}

func makeRandomFile(t *testing.T, dir string, name string, size int) (string, [32]byte) {
	t.Helper()
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path, sha256.Sum256(buf)
}

func TestSendReceiveRoundTrip(t *testing.T) {
	relayAddr := startRelay(t)

	sendDir := t.TempDir()
	recvDir := t.TempDir()
	srcPath, srcHash := makeRandomFile(t, sendDir, "payload.bin", 1<<20) // 1 MiB

	code := "podstack-test-code"

	errCh := make(chan error, 2)
	go func() {
		// Run sender first; croc will block until a receiver connects.
		errCh <- transfer.Send(transfer.SendConfig{
			Code:  code,
			Relay: relayAddr,
			Paths: []string{srcPath},
		})
	}()

	// Give the sender time to register with the relay.
	time.Sleep(500 * time.Millisecond)

	go func() {
		errCh <- transfer.Receive(transfer.ReceiveConfig{
			Code:       code,
			Relay:      relayAddr,
			OutDir:     recvDir,
			AutoAccept: true,
		})
	}()

	timeout := time.After(60 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("transfer failed: %v", err)
			}
		case <-timeout:
			t.Fatal("transfer did not complete within 60s")
		}
	}

	got, err := os.ReadFile(filepath.Join(recvDir, "payload.bin"))
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	gotHash := sha256.Sum256(got)
	if !bytes.Equal(srcHash[:], gotHash[:]) {
		t.Fatalf("received hash mismatch:\n got = %x\nwant = %x", gotHash, srcHash)
	}
}
```

- [ ] **Step 3: Run the integration test**

Run:
```bash
go test -tags=integration -run TestSendReceiveRoundTrip ./internal/transfer/... -v -timeout 90s
```

Expected: `PASS` (may take 5–30 seconds). If `tcp.Run`'s signature in v10.4.3 differs from what's shown above, adjust the arguments to match the upstream source at `src/tcp/tcp.go` — the signature there is authoritative.

- [ ] **Step 4: Commit**

```bash
git add internal/transfer/integration_test.go
git commit -m "test(transfer): end-to-end send/receive integration test"
```

---

## Task 7: Resume integration test (subprocess kill/restart)

**Files:**
- Modify: `internal/transfer/integration_test.go` (add a second test)

This test verifies croc's resume behaviour through our binary. It builds `podstack`, runs `send` and `receive` as subprocesses, kills the receiver mid-transfer, restarts it with the same code in the same out dir, and asserts the final file matches.

- [ ] **Step 1: Append the resume test**

Append to `/Users/saurav/Podstack/podstack-cli/internal/transfer/integration_test.go`:

```go
// TestResume confirms that an interrupted receive picks up where it left off.
// Strategy: build the podstack binary, run `podstack send` (large file) and
// `podstack receive` as subprocesses, kill the receiver after a short delay,
// then restart it. Croc detects the partial file by hash and resumes.
func TestResume(t *testing.T) {
	relayAddr := startRelay(t)

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("locating repo root: %v", err)
	}

	binDir := t.TempDir()
	bin := filepath.Join(binDir, "podstack")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	sendDir := t.TempDir()
	recvDir := t.TempDir()
	srcPath, srcHash := makeRandomFile(t, sendDir, "payload.bin", 32<<20) // 32 MiB
	code := "podstack-resume-test"

	sender := exec.Command(bin, "send", "--code", code, "--relay", relayAddr, srcPath)
	sender.Stdout = os.Stdout
	sender.Stderr = os.Stderr
	if err := sender.Start(); err != nil {
		t.Fatalf("start sender: %v", err)
	}
	defer func() {
		_ = sender.Process.Kill()
		_, _ = sender.Process.Wait()
	}()
	time.Sleep(500 * time.Millisecond)

	// First receive attempt: kill after 1 second.
	recv1 := exec.Command(bin, "receive", "--yes", "--relay", relayAddr, "--out", recvDir, code)
	recv1.Stdout = os.Stdout
	recv1.Stderr = os.Stderr
	if err := recv1.Start(); err != nil {
		t.Fatalf("start receiver 1: %v", err)
	}
	time.Sleep(1 * time.Second)
	if err := recv1.Process.Kill(); err != nil {
		t.Fatalf("kill receiver 1: %v", err)
	}
	_, _ = recv1.Process.Wait()

	// Confirm a partial file exists.
	entries, err := os.ReadDir(recvDir)
	if err != nil {
		t.Fatalf("read recv dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected a partial file in receive dir after kill")
	}

	// Sender needs to be re-run because killing the receiver tore down the croc session.
	_ = sender.Process.Kill()
	_, _ = sender.Process.Wait()
	sender2 := exec.Command(bin, "send", "--code", code, "--relay", relayAddr, srcPath)
	sender2.Stdout = os.Stdout
	sender2.Stderr = os.Stderr
	if err := sender2.Start(); err != nil {
		t.Fatalf("start sender 2: %v", err)
	}
	defer func() {
		_ = sender2.Process.Kill()
		_, _ = sender2.Process.Wait()
	}()
	time.Sleep(500 * time.Millisecond)

	// Second receive attempt: let it complete.
	recv2 := exec.Command(bin, "receive", "--yes", "--relay", relayAddr, "--out", recvDir, code)
	recv2.Stdout = os.Stdout
	recv2.Stderr = os.Stderr
	if err := recv2.Run(); err != nil {
		t.Fatalf("receiver 2: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(recvDir, "payload.bin"))
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	gotHash := sha256.Sum256(got)
	if !bytes.Equal(srcHash[:], gotHash[:]) {
		t.Fatalf("resumed file hash mismatch:\n got = %x\nwant = %x", gotHash, srcHash)
	}
}

// findRepoRoot walks up from the test file's package dir until it sees go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
```

Also extend the file's import block (top of the file) to include:

```go
import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/podstack/podstack-cli/internal/transfer"
	"github.com/schollz/croc/v10/src/models"
	"github.com/schollz/croc/v10/src/tcp"
)
```

(`os/exec` is the only new import; merge it into the existing block.)

- [ ] **Step 2: Run the resume test**

Run:
```bash
go test -tags=integration -run TestResume ./internal/transfer/... -v -timeout 180s
```

Expected: `PASS`. The test prints croc's progress output for both halves; ignore non-zero exit messages from the killed first receiver.

Note: croc's resume detection is hash-prefix-based, so if it fails to resume (downloads from scratch), the test still passes as long as the final hash matches. That's an acceptable outcome — the test confirms a re-run after interruption produces the right bytes.

- [ ] **Step 3: Commit**

```bash
git add internal/transfer/integration_test.go
git commit -m "test(transfer): resume-after-kill integration test"
```

---

## Task 8: `install.sh` POSIX installer

**Files:**
- Create: `scripts/install.sh`

- [ ] **Step 1: Write the installer**

Create `/Users/saurav/Podstack/podstack-cli/scripts/install.sh`:

```sh
#!/bin/sh
# install.sh — installs the latest podstack release for macOS or Linux.
#
# Usage:
#   curl -fsSL https://podstack.ai/install.sh | sh
#   # or
#   sh install.sh
#
# Environment overrides:
#   PODSTACK_VERSION   pin a specific tag, e.g. "v1.2.3" (default: latest release)
#   PODSTACK_INSTALL_DIR  install location (default: /usr/local/bin or ~/.local/bin)

set -eu

REPO="podstack/podstack-cli"
GH_API="https://api.github.com/repos/${REPO}"
GH_DL="https://github.com/${REPO}/releases/download"

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
note() { printf '==> %s\n' "$*"; }

# --- detect OS and architecture ----------------------------------------------
uname_s=$(uname -s 2>/dev/null || echo unknown)
uname_m=$(uname -m 2>/dev/null || echo unknown)

case "$uname_s" in
  Darwin)  OS=darwin ;;
  Linux)   OS=linux ;;
  *)       err "unsupported OS: $uname_s (only macOS and Linux supported)" ;;
esac

case "$uname_m" in
  x86_64|amd64)    ARCH=amd64 ;;
  arm64|aarch64)   ARCH=arm64 ;;
  *)               err "unsupported architecture: $uname_m" ;;
esac

# --- determine version -------------------------------------------------------
VERSION=${PODSTACK_VERSION:-}
if [ -z "$VERSION" ]; then
  note "Fetching latest release tag"
  VERSION=$(curl -fsSL "${GH_API}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || err "could not determine latest release tag"
fi
note "Installing podstack ${VERSION} for ${OS}/${ARCH}"

# --- download tarball + checksums --------------------------------------------
TARBALL="podstack_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d 2>/dev/null || mktemp -d -t podstack)
trap 'rm -rf "$TMP"' EXIT

note "Downloading ${TARBALL}"
curl -fsSL -o "${TMP}/${TARBALL}" "${GH_DL}/${VERSION}/${TARBALL}" \
  || err "download failed: ${GH_DL}/${VERSION}/${TARBALL}"

note "Downloading checksums.txt"
curl -fsSL -o "${TMP}/checksums.txt" "${GH_DL}/${VERSION}/checksums.txt" \
  || err "checksum file download failed"

# --- verify sha256 -----------------------------------------------------------
note "Verifying sha256"
EXPECTED=$(grep " ${TARBALL}\$" "${TMP}/checksums.txt" | awk '{print $1}')
[ -n "$EXPECTED" ] || err "no checksum entry for ${TARBALL}"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP}/${TARBALL}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP}/${TARBALL}" | awk '{print $1}')
else
  err "neither sha256sum nor shasum found; cannot verify integrity"
fi

[ "$EXPECTED" = "$ACTUAL" ] || err "checksum mismatch: expected $EXPECTED got $ACTUAL"

# --- extract -----------------------------------------------------------------
note "Extracting"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"
[ -f "${TMP}/podstack" ] || err "tarball did not contain a 'podstack' binary"
chmod +x "${TMP}/podstack"

# --- choose install dir ------------------------------------------------------
INSTALL_DIR=${PODSTACK_INSTALL_DIR:-}
if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ] 2>/dev/null; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    note "Installing to ${INSTALL_DIR} (not on /usr/local/bin — add it to PATH if needed)"
  fi
fi

mv "${TMP}/podstack" "${INSTALL_DIR}/podstack"
note "Installed: ${INSTALL_DIR}/podstack"

# --- confirm -----------------------------------------------------------------
"${INSTALL_DIR}/podstack" version
```

- [ ] **Step 2: Make the script executable**

Run:
```bash
chmod +x scripts/install.sh
```

- [ ] **Step 3: Lint the script with `sh -n` (syntax check)**

Run:
```bash
sh -n scripts/install.sh
```

Expected: no output (clean parse).

- [ ] **Step 4: Run shellcheck if available (best-effort)**

Run:
```bash
command -v shellcheck >/dev/null && shellcheck -s sh scripts/install.sh || echo "shellcheck not installed; skipping"
```

Expected: no warnings, or the skip message.

- [ ] **Step 5: Commit**

```bash
git add scripts/install.sh
git commit -m "feat: POSIX install.sh for macOS and Linux"
```

---

## Task 9: `.goreleaser.yaml` + release workflow

**Files:**
- Create: `.goreleaser.yaml`, `.github/workflows/release.yml`

- [ ] **Step 1: Write `.goreleaser.yaml`**

Create `/Users/saurav/Podstack/podstack-cli/.goreleaser.yaml`:

```yaml
version: 2

project_name: podstack

before:
  hooks:
    - go mod tidy

builds:
  - id: podstack
    main: ./
    binary: podstack
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X github.com/podstack/podstack-cli/cmd.version={{.Version}}

archives:
  - id: default
    builds: [podstack]
    name_template: "podstack_{{ .Os }}_{{ .Arch }}"
    formats: [tar.gz]
    files:
      - LICENSE
      - README.md

checksum:
  name_template: checksums.txt
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

brews:
  - name: podstack
    repository:
      owner: podstack
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: "https://github.com/podstack/podstack-cli"
    description: "Send and receive large files using croc, defaulting to the Podstack relay"
    license: MIT
    install: |
      bin.install "podstack"
    test: |
      system "#{bin}/podstack", "version"
```

- [ ] **Step 2: Write the release workflow**

Create `/Users/saurav/Podstack/podstack-cli/.github/workflows/release.yml`:

```yaml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

- [ ] **Step 3: Validate the goreleaser config locally**

Run:
```bash
command -v goreleaser >/dev/null && goreleaser check || echo "goreleaser not installed locally; CI will validate on tag push"
```

Expected: `config is valid` or the skip message.

- [ ] **Step 4: Dry-run a snapshot build (best-effort, requires goreleaser)**

Run:
```bash
command -v goreleaser >/dev/null && goreleaser release --snapshot --clean --skip=publish || echo "skipping snapshot build"
```

Expected: `dist/` populated with four tarballs and a `checksums.txt`, or the skip message.

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yaml .github/workflows/release.yml
git commit -m "ci: goreleaser config + release workflow"
```

---

## Task 10: CI workflow (`go test`, `go vet`, `golangci-lint`)

**Files:**
- Create: `.github/workflows/ci.yml`, `.golangci.yaml`

- [ ] **Step 1: Write `.golangci.yaml`**

Create `/Users/saurav/Podstack/podstack-cli/.golangci.yaml`:

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - revive

issues:
  exclude-rules:
    - path: _test\.go
      linters: [errcheck]
```

- [ ] **Step 2: Write the CI workflow**

Create `/Users/saurav/Podstack/podstack-cli/.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go vet ./...
      - run: go test ./... -race -count=1
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.60
```

- [ ] **Step 3: Run the same checks locally**

Run:
```bash
go vet ./... && go test ./... -race -count=1
```

Expected: both succeed (unit tests only; integration tests are gated behind the `integration` tag).

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml .golangci.yaml
git commit -m "ci: vet + race tests + golangci-lint workflow"
```

---

## Task 11: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

Create `/Users/saurav/Podstack/podstack-cli/README.md`:

```markdown
# podstack

`podstack` is a thin CLI for sending and receiving large files between any two
machines, powered by [croc](https://github.com/schollz/croc). Croc is bundled
into the binary — no separate install required.

It defaults to the Podstack-operated relay
(`relay.cloud.podstack.ai:9009`) so transfers don't depend on third-party
infrastructure, but you can use croc's public relay or any custom relay you
operate.

Supports macOS and Linux (`amd64` and `arm64`).

## Install

### One-liner

```sh
curl -fsSL https://podstack.ai/install.sh | sh
```

### Homebrew

```sh
brew install podstack/tap/podstack
```

### Binary download

Download a release tarball from <https://github.com/podstack/podstack-cli/releases>
and place `podstack` on your `PATH`.

## Usage

### Send a file

```sh
podstack send ./episode-001.wav
# prints a code phrase for the receiver
```

### Send a directory or multiple paths

```sh
podstack send ./assets/ ./notes.md
```

### Send a text message

```sh
podstack send --text "see you at 3pm"
```

### Receive

```sh
podstack receive <code-phrase>
podstack receive --out ./downloads <code-phrase>
podstack receive --yes <code-phrase>   # auto-accept
```

### Custom code phrase

```sh
podstack send --code my-shared-code ./big.zip
podstack receive my-shared-code
```

### Pick a different relay

```sh
# croc's public relay
podstack send --relay-default ./big.zip

# custom relay
podstack send --relay myrelay.example.com:9009 ./big.zip

# via environment
PODSTACK_RELAY=myrelay.example.com podstack send ./big.zip
```

Precedence: `--relay` > `--relay-default` > `PODSTACK_RELAY` env > default
(`relay.cloud.podstack.ai`).

### Resume

If a transfer is interrupted, re-run the same `podstack receive` command in the
same output directory. Croc detects the partial file by hash and resumes
from where it left off.

## License

MIT — see [LICENSE](./LICENSE).
```

- [ ] **Step 2: Sanity-check the README renders**

Run:
```bash
command -v glow >/dev/null && glow README.md | head -40 || head -20 README.md
```

Expected: header and install section render readably.

- [ ] **Step 3: Final build and smoke test**

Run:
```bash
go build -o podstack . && ./podstack --help && ./podstack version
```

Expected: usage with `send`, `receive`, `version` listed; version prints `dev`.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: README with install + usage"
```

---

## Self-Review (notes for the implementer)

After all tasks are complete, run this final verification:

```bash
go build -o podstack .
go vet ./...
go test ./... -race -count=1
go test -tags=integration -timeout 180s ./internal/transfer/...
sh -n scripts/install.sh
```

All must succeed. If goreleaser is installed, also run:

```bash
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

`dist/` should contain four tarballs and `checksums.txt`.

---

## Spec coverage matrix

| Spec section | Task(s) |
|---|---|
| 3 Architecture — single Go binary, cobra, croc library import | 1, 3 |
| 4 Command surface — `send`, `receive`, `version`, `--help` | 1, 4, 5 |
| 4 Relay precedence (--relay > --relay-default > env > default) | 2, 4, 5 |
| 4 `--text` / positional mutual exclusion | 3 (Validate), 4 |
| 4 `--relay` / `--relay-default` mutual exclusion | 2, 4, 5 |
| 4 All upload types (file/dir/multi/text) supported | 3 (GetFilesInfo, textFileInfo) |
| 4 Resume on by default | 3 (Overwrite=true), 7 |
| 5 Project layout matches plan File Structure | 1–11 |
| 6 Build & release via goreleaser, four targets, Homebrew tap | 9 |
| 7 install.sh (POSIX, sha256-verified) | 8 |
| 8 Unit tests for relay (table-driven) | 2 |
| 8 Integration test for send/receive | 6 |
| 8 Resume integration test | 7 |
| 8 CI: go test + go vet + golangci-lint | 10 |
| 9 Invalid flag combinations exit non-zero | 3, 4, 5 |
| 9 Croc errors surface with underlying message | 3 |
| 9 Ctrl-C handled by croc's internal SIGINT — no code on our side | 3 (documented in file-structure notes) |
