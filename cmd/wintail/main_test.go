package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/fermat-tech/winheadtail/shared"
)

// Expectations captured from GNU coreutils 8.32 on the project's test VM.
//
// tail's obsolescent `-NUM` form is much stricter than head's: GNU accepts it
// only as a lone first argument with at most one file, and rejects everything
// else with "option used in invalid context" rather than guessing. The sign
// also carries meaning here -- `-n 3` is the last three lines while `-n +3` is
// everything from line three on -- so the two must never be conflated.

// capture runs parseFlags with the exit path intercepted.
func capture(t *testing.T, args ...string) (opts *options, rest []string, errMsg string) {
	t.Helper()
	saved := fatal
	defer func() { fatal = saved }()

	type stop struct{}
	fatal = func(format string, a ...any) {
		errMsg = fmt.Sprintf(format, a...)
		panic(stop{})
	}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(stop); !ok {
				panic(r)
			}
		}
	}()

	opts, rest = parseFlags(args)
	return opts, rest, errMsg
}

func TestObsoleteCount(t *testing.T) {
	tests := []struct {
		args      []string
		count     int64
		useBytes  bool
		fromStart bool
	}{
		{args: []string{"-3", "f"}, count: 3},
		{args: []string{"-3"}, count: 3},
		{args: []string{"-0", "f"}, count: 0},
		{args: []string{"-999", "f"}, count: 999},
		{args: []string{"-3l", "f"}, count: 3},
		{args: []string{"-3c", "f"}, count: 3, useBytes: true},
		{args: []string{"-1b", "f"}, count: 512, useBytes: true},
		// The '+' form counts forward from the start of the file.
		{args: []string{"+3", "f"}, count: 3, fromStart: true},
		{args: []string{"+3c", "f"}, count: 3, useBytes: true, fromStart: true},
		// A bare "-" is stdin, not an option, so the context stays valid.
		{args: []string{"-3", "-"}, count: 3},
	}
	for _, tt := range tests {
		opts, _, errMsg := capture(t, tt.args...)
		if errMsg != "" {
			t.Errorf("%v: unexpected error %q", tt.args, errMsg)
			continue
		}
		if opts.count != tt.count {
			t.Errorf("%v: count = %d, want %d", tt.args, opts.count, tt.count)
		}
		if opts.useBytes != tt.useBytes {
			t.Errorf("%v: useBytes = %v, want %v", tt.args, opts.useBytes, tt.useBytes)
		}
		if opts.fromStart != tt.fromStart {
			t.Errorf("%v: fromStart = %v, want %v", tt.args, opts.fromStart, tt.fromStart)
		}
	}
}

// TestObsoleteCountInvalidContext covers GNU's restriction: the obsolescent
// count must stand alone, with no other option and no more than one file.
func TestObsoleteCountInvalidContext(t *testing.T) {
	for _, args := range [][]string{
		{"-3", "-q", "f"},
		{"-q", "-3", "f"},
		{"-n", "5", "-3", "f"},
		{"-3", "-n", "5", "f"},
		{"-3", "a", "b"}, // more than one file
		{"-3q", "f"},     // q is not a valid trailing letter for tail
		{"-3v", "f"},
		{"-3r", "f"},
		{"-3x", "f"},
	} {
		_, _, errMsg := capture(t, args...)
		if !strings.Contains(errMsg, "option used in invalid context") {
			t.Errorf("%v: error = %q, want it to mention an invalid context", args, errMsg)
		}
	}
}

// TestPlusFormIsNotTheMinusForm is the regression guard for the bug this
// replaced: `-n +3` used to be parsed as `-n 3`, silently printing the last
// three lines instead of everything from line three on.
func TestPlusFormIsNotTheMinusForm(t *testing.T) {
	minus, _, err := capture(t, "-n", "3", "f")
	if err != "" {
		t.Fatalf("unexpected error %q", err)
	}
	plus, _, err := capture(t, "-n", "+3", "f")
	if err != "" {
		t.Fatalf("unexpected error %q", err)
	}
	if minus.fromStart {
		t.Error(`-n 3 set fromStart, want it counting back from the end`)
	}
	if !plus.fromStart {
		t.Error(`-n +3 did not set fromStart, so it would print the last 3 lines instead`)
	}
	if minus.count != 3 || plus.count != 3 {
		t.Errorf("counts = %d and %d, want 3 and 3", minus.count, plus.count)
	}
}

