//go:build unix

package acceptance

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestTerminalWizardCancels(t *testing.T) {
	acceptance(t)
	bin := filepath.Join(t.TempDir(), "rubric")
	mustCommand(t, filepath.Join("..", ".."), nil, "go", "build", "-o", bin, "./cmd/rubric")
	target := t.TempDir()

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ptmx.Close() }()
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 30, Cols: 100}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(tty.Fd())
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bin, "init", target)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var output lockedBuffer
	go func() { _, _ = ioCopy(&output, ptmx) }()

	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(output.String(), "Module path") {
		if time.Now().After(deadline) {
			t.Fatalf("wizard did not start:\n%q", output.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := ptmx.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 130 {
		t.Fatalf("exit = %v\n%q", err, output.String())
	}
	after, err := term.GetState(tty.Fd())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("terminal state not restored")
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Fatalf("cancelled wizard wrote %d entries", len(entries))
	}
	_ = tty.Close()
}

func ioCopy(dst *lockedBuffer, src *os.File) (int64, error) {
	buf := make([]byte, 4096)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			_, _ = dst.Write(buf[:n])
			total += int64(n)
		}
		if err != nil {
			return total, err
		}
	}
}
