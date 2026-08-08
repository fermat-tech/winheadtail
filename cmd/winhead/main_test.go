package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/fermat-tech/winheadtail/shared"
)

// Every expectation here was captured from GNU coreutils 8.32 on the project's
// test VM, not derived from the man page. The obsolescent `-NUM` form has more
// rules than it looks: it is only valid as the first argument, its trailing
// letters can switch between lines and bytes (and scale the count), and the
// last mode letter wins.

// capture runs parseFlags with the exit path intercepted, returning the
// options, the file operands, and the message a bad option produced.
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
		args     []string
		lines    int64
		bytes    int64
		useBytes bool
		quiet    bool
		verbose  bool
	}{
		{args: []string{"-3", "f"}, lines: 3},
		{args: []string{"-03", "f"}, lines: 3},
		{args: []string{"-0", "f"}, lines: 0},
		{args: []string{"-1", "f"}, lines: 1},
		{args: []string{"-999", "f"}, lines: 999},
		// Trailing letters pick the unit and scale the count.
		{args: []string{"-3c", "f"}, bytes: 3, useBytes: true},
		{args: []string{"-1b", "f"}, bytes: 512, useBytes: true},
		{args: []string{"-1k", "f"}, bytes: 1024, useBytes: true},
		{args: []string{"-1m", "f"}, bytes: 1024 * 1024, useBytes: true},
		{args: []string{"-3l", "f"}, lines: 3},
		// The last mode letter wins.
		{args: []string{"-3cl", "f"}, lines: 3},
		{args: []string{"-3lc", "f"}, bytes: 3, useBytes: true},
		// Header flags combine with the count.
		{args: []string{"-3q", "f"}, lines: 3, quiet: true},
		{args: []string{"-3v", "f"}, lines: 3, verbose: true},
		{args: []string{"-3lq", "f"}, lines: 3, quiet: true},
		// A later -n overrides the obsolescent count.
		{args: []string{"-3", "-n", "5", "f"}, lines: 5},
		// The count also works with several files and with -q after it.
		{args: []string{"-2", "a", "b"}, lines: 2},
		{args: []string{"-3", "-q", "a", "b"}, lines: 3, quiet: true},
	}

	for _, tt := range tests {
		opts, _, errMsg := capture(t, tt.args...)
		if errMsg != "" {
			t.Errorf("%v: unexpected error %q", tt.args, errMsg)
			continue
		}
		if opts.useBytes != tt.useBytes {
			t.Errorf("%v: useBytes = %v, want %v", tt.args, opts.useBytes, tt.useBytes)
		}
		if tt.useBytes {
			if opts.bytes != tt.bytes {
				t.Errorf("%v: bytes = %d, want %d", tt.args, opts.bytes, tt.bytes)
			}
		} else if opts.lines != tt.lines {
			t.Errorf("%v: lines = %d, want %d", tt.args, opts.lines, tt.lines)
		}
		if opts.quiet != tt.quiet {
			t.Errorf("%v: quiet = %v, want %v", tt.args, opts.quiet, tt.quiet)
		}
		if opts.verbose != tt.verbose {
			t.Errorf("%v: verbose = %v, want %v", tt.args, opts.verbose, tt.verbose)
		}
	}
}

// TestObsoleteCountPositionOnly covers the rule that trips people up: the count
// is only obsolescent syntax as the very first argument.
func TestObsoleteCountPositionOnly(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"-q", "-3", "f"}, "invalid trailing option -- 3"},
		{[]string{"-n", "5", "-3", "f"}, "invalid trailing option -- 3"},
		{[]string{"-3", "-3", "f"}, "invalid trailing option -- 3"},
		{[]string{"-3x", "f"}, "invalid trailing option -- x"},
		{[]string{"-x", "f"}, "invalid option -- 'x'"},
	}
	for _, tt := range tests {
		_, _, errMsg := capture(t, tt.args...)
		if !strings.Contains(errMsg, tt.wantErr) {
			t.Errorf("%v: error = %q, want it to contain %q", tt.args, errMsg, tt.wantErr)
		}
	}
}

