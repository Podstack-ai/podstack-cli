//go:build integration

package transfer_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/podstack/podstack-cli/internal/transfer"
	"github.com/schollz/croc/v10/src/models"
	"github.com/schollz/croc/v10/src/tcp"
)

// startRelay spins up a croc relay on 127.0.0.1:9009..9013 in goroutines.
// It returns the host:port of the base port. The relay shuts down when the
// test process exits.
//
// This mirrors croc's own CLI relay() function in src/cli/cli.go: each
// transfer port (9010-9013) runs as its own goroutine, and the base port
// (9009) gets the comma-joined transfer port list as the variadic `banner`
// arg (croc uses this to advertise transfer ports to clients).
func startRelay(t *testing.T) string {
	t.Helper()
	ports := []string{"9009", "9010", "9011", "9012", "9013"}
	tcpPorts := strings.Join(ports[1:], ",")

	for _, p := range ports[1:] {
		port := p
		go func() {
			if err := tcp.Run("info", "127.0.0.1", port, models.DEFAULT_PASSPHRASE); err != nil {
				t.Logf("relay port %s exited: %v", port, err)
			}
		}()
	}
	go func() {
		if err := tcp.Run("info", "127.0.0.1", ports[0], models.DEFAULT_PASSPHRASE, tcpPorts); err != nil {
			t.Logf("relay base port exited: %v", err)
		}
	}()

	// Give the relay a moment to listen on all five ports.
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
		errCh <- transfer.Send(transfer.SendConfig{
			Code:  code,
			Relay: relayAddr,
			Paths: []string{srcPath},
		})
	}()

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
