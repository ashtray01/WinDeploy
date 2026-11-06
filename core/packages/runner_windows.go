//go:build windows

package packages

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode"
)

// WindowsRunner executes installers with a hidden console window. Output is kept
// to a bounded tail and never forwarded to the UI verbatim.
type WindowsRunner struct {
	// MaxOutputBytes bounds captured diagnostics. Zero uses 64 KiB.
	MaxOutputBytes int
}

// Run implements Runner. A missing executable is reported as a start error; a
// non-zero exit code is a normal Execution, not an error.
func (r WindowsRunner) Run(ctx context.Context, command Command, onLine func(string)) (Execution, error) {
	if strings.TrimSpace(command.Path) == "" {
		return Execution{}, fmt.Errorf("executable path is required")
	}
	path := command.Path
	if !strings.ContainsAny(path, `\/`) {
		resolved, err := exec.LookPath(path)
		if err != nil {
			return Execution{}, fmt.Errorf("executable was not found")
		}
		path = resolved
	}
	cmd := exec.CommandContext(ctx, path, command.Args...)
	cmd.Dir = command.Dir
	cmd.SysProcAttr = hiddenProcessAttributes()
	limit := r.MaxOutputBytes
	if limit <= 0 {
		limit = 64 * 1024
	}
	output := &boundedOutput{limit: limit, onLine: onLine}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	if ctx.Err() != nil {
		return Execution{ExitCode: -1, Output: output.String()}, ctx.Err()
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return Execution{ExitCode: exitErr.ExitCode(), Output: output.String()}, nil
		}
		return Execution{}, fmt.Errorf("installer process could not be started")
	}
	return Execution{ExitCode: 0, Output: output.String()}, nil
}

// hiddenProcessAttributes prevents a console window from flashing for background
// probes and installers.
func hiddenProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

// boundedOutput keeps the last limit bytes and optionally reports sanitized lines.
type boundedOutput struct {
	buffer    bytes.Buffer
	limit     int
	line      []byte
	onLine    func(string)
	truncated bool
}

func (w *boundedOutput) Write(data []byte) (int, error) {
	written := len(data)
	if w.onLine != nil {
		w.line = append(w.line, data...)
		for {
			index := bytes.IndexByte(w.line, '\n')
			if index < 0 {
				break
			}
			line := sanitizeLine(string(w.line[:index]))
			w.line = append(w.line[:0], w.line[index+1:]...)
			if line != "" {
				w.onLine(line)
			}
		}
		if len(w.line) > 4096 {
			w.line = w.line[:0]
		}
	}
	if len(data) >= w.limit {
		w.buffer.Reset()
		w.buffer.Write(data[len(data)-w.limit:])
		w.truncated = true
		return written, nil
	}
	if w.buffer.Len()+len(data) > w.limit {
		excess := w.buffer.Len() + len(data) - w.limit
		remaining := append([]byte{}, w.buffer.Bytes()[excess:]...)
		w.buffer.Reset()
		w.buffer.Write(remaining)
		w.truncated = true
	}
	w.buffer.Write(data)
	return written, nil
}

func (w *boundedOutput) String() string {
	value := sanitizeOutput(w.buffer.String())
	if w.truncated {
		value = "..." + value
	}
	return value
}

func sanitizeOutput(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	for i, line := range lines {
		lines[i] = sanitizeLine(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sanitizeLine(line string) string {
	var builder strings.Builder
	for _, r := range line {
		if r == '\t' {
			builder.WriteByte(' ')
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		builder.WriteRune(r)
		if builder.Len() >= 500 {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}
