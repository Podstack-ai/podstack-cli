//go:build integration

package transfer_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"os/exec"
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
	srcPath, srcHash := makeRandomFile(t, sendDir, "payload.bin", 256<<20) // 256 MiB — large enough to leave a partial after the kill
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
	// Wait long enough for the sender to hash a 256 MiB file and register
	// with the relay. A bare 500ms is not enough on a slow disk/CI box.
	time.Sleep(3 * time.Second)

	// First receive attempt: kill after enough time for the secure-channel
	// handshake to complete and bytes to start landing on disk.
	recv1 := exec.Command(bin, "receive", "--yes", "--relay", relayAddr, "--out", recvDir, code)
	recv1.Stdout = os.Stdout
	recv1.Stderr = os.Stderr
	if err := recv1.Start(); err != nil {
		t.Fatalf("start receiver 1: %v", err)
	}
	time.Sleep(2 * time.Second)
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
	// Same generous wait — sender2 hashes the full file again before it's ready.
	time.Sleep(3 * time.Second)

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
