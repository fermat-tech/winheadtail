package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/fermat-tech/winheadtail/shared"
)

var progName = shared.ProgName()

// version is what --version reports. It is empty here so that the release
// builds can stamp the tag in with -ldflags "-X main.version=vX.Y.Z";
// otherwise shared.PrintVersion resolves it with shared.BuildVersion.
var version = ""

// ---- options ----

type options struct {
	lines      int64 // -n
	bytes      int64 // -c
	useBytes   bool  // -c was given — distinct from bytes==0, which is legal
	exceptLast bool  // the -N forms: print all but the last N, not the first N
	quiet      bool  // -q: never print headers
	verbose    bool  // -v: always print headers

	flagColor   bool
	flagNoColor bool
}

// ---- fatal / warn ----

// fatal is a variable so the tests can intercept the exit path and assert on
// the message a bad option produces.
var fatal = func(format string, args ...any) {
	fmt.Fprintf(shared.Stderr, progName+": "+format+"\n", args...)
	os.Exit(1)
}

func warn(format string, args ...any) {
	fmt.Fprintf(shared.Stderr, progName+": "+format+"\n", args...)
}

// tryHelp is the trailer GNU puts on every option error.
func tryHelp() string {
	return fmt.Sprintf("\nTry '%s --help' for more information.", progName)
}

// ---- flag parsing ----

func parseFlags(args []string) (*options, []string) {
	opts := &options{lines: 10}
	var rest []string

	i := 0
	// The obsolescent count -- `head -3` for `head -n 3` -- is only recognized
	// as the very first argument, exactly as GNU does it: `head -3 f` is fine
	// while `head -q -3 f` and `head -3 -3 f` are errors. Everywhere else a
	// digit is a bad option, not a count.
	if len(args) > 0 && isObsoleteCount(args[0]) {
		applyObsoleteCount(args[0], opts)
		i = 1
	}

	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		switch arg {
		case "--color":
			opts.flagColor = true
			i++
			continue
		case "--no-color":
			opts.flagNoColor = true
			i++
			continue
		case "--help", "-h":
			usage()
		case "--version":
			shared.PrintVersion(version)
			os.Exit(0)
		case "--quiet", "--silent":
			opts.quiet = true
			i++
			continue
		case "--verbose":
			opts.verbose = true
			i++
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// handle -n N  -c N  combined short flags
			runes := []rune(arg[1:])
			j := 0
			for j < len(runes) {
				switch c := runes[j]; {
				case c == 'n':
					j++
					opts.lines, opts.exceptLast = parseCount("n", string(runes[j:]), &i, args)
					opts.useBytes = false
					j = len(runes)
				case c == 'c':
					j++
					opts.bytes, opts.exceptLast = parseCount("c", string(runes[j:]), &i, args)
					opts.useBytes = true
					j = len(runes)
				case c == 'q':
					opts.quiet = true
					j++
				case c == 'v':
					opts.verbose = true
					j++
				case c >= '0' && c <= '9':
					fatal("invalid trailing option -- %c%s", c, tryHelp())
					j++
				default:
					fatal("invalid option -- '%c'%s", c, tryHelp())
					j++
				}
			}
			i++
			continue
		}
		rest = append(rest, arg)
		i++
	}
	return opts, rest
}

// isObsoleteCount reports whether arg is the historic `-NUM` count form.
func isObsoleteCount(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-' && arg[1] >= '0' && arg[1] <= '9'
}

// applyObsoleteCount parses `-NUM[bkmclqv]`. NUM is the count; b, k, m and c
// switch to bytes while scaling by 512, 1024, 1048576 and 1 respectively; l
// switches back to lines; q and v set the header flags. The letters are read
// left to right and the last mode wins, so `-3cl` is three lines and `-3lc` is
// three bytes.
func applyObsoleteCount(arg string, opts *options) {
	end := 1
	for end < len(arg) && arg[end] >= '0' && arg[end] <= '9' {
		end++
	}
	n, err := strconv.ParseInt(arg[1:end], 10, 64)
	if err != nil {
		fatal("invalid number of lines: %q", arg[1:end])
		return
	}

	opts.lines, opts.bytes, opts.useBytes, opts.exceptLast = n, n, false, false
	for _, c := range arg[end:] {
		switch c {
		case 'b':
			opts.bytes, opts.useBytes = scale(n, 512), true
		case 'k':
			opts.bytes, opts.useBytes = scale(n, 1024), true
		case 'm':
			opts.bytes, opts.useBytes = scale(n, 1024*1024), true
		case 'c':
			opts.bytes, opts.useBytes = n, true
		case 'l':
			opts.lines, opts.useBytes = n, false
		case 'q':
			opts.quiet = true
		case 'v':
			opts.verbose = true
		default:
			fatal("invalid trailing option -- %c%s", c, tryHelp())
			return
		}
	}
}

