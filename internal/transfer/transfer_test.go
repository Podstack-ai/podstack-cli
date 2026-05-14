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
