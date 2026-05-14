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

// basePort and DefaultTransfers mirror the underlying engine's defaults.
// The relay must expose basePort..basePort+Transfers.
const (
	basePort         = 9009
	DefaultTransfers = 4
)

// SendConfig is the user-facing send configuration.
type SendConfig struct {
	Code       string   // shared secret; must be >=6 chars
	Relay      string   // host:port
	Paths      []string // file/dir paths to send (mutually exclusive with Text)
	Text       string   // text body (mutually exclusive with Paths)
	ZipFolder  bool
	NoCompress bool
	Transfers  int // parallel TCP streams; 0 falls back to DefaultTransfers
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

func buildRelayPorts(transfers int) []string {
	if transfers <= 0 {
		transfers = DefaultTransfers
	}
	ports := make([]string, transfers+1)
	for i := 0; i <= transfers; i++ {
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
		RelayPorts:    buildRelayPorts(cfg.Transfers),
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

// Send performs the send half of the transfer handshake.
func Send(cfg SendConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	restore := suppressStderr()
	defer restore()
	opts := buildSendOptions(cfg)

	var (
		filesInfo    []croc.FileInfo
		emptyFolders []croc.FileInfo
		totalFolders int
		err          error
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

// Receive performs the receive half of the transfer handshake.
func Receive(cfg ReceiveConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	restore := suppressStderr()
	defer restore()
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
	if len(infos) > 0 {
		infos[0].Name = "message.txt"
		infos[0].FolderRemote = "./"
		infos[0].FolderSource = filepath.Dir(f.Name())
	}
	return infos, nil
}