// scale multiplies without wrapping; a count past the end of the file behaves
// the same as one merely enormous, so saturating is the right answer.
func scale(n, mult int64) int64 {
	if n != 0 && n > math.MaxInt64/mult {
		return math.MaxInt64
	}
	return n * mult
}

// parseCount reads a count value either from the remainder of the current flag
// token or from the next argument.
func parseCount(flag, rem string, i *int, args []string) (int64, bool) {
	if rem != "" {
		return mustInt(flag, rem)
	}
	*i++
	if *i >= len(args) {
		fatal("option requires an argument -- '%s'%s", flag, tryHelp())
		return 0, false
	}
	return mustInt(flag, args[*i])
}

// mustInt parses a -n/-c value, reporting whether it was the negative form.
//
// A leading '-' flips the meaning from "the first N" to "all but the last N".
// A leading '+' is accepted and ignored, so `head -n +3` and `head -n 3` are
// the same request — GNU allows both signs together in that order, which is
// why `-+3` parses as -3 while `+-3` does not parse at all.
func mustInt(flag, s string) (n int64, exceptLast bool) {
	body := s
	if strings.HasPrefix(body, "-") {
		exceptLast = true
		body = body[1:]
	}
	digits := strings.TrimPrefix(body, "+")
	if digits == "" || strings.ContainsFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) {
		// GNU reports what is left after the sign, so `-c -abc` complains
		// about "abc" while `-n +-3` complains about the whole "+-3".
		fatal("invalid number of %s: %q", unitName(flag), body)
		return 0, false
	}
	v, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		fatal("invalid number of %s: %q", unitName(flag), body)
		return 0, false
	}
	return v, exceptLast
}

func unitName(flag string) string {
	if flag == "c" {
		return "bytes"
	}
	return "lines"
}

// ---- head logic ----

// headLines prints the first n lines, each exactly as it arrived — a final
// line with no newline of its own does not acquire one, and CRLF stays CRLF.
func headLines(r io.Reader, n int64) error {
	if n <= 0 {
		return nil // `head -n 0` prints nothing, rather than one line
	}
	sc := shared.NewLineScanner(r)
	var count int64
	for sc.Scan() {
		if _, err := shared.Stdout.Write(sc.Bytes()); err != nil {
			return err
		}
		if count++; count >= n {
			break
		}
	}
	return sc.Err()
}

// headLinesExceptLast implements `-n -N`: print every line but the last N.
//
// It holds back a window of N lines rather than reading the file in, so a
// gigabyte log costs N lines of memory and not a gigabyte. Once the window is
// full, each new line lets the oldest one out the front.
func headLinesExceptLast(r io.Reader, n int64) error {
	if n <= 0 {
		// `-n -0` removes nothing, so this is a plain copy of every line.
		return headLines(r, math.MaxInt64)
	}

	sc := shared.NewLineScanner(r)
	// The scanner reuses its buffer, so a held line has to be copied. Reusing
	// each slot's capacity on eviction keeps the steady state allocation-free.
	window := make([][]byte, 0, windowCap(n))
	oldest := 0
	full := false

	for sc.Scan() {
		if !full {
			window = append(window, append([]byte(nil), sc.Bytes()...))
			full = int64(len(window)) == n
			continue
		}
		// The window holds exactly N lines, so the one at the front is N lines
		// behind the newest and is therefore safe to print.
		if _, err := shared.Stdout.Write(window[oldest]); err != nil {
			return err
		}
		window[oldest] = append(window[oldest][:0], sc.Bytes()...)
		if oldest++; oldest == len(window) {
			oldest = 0
		}
	}
	return sc.Err()
}