func TestExplicitCounts(t *testing.T) {
	tests := []struct {
		args     []string
		lines    int64
		bytes    int64
		useBytes bool
	}{
		{args: []string{"-n", "3", "f"}, lines: 3},
		{args: []string{"-n3", "f"}, lines: 3},
		{args: []string{"-c", "3", "f"}, bytes: 3, useBytes: true},
		{args: []string{"-c3", "f"}, bytes: 3, useBytes: true},
		// A leading '+' is accepted and ignored, as GNU head does.
		{args: []string{"-n", "+3", "f"}, lines: 3},
		{args: []string{"-c", "+3", "f"}, bytes: 3, useBytes: true},
		// Zero is a legal count, and must not be mistaken for "unset".
		{args: []string{"-n", "0", "f"}, lines: 0},
		{args: []string{"-c", "0", "f"}, bytes: 0, useBytes: true},
		// The last of -n / -c wins.
		{args: []string{"-c", "5", "-n", "2", "f"}, lines: 2},
		{args: []string{"-n", "2", "-c", "5", "f"}, bytes: 5, useBytes: true},
		// Bundled with a header flag.
		{args: []string{"-qn3", "f"}, lines: 3},
	}
	for _, tt := range tests {
		opts, _, errMsg := capture(t, tt.args...)
		if errMsg != "" {
			t.Errorf("%v: unexpected error %q", tt.args, errMsg)
			continue
		}
		if opts.useBytes != tt.useBytes {
			t.Errorf("%v: useBytes = %v, want %v", tt.args, opts.useBytes, tt.useBytes)
		}
		if tt.useBytes && opts.bytes != tt.bytes {
			t.Errorf("%v: bytes = %d, want %d", tt.args, opts.bytes, tt.bytes)
		}
		if !tt.useBytes && opts.lines != tt.lines {
			t.Errorf("%v: lines = %d, want %d", tt.args, opts.lines, tt.lines)
		}
	}
}

func TestBadCounts(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"-n", "abc", "f"}, `invalid number of lines: "abc"`},
		{[]string{"-n", "1x", "f"}, `invalid number of lines: "1x"`},
		{[]string{"-c", "abc", "f"}, `invalid number of bytes: "abc"`},
		{[]string{"-n"}, "option requires an argument"},
		// GNU reports what is left after the sign, so these two differ.
		{[]string{"-c", "-abc", "f"}, `invalid number of bytes: "abc"`},
		{[]string{"-n", "+-3", "f"}, `invalid number of lines: "+-3"`},
		{[]string{"-n", "--", "f"}, `invalid number of lines: "-"`},
	}
	for _, tt := range tests {
		_, _, errMsg := capture(t, tt.args...)
		if !strings.Contains(errMsg, tt.wantErr) {
			t.Errorf("%v: error = %q, want it to contain %q", tt.args, errMsg, tt.wantErr)
		}
	}
}

// TestNegativeCounts covers `-n -N` / `-c -N`: all but the last N.
func TestNegativeCounts(t *testing.T) {
	tests := []struct {
		args       []string
		count      int64
		useBytes   bool
		exceptLast bool
	}{
		{args: []string{"-n", "-9", "f"}, count: 9, exceptLast: true},
		{args: []string{"-n-9", "f"}, count: 9, exceptLast: true},
		{args: []string{"-n", "-0", "f"}, count: 0, exceptLast: true},
		{args: []string{"-c", "-7", "f"}, count: 7, useBytes: true, exceptLast: true},
		{args: []string{"-c", "-0", "f"}, count: 0, useBytes: true, exceptLast: true},
		// GNU accepts both signs in this order, reading -+3 as -3.
		{args: []string{"-n", "-+3", "f"}, count: 3, exceptLast: true},
		// A positive count must not come out marked negative.
		{args: []string{"-n", "3", "f"}, count: 3},
		{args: []string{"-n", "+3", "f"}, count: 3},
		// A later positive count clears the negative form, and vice versa.
		{args: []string{"-n", "-3", "-n", "5", "f"}, count: 5},
		{args: []string{"-n", "5", "-n", "-3", "f"}, count: 3, exceptLast: true},
		// The obsolescent form is always "the first N".
		{args: []string{"-3", "-n", "-5", "f"}, count: 5, exceptLast: true},
		{args: []string{"-3", "f"}, count: 3},
	}
	for _, tt := range tests {
		opts, _, errMsg := capture(t, tt.args...)
		if errMsg != "" {
			t.Errorf("%v: unexpected error %q", tt.args, errMsg)
			continue
		}
		got := opts.lines
		if tt.useBytes {
			got = opts.bytes
		}
		if got != tt.count {
			t.Errorf("%v: count = %d, want %d", tt.args, got, tt.count)
		}
		if opts.useBytes != tt.useBytes {
			t.Errorf("%v: useBytes = %v, want %v", tt.args, opts.useBytes, tt.useBytes)
		}
		if opts.exceptLast != tt.exceptLast {
			t.Errorf("%v: exceptLast = %v, want %v", tt.args, opts.exceptLast, tt.exceptLast)
		}
	}
}