func TestExplicitCounts(t *testing.T) {
	tests := []struct {
		args      []string
		count     int64
		useBytes  bool
		fromStart bool
	}{
		{args: []string{"-n", "3", "f"}, count: 3},
		{args: []string{"-n3", "f"}, count: 3},
		{args: []string{"-c", "3", "f"}, count: 3, useBytes: true},
		{args: []string{"-c+8", "f"}, count: 8, useBytes: true, fromStart: true},
		{args: []string{"-c", "+8", "f"}, count: 8, useBytes: true, fromStart: true},
		{args: []string{"-n", "+1", "f"}, count: 1, fromStart: true},
		{args: []string{"-n", "+0", "f"}, count: 0, fromStart: true},
		// GNU reads `-n -3` the same as `-n 3`.
		{args: []string{"-n", "-3", "f"}, count: 3},
		{args: []string{"-c", "-3", "f"}, count: 3, useBytes: true},
		// Zero is a legal count and must not read as "unset".
		{args: []string{"-c", "0", "f"}, count: 0, useBytes: true},
		{args: []string{"-n", "0", "f"}, count: 0},
		// The last of -n / -c wins.
		{args: []string{"-n", "2", "-c", "5", "f"}, count: 5, useBytes: true},
		{args: []string{"-c", "5", "-n", "2", "f"}, count: 2},
	}
	for _, tt := range tests {
		opts, _, errMsg := capture(t, tt.args...)
		if errMsg != "" {
			t.Errorf("%v: unexpected error %q", tt.args, errMsg)
			continue
		}
		if opts.count != tt.count {
			t.Errorf("%v: count = %d, want %d", tt.args, opts.count, tt.count)
		}
		if opts.useBytes != tt.useBytes {
			t.Errorf("%v: useBytes = %v, want %v", tt.args, opts.useBytes, tt.useBytes)
		}
		if opts.fromStart != tt.fromStart {
			t.Errorf("%v: fromStart = %v, want %v", tt.args, opts.fromStart, tt.fromStart)
		}
	}
}

func TestBadCounts(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"-n", "abc", "f"}, "invalid number of lines"},
		{[]string{"-n", "+abc", "f"}, "invalid number of lines"},
		{[]string{"-c", "abc", "f"}, "invalid number of bytes"},
		{[]string{"-n"}, "option requires an argument"},
		{[]string{"-z", "f"}, "invalid option -- 'z'"},
	}
	for _, tt := range tests {
		_, _, errMsg := capture(t, tt.args...)
		if !strings.Contains(errMsg, tt.wantErr) {
			t.Errorf("%v: error = %q, want it to contain %q", tt.args, errMsg, tt.wantErr)
		}
	}
}

// TestStartOffset pins the arithmetic both output paths share.
func TestStartOffset(t *testing.T) {
	tests := []struct {
		n         int64
		fromStart bool
		total     int64
		want      int64
	}{
		// Counting back from the end.
		{3, false, 12, 9},
		{12, false, 12, 0},
		{999, false, 12, 0}, // a count past the start yields the whole file
		{0, false, 12, 12},  // -n 0 yields nothing
		// Counting forward from the start; +0 and +1 both mean the beginning.
		{1, true, 12, 0},
		{0, true, 12, 0},
		{3, true, 12, 2},
		{12, true, 12, 11},
		{13, true, 12, 12}, // just past the end yields nothing
		{99, true, 12, 12}, // well past the end also yields nothing, not a panic
		// Degenerate input.
		{3, false, 0, 0},
		{3, true, 0, 0},
	}
	for _, tt := range tests {
		if got := startOffset(tt.n, tt.fromStart, tt.total); got != tt.want {
			t.Errorf("startOffset(%d, %v, %d) = %d, want %d",
				tt.n, tt.fromStart, tt.total, got, tt.want)
		}
	}
}