// headBytesExceptLast implements `-c -N`: print every byte but the last N.
// Like its line counterpart it withholds only N bytes at a time.
func headBytesExceptLast(r io.Reader, n int64) error {
	if n <= 0 {
		_, err := io.Copy(shared.Stdout, r)
		return err
	}

	hold := make([]byte, 0, windowCap(n))
	buf := make([]byte, 32*1024)
	for {
		nr, err := r.Read(buf)
		if nr > 0 {
			hold = append(hold, buf[:nr]...)
			// Anything beyond the last N bytes can be released now.
			if excess := int64(len(hold)) - n; excess > 0 {
				if _, werr := shared.Stdout.Write(hold[:excess]); werr != nil {
					return werr
				}
				hold = append(hold[:0], hold[excess:]...)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// windowCap picks a starting capacity for a hold-back window. The window only
// ever grows to fit what the input actually contains, so an absurd count like
// `-n -1000000000` must not try to reserve room for it up front.
func windowCap(n int64) int {
	const max = 4096
	if n < max {
		return int(n)
	}
	return max
}

func headBytes(r io.Reader, n int64) error {
	buf := make([]byte, 32*1024)
	var written int64
	for written < n {
		want := int64(len(buf))
		if n-written < want {
			want = n - written
		}
		nr, err := r.Read(buf[:want])
		if nr > 0 {
			shared.Stdout.Write(buf[:nr])
			written += int64(nr)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func printHeader(name string) {
	fmt.Fprintln(shared.Stdout, shared.Col(shared.ColorHeader, "==> "+name+" <=="))
}

func processReader(r io.Reader, name string, opts *options, showHeader bool) {
	if showHeader {
		printHeader(name)
	}
	var err error
	switch {
	case opts.useBytes && opts.exceptLast:
		err = headBytesExceptLast(r, opts.bytes)
	case opts.useBytes:
		err = headBytes(r, opts.bytes)
	case opts.exceptLast:
		err = headLinesExceptLast(r, opts.lines)
	default:
		err = headLines(r, opts.lines)
	}
	if err != nil {
		warn("%s: %v", name, err)
	}
}

// ---- usage ----

func usage() {
	fmt.Fprintf(shared.Stderr, `Usage: %s [OPTIONS] [FILE...]

Print the first lines of each FILE to standard output.
With no FILE, reads from standard input.
Default is 10 lines.

Options:
  -n N        Print the first N lines (default 10)
  -n -N       Print all but the last N lines
  -c N        Print the first N bytes instead of lines
  -c -N       Print all but the last N bytes
  -q          Never print file headers
  -v          Always print file headers
  --color     Force color output on
  --no-color  Force color output off

Obsolescent form (first argument only):
  -N          Same as -n N, so %s -3 is %s -n 3
  -N[bkmc]    Bytes instead of lines, scaled by 512, 1024, 1048576 and 1
  -N[l]       Lines (the default)
  -N[qv]      Combine the count with -q or -v, as in %s -3v

Color control (lowest to highest priority):
  Auto           Enabled when stdout is a terminal
  WINHEAD_COLOR  Set to always, never, or auto
  NO_COLOR       Any value disables color (https://no-color.org)
  --color        Force enable
  --no-color     Force disable

Examples:
  %s file.txt
  %s -3 file.txt
  %s -n 20 file.txt
  %s -n -1 file.txt          everything except the last line
  %s -c 512 file.bin
  %s -v *.log
  %s -n 5 file1.txt file2.txt
`, progName, progName, progName, progName, progName, progName,
		progName, progName, progName, progName, progName)
	os.Exit(0)
}

// ---- main ----

func main() {
	opts, rest := parseFlags(os.Args[1:])
	shared.ResolveColor("WINHEAD_COLOR", opts.flagColor, opts.flagNoColor)

	files := shared.ExpandGlobs(rest)

	if len(files) == 0 {
		processReader(os.Stdin, "(standard input)", opts, false)
		return
	}

	// Headers shown when: multiple files, or -v; suppressed by -q.
	multiFile := len(files) > 1

	for idx, path := range files {
		showHeader := !opts.quiet && (multiFile || opts.verbose)

		if path == "-" {
			if idx > 0 {
				fmt.Fprintln(shared.Stdout)
			}
			processReader(os.Stdin, "standard input", opts, showHeader)
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			warn("cannot open %q: %v", path, err)
			continue
		}

		if idx > 0 && showHeader {
			fmt.Fprintln(shared.Stdout)
		}
		processReader(f, path, opts, showHeader)
		f.Close()
	}
}