// TestHeadLinesExceptLast checks the hold-back window against GNU's output for
// a 12-line input, including the cases where N meets or exceeds the input.
func TestHeadLinesExceptLast(t *testing.T) {
	input := ""
	for i := 1; i <= 12; i++ {
		input += fmt.Sprintf("%d\n", i)
	}

	tests := []struct {
		n    int64
		want string
	}{
		{9, "1\n2\n3\n"},
		{0, input}, // -n -0 removes nothing
		{1, lines(1, 11)},
		{11, "1\n"},
		{12, ""},
		{13, ""},
		{99, ""},
	}
	for _, tt := range tests {
		got := captureStdout(t, func() {
			if err := headLinesExceptLast(strings.NewReader(input), tt.n); err != nil {
				t.Fatalf("-n -%d: %v", tt.n, err)
			}
		})
		if got != tt.want {
			t.Errorf("-n -%d = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestHeadBytesExceptLast(t *testing.T) {
	const input = "abcdefghij" // 10 bytes
	tests := []struct {
		n    int64
		want string
	}{
		{7, "abc"},
		{0, input},
		{1, "abcdefghi"},
		{10, ""},
		{99, ""},
	}
	for _, tt := range tests {
		got := captureStdout(t, func() {
			if err := headBytesExceptLast(strings.NewReader(input), tt.n); err != nil {
				t.Fatalf("-c -%d: %v", tt.n, err)
			}
		})
		if got != tt.want {
			t.Errorf("-c -%d = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestHoldBackWindowStreams guards the reason the window exists: the input is
// not read into memory, so a count far larger than any sane buffer, and an
// input far larger than the count, both stay cheap and correct.
func TestHoldBackWindowStreams(t *testing.T) {
	var sb strings.Builder
	const total = 50000
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	got := captureStdout(t, func() {
		if err := headLinesExceptLast(strings.NewReader(sb.String()), total-3); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if got != "1\n2\n3\n" {
		t.Errorf("kept %q, want the first three lines", got)
	}
}

// TestByteFidelity is the regression guard for silent stream corruption.
// winhead sits in pipelines, so what it writes has to be what it read: a final
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
			name:  "unterminated last line is not given a newline",
			input: "a\nb",
			run:   func(r io.Reader) error { return headLines(r, 2) },
			want:  "a\nb",
		},
		{
			name:  "a terminated line keeps its newline",
			input: "a\nb",
			run:   func(r io.Reader) error { return headLines(r, 1) },
			want:  "a\n",
		},
		{
			name:  "reading past the end adds nothing",
			input: "a\nb",
			run:   func(r io.Reader) error { return headLines(r, 99) },
			want:  "a\nb",
		},
		{
			name:  "CRLF survives",
			input: "a\r\nb\r\n",
			run:   func(r io.Reader) error { return headLines(r, 1) },
			want:  "a\r\n",
		},
		{
			name:  "CRLF survives in full",
			input: "a\r\nb\r\n",
			run:   func(r io.Reader) error { return headLines(r, 2) },
			want:  "a\r\nb\r\n",
		},
		{
			name:  "a lone CR is data, not a terminator",
			input: "a\rb\n",
			run:   func(r io.Reader) error { return headLines(r, 1) },
			want:  "a\rb\n",
		},
		{
			name:  "negative form: the dropped line was the unterminated one",
			input: "a\nb",
			run:   func(r io.Reader) error { return headLinesExceptLast(r, 1) },
			want:  "a\n",
		},
		{
			name:  "negative form keeps an unterminated line it does not drop",
			input: "a\nb\nc",
			run:   func(r io.Reader) error { return headLinesExceptLast(r, 0) },
			want:  "a\nb\nc",
		},
		{
			name:  "negative form preserves CRLF",
			input: "a\r\nb\r\nc\r\n",
			run:   func(r io.Reader) error { return headLinesExceptLast(r, 1) },
			want:  "a\r\nb\r\n",
		},
		{
			name:  "bytes mode is a byte copy",
			input: "a\r\nb",
			run:   func(r io.Reader) error { return headBytes(r, 4) },
			want:  "a\r\nb",
		},
		{
			name:  "negative bytes mode is a byte copy",
			input: "a\r\nb",
			run:   func(r io.Reader) error { return headBytesExceptLast(r, 1) },
			want:  "a\r\n",
		},
		{
			name:  "empty input yields nothing",
			input: "",
			run:   func(r io.Reader) error { return headLines(r, 3) },
			want:  "",
		},
		{
			name:  "a file that is just a newline yields just a newline",
			input: "\n",
			run:   func(r io.Reader) error { return headLines(r, 3) },
			want:  "\n",
		},
		{
			name:  "blank lines are preserved, not collapsed",
			input: "a\n\n\nb\n",
			run:   func(r io.Reader) error { return headLines(r, 4) },
			want:  "a\n\n\nb\n",
		},
		{
			name:  "NUL bytes pass through",
			input: "a\x00b\nc\n",
			run:   func(r io.Reader) error { return headLines(r, 1) },
			want:  "a\x00b\n",
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

func TestWindowCapIsBounded(t *testing.T) {
	// A huge count must not be preallocated; the window grows to fit the input.
	if got := windowCap(1 << 30); got > 4096 {
		t.Errorf("windowCap(1<<30) = %d, want it capped at 4096", got)
	}
	if got := windowCap(9); got != 9 {
		t.Errorf("windowCap(9) = %d, want 9", got)
	}
}

// lines renders the inclusive range as newline-terminated decimal lines.
func lines(from, to int) string {
	var sb strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	return sb.String()
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
	opts, rest, errMsg := capture(t, "-3", "a.txt", "b.txt")
	if errMsg != "" {
		t.Fatalf("unexpected error %q", errMsg)
	}
	if opts.lines != 3 {
		t.Errorf("lines = %d, want 3", opts.lines)
	}
	if len(rest) != 2 || rest[0] != "a.txt" || rest[1] != "b.txt" {
		t.Errorf("operands = %v, want [a.txt b.txt]", rest)
	}

	// Everything after -- is a file, even if it looks like an option.
	_, rest, errMsg = capture(t, "-3", "--", "-weird.txt")
	if errMsg != "" {
		t.Fatalf("unexpected error %q", errMsg)
	}
	if len(rest) != 1 || rest[0] != "-weird.txt" {
		t.Errorf("operands = %v, want [-weird.txt]", rest)
	}
}

func TestScaleSaturates(t *testing.T) {
	// A count vastly past the end of any file must not wrap into a negative.
	if got := scale(1<<62, 1024*1024); got <= 0 {
		t.Errorf("scale(1<<62, 1Mi) = %d, want a large positive value", got)
	}
	if got := scale(0, 512); got != 0 {
		t.Errorf("scale(0, 512) = %d, want 0", got)
	}
	if got := scale(3, 512); got != 1536 {
		t.Errorf("scale(3, 512) = %d, want 1536", got)
	}
}
