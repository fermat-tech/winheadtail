// Package shared provides common utilities for winhead and wintail.
package shared

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

// ---- program name ----

// ProgName returns the stem of the executable path (no extension).
func ProgName() string {
	name := filepath.Base(os.Args[0])
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// ---- version ----

// PrintVersion writes the banner this tool family shares, itself modeled on
// `bash --version`. Callers pass their own build-stamped version string.
func PrintVersion(version string) {
	fmt.Fprintf(Stdout, `%s, version %s
Copyright (c) 2026 fermat-tech
License: MIT <https://opensource.org/licenses/MIT>

This is free software; you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`, ProgName(), version)
}

// ---- output writers ----

// Stdout is a color-capable writer for standard output.
var Stdout io.Writer = colorable.NewColorableStdout()

// Stderr is a color-capable writer for standard error.
var Stderr io.Writer = colorable.NewColorableStderr()

// ---- color ----

// UseColor reports whether color output is enabled.
var UseColor bool

const (
	ColorReset  = "\033[0m"
	ColorHeader = "\033[1;36m" // bold cyan  — file headers
	ColorNum    = "\033[2;37m" // dim white  — line numbers (future use)
)

// ResolveColor sets UseColor based on flags, env vars, and terminal detection.
// Priority (highest wins): flagNoColor > flagColor > NO_COLOR > envVar > auto.
func ResolveColor(envVar string, flagColor, flagNoColor bool) {
	if flagNoColor {
		UseColor = false
		return
	}
	if flagColor {
		UseColor = true
		return
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		UseColor = false
		return
	}
	switch strings.ToLower(os.Getenv(envVar)) {
	case "always", "yes", "1", "true":
		UseColor = true
		return
	case "never", "no", "0", "false":
		UseColor = false
		return
	}
	UseColor = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// Col wraps s in an ANSI color code if color is enabled.
func Col(code, s string) string {
	if !UseColor {
		return s
	}
	return code + s + ColorReset
}

// ---- line scanning ----

// ScanLinesKeepEnding is bufio.ScanLines except that each token keeps its own
// terminator, and the terminator is only ever the '\n'.
//
// head and tail sit in the middle of byte streams, so what they write has to be
// what they read. bufio.ScanLines drops the terminator and strips a preceding
// '\r', which leaves a caller two ways to corrupt data: re-adding a '\n' invents
// a byte for a final line that never had one, and dropping the '\r' rewrites
// every CRLF file as LF. Splitting on '\n' alone and handing the line back
// intact avoids both.
func ScanLinesKeepEnding(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// NewLineScanner returns a scanner over r yielding whole lines with their
// terminators, sized to cope with long lines.
//
// As with any bufio.Scanner, the slice from Bytes() is only valid until the
// next Scan; a caller that holds lines must copy them.
func NewLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sc.Split(ScanLinesKeepEnding)
	return sc
}

// ---- glob expansion ----

// ExpandGlobs expands any path argument that contains glob metacharacters.
// Unmatched globs are passed through unchanged so callers can emit a proper error.
func ExpandGlobs(paths []string) []string {
	var out []string
	for _, p := range paths {
		if !strings.ContainsAny(p, "*?[") {
			out = append(out, p)
			continue
		}
		matches, err := filepath.Glob(p)
		if err != nil || len(matches) == 0 {
			out = append(out, p)
			continue
		}
		out = append(out, matches...)
	}
	return out
}
