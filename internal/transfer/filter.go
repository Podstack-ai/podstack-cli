package transfer

import (
	"bytes"
	"errors"
	"io"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/creack/pty"
)

// suppressPatterns match complete \n-terminated lines from the underlying
// transfer engine that leak its name or its instructional invocations. The
// user-visible CLI is podstack; these lines never reach the terminal.
var suppressPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^Code is: `),
	regexp.MustCompile(`^On the other computer run:`),
	regexp.MustCompile(`^\(For Windows\)`),
	regexp.MustCompile(`^\(For Linux/macOS\)`),
	regexp.MustCompile(`^\s+croc `),
	regexp.MustCompile(`CROC_SECRET=`),
	regexp.MustCompile(`Code copied to clipboard`),
}

// suppressStarters are byte prefixes that a *partial* (not yet terminated)
// line might be growing into. While buffered data still matches one of
// these starts, we keep waiting for the rest of the line. Once it's clear
// the buffered data can't possibly become a suppressible line, we flush
// it on a timeout — this is how interactive prompts like
// "Accept 'X' (Y/n) " (which never include a newline) reach the user.
var suppressStarters = [][]byte{
	[]byte("Code is: "),
	[]byte("On the other computer run:"),
	[]byte("(For Windows)"),
	[]byte("(For Linux/macOS)"),
	[]byte("croc "), // handled after stripping leading whitespace
	[]byte("Code copied to clipboard"),
}

// flushTimeout is how long we'll hold partial output before assuming it's
// a prompt (or similar non-terminated chunk) and flushing it.
const flushTimeout = 50 * time.Millisecond

// suppressStderr installs a line filter over os.Stderr that drops engine-
// branded instructional lines while passing everything else (including
// \r-terminated progress-bar updates and interactive prompts) through.
//
// Backed by a PTY rather than a pipe so progress libraries that check
// isatty(2) continue to render in real time.
//
// If PTY allocation fails (e.g., a container without /dev/ptmx), this
// silently bails out and leaves os.Stderr alone. Transfers still work;
// only the line filter is lost.
func suppressStderr() func() {
	orig := os.Stderr
	ptmx, ttyFile, err := pty.Open()
	if err != nil {
		return func() {}
	}
	if size, err := pty.GetsizeFull(orig); err == nil {
		_ = pty.Setsize(ptmx, size)
	}
	os.Stderr = ttyFile

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		filterStream(ptmx, orig)
	}()

	return func() {
		_ = ttyFile.Close()
		wg.Wait()
		_ = ptmx.Close()
		os.Stderr = orig
	}
}

// filterStream copies from r to dst, dropping lines that match
// suppressPatterns. \r-terminated chunks (progress bars) are forwarded as
// soon as they arrive. Non-terminated partial data is held briefly so
// suppress-line matching can complete; if nothing else arrives for
// flushTimeout and the buffered prefix cannot grow into a suppressible
// line, it gets flushed — that's how interactive prompts reach the user.
func filterStream(r *os.File, dst io.Writer) {
	var buf bytes.Buffer
	chunk := make([]byte, 4096)
	for {
		_ = r.SetReadDeadline(time.Now().Add(flushTimeout))
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			drainBuffer(&buf, dst)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if buf.Len() > 0 && !couldBePartialSuppressLine(buf.Bytes()) {
				_, _ = dst.Write(buf.Bytes())
				buf.Reset()
			}
			continue
		}
		// EOF or pipe closed
		if buf.Len() > 0 {
			_, _ = dst.Write(buf.Bytes())
		}
		return
	}
}

func drainBuffer(buf *bytes.Buffer, dst io.Writer) {
	for {
		data := buf.Bytes()
		if len(data) == 0 {
			return
		}
		nl := bytes.IndexByte(data, '\n')
		if nl < 0 {
			cr := bytes.IndexByte(data, '\r')
			if cr < 0 {
				return // partial — handled by the timeout flush in filterStream
			}
			_, _ = dst.Write(data[:cr+1])
			buf.Next(cr + 1)
			continue
		}
		line := data[:nl+1]
		if !shouldSuppress(line) {
			_, _ = dst.Write(line)
		}
		buf.Next(nl + 1)
	}
}

// couldBePartialSuppressLine returns true when the buffered data could
// still grow into a line that shouldSuppress would drop. Used by the
// timeout flush to decide whether to wait for more data or release the
// buffer to the terminal.
func couldBePartialSuppressLine(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t")
	for _, s := range suppressStarters {
		n := len(trimmed)
		if n > len(s) {
			n = len(s)
		}
		if n == 0 {
			continue
		}
		if bytes.Equal(trimmed[:n], s[:n]) {
			return true
		}
	}
	// CROC_SECRET= can appear anywhere in a line; if it's present in the
	// buffered prefix, the whole line should be suppressed once \n lands.
	if bytes.Contains(data, []byte("CROC_SECRET=")) {
		return true
	}
	return false
}

func shouldSuppress(line []byte) bool {
	for _, re := range suppressPatterns {
		if re.Match(line) {
			return true
		}
	}
	return false
}
