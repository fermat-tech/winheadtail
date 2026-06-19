// Package shared provides common utilities for winhead and wintail.
package shared

import (
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
