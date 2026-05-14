package transfer

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"sync"

	"github.com/creack/pty"
)

// suppressPatterns are regexes matched against complete \n-terminated lines
// written to stderr by the underlying transfer engine. Matching lines are
// dropped before the user sees them. The patterns target instructional
// output that names the underlying engine or leaks its CLI invocations;
// the user-visible CLI is podstack.
var suppressPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^Code is: `),
	regexp.MustCompile(`^On the other computer run:`),
	regexp.MustCompile(`^\(For Windows\)`),
	regexp.MustCompile(`^\(For Linux/macOS\)`),
	regexp.MustCompile(`^\s+croc `),
	regexp.MustCompile(`CROC_SECRET=`),
	regexp.MustCompile(`Code copied to clipboard`),
}

// suppressStderr installs a line filter over os.Stderr that drops
// engine-branded instructional lines while passing everything else
// (including \r-terminated progress-bar updates) through unchanged.
//
// We back the swap with a PTY rather than a pipe so progress libraries that
// detect interactive terminals via isatty(2) continue to render in real
// time — a plain os.Pipe is not a TTY and causes those libraries to fall
// back to non-interactive output (which looks frozen to the user).
//
// If PTY allocation fails (e.g., a container without /dev/ptmx), we bail
// out and leave os.Stderr untouched. Transfers still work; the only loss
// is the line filter.
//
// Returns a restore function the caller must invoke (typically via defer)
// to flush the goroutine and restore os.Stderr.
func suppressStderr() func() {
	orig := os.Stderr
	ptmx, ttyFile, err := pty.Open()
	if err != nil {
		return func() {}
	}

	// Inherit the original terminal size so the progress bar's width math
	// (and any future tabular output) matches what the user actually sees.
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
// suppressPatterns. \r-terminated chunks (progress bars) are passed
// through immediately so the terminal can repaint them in place.
func filterStream(r io.Reader, dst io.Writer) {
	var buf bytes.Buffer
	chunk := make([]byte, 4096)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			drainBuffer(&buf, dst)
		}
		if err != nil {
			if buf.Len() > 0 {
				_, _ = dst.Write(buf.Bytes())
			}
			return
		}
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
				return // wait for more data
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

func shouldSuppress(line []byte) bool {
	for _, re := range suppressPatterns {
		if re.Match(line) {
			return true
		}
	}
	return false
}