// TestByteFidelity is the regression guard for silent stream corruption.
// wintail sits in pipelines, so what it writes has to be what it read: a final
// line that arrived without a newline must not gain one, and a CRLF file must
// not come out LF-only. Both used to happen, because bufio.ScanLines drops the
// terminator and strips a preceding '\r'.
func TestByteFidelity(t *testing.T) {
	tests := []struct {
		name  string
		input string
		run   func(io.Reader) error
		want  string
	}{
		{
			name:  "the last line keeps its missing newline missing",
			input: "a\nb",
			run:   func(r io.Reader) error { return tailLines(r, 1, false) },
			want:  "b",
		},
		{
			name:  "whole file, unterminated",
			input: "a\nb",
			run:   func(r io.Reader) error { return tailLines(r, 2, false) },
			want:  "a\nb",
		},
		{
			name:  "from-start form, unterminated",
			input: "a\nb",
			run:   func(r io.Reader) error { return tailLines(r, 2, true) },
			want:  "b",
		},
		{
			name:  "from line 1, unterminated",
			input: "a\nb",
			run:   func(r io.Reader) error { return tailLines(r, 1, true) },
			want:  "a\nb",
		},
		{
			name:  "CRLF survives",
			input: "a\r\nb\r\n",
			run:   func(r io.Reader) error { return tailLines(r, 1, false) },
			want:  "b\r\n",
		},
		{
			name:  "CRLF survives in the from-start form",
			input: "a\r\nb\r\n",
			run:   func(r io.Reader) error { return tailLines(r, 2, true) },
			want:  "b\r\n",
		},
		{
			name:  "a lone CR is data, not a terminator",
			input: "a\rb\n",
			run:   func(r io.Reader) error { return tailLines(r, 1, false) },
			want:  "a\rb\n",
		},
		{
			name:  "bytes mode is a byte copy",
			input: "a\r\nb",
			run:   func(r io.Reader) error { return tailBytes(r, 3, false) },
			want:  "\r\nb",
		},
		{
			name:  "empty input yields nothing",
			input: "",
			run:   func(r io.Reader) error { return tailLines(r, 3, false) },
			want:  "",
		},
		{
			name:  "blank lines are preserved, not collapsed",
			input: "a\n\n\nb\n",
			run:   func(r io.Reader) error { return tailLines(r, 4, false) },
			want:  "a\n\n\nb\n",
		},
		{
			name:  "NUL bytes pass through",
			input: "a\nb\x00c\n",
			run:   func(r io.Reader) error { return tailLines(r, 1, false) },
			want:  "b\x00c\n",
		},
	}

	for _, tt := range tests {
		got := captureStdout(t, func() {
			if err := tt.run(strings.NewReader(tt.input)); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
		})
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

// captureStdout swaps the shared writer for the duration of fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	saved := shared.Stdout
	defer func() { shared.Stdout = saved }()
	var buf bytes.Buffer
	shared.Stdout = &buf
	fn()
	return buf.String()
}

func TestFileOperands(t *testing.T) {
	opts, rest, errMsg := capture(t, "-n", "3", "a.txt", "b.txt")
	if errMsg != "" {
		t.Fatalf("unexpected error %q", errMsg)
	}
	if opts.count != 3 {
		t.Errorf("count = %d, want 3", opts.count)
	}
	if len(rest) != 2 || rest[0] != "a.txt" || rest[1] != "b.txt" {
		t.Errorf("operands = %v, want [a.txt b.txt]", rest)
	}
}

func TestScaleSaturates(t *testing.T) {
	if got := scale(1<<62, 512); got <= 0 {
		t.Errorf("scale(1<<62, 512) = %d, want a large positive value", got)
	}
	if got := scale(3, 512); got != 1536 {
		t.Errorf("scale(3, 512) = %d, want 1536", got)
	}
}
