package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fermat-tech/winheadtail/shared"
)

var progName = shared.ProgName()

// version is what --version reports. It is empty here so that the release
// builds can stamp the tag in with -ldflags "-X main.version=vX.Y.Z";
// otherwise shared.PrintVersion resolves it with shared.BuildVersion.
var version = ""

// ---- options ----

type options struct {
	count     int64 // the -n or -c value, per useBytes
	useBytes  bool  // -c was given — distinct from count==0, which is legal
	fromStart bool  // the +N forms: start at count, rather than counting back
	follow    bool  // -f
	quiet     bool  // -q
	verbose   bool  // -v

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

// tryHelp is the trailer GNU puts on most option errors.
func tryHelp() string {
	return fmt.Sprintf("\nTry '%s --help' for more information.", progName)
}

// ---- flag parsing ----

func parseFlags(args []string) (*options, []string) {
	opts := &options{count: 10}
	var rest []string

	i := 0
	// The obsolescent count -- `tail -3` for `tail -n 3`, `tail +3` for
	// `tail -n +3` -- is far more restricted than head's. GNU takes it only as
	// the first argument, only as the lone option, and only with at most one
	// file, because `tail -3 a b` is genuinely ambiguous. Everything else gets
	// rejected rather than guessed at.
	if len(args) > 0 && isObsoleteCount(args[0]) {
		if !obsoleteContextOK(args) {
			fatal("option used in invalid context -- %c", args[0][1])
		}
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
		case "--follow":
			opts.follow = true
			i++
			continue
		case "--quiet", "--silent":
			opts.quiet = true
			i++
			continue
		case "--verbose":
			opts.verbose = true
			i++
			continue
		case "--help", "-h":
			usage()
		case "--version":
			shared.PrintVersion(version)
			os.Exit(0)
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			runes := []rune(arg[1:])
			j := 0
			for j < len(runes) {
				switch c := runes[j]; {
				case c == 'n':
					j++
					opts.count, opts.fromStart = parseCount("n", string(runes[j:]), &i, args)
					opts.useBytes = false
					j = len(runes)
				case c == 'c':
					j++
					opts.count, opts.fromStart = parseCount("c", string(runes[j:]), &i, args)
					opts.useBytes = true
					j = len(runes)
				case c == 'f':
					opts.follow = true
					j++
				case c == 'q':
					opts.quiet = true
					j++
				case c == 'v':
					opts.verbose = true
					j++
				case c >= '0' && c <= '9':
					fatal("option used in invalid context -- %c", c)
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

// isObsoleteCount reports whether arg is the historic `-NUM` / `+NUM` form.
func isObsoleteCount(arg string) bool {
	return len(arg) >= 2 && (arg[0] == '-' || arg[0] == '+') && arg[1] >= '0' && arg[1] <= '9'
}

// obsoleteContextOK enforces GNU's restriction on the obsolescent count: it
// must stand alone, with no other option and no more than one file.
func obsoleteContextOK(args []string) bool {
	rest := args[1:]
	if len(rest) > 1 {
		return false
	}
	// A bare "-" is stdin, not an option.
	if len(rest) == 1 && strings.HasPrefix(rest[0], "-") && rest[0] != "-" {
		return false
	}
	return true
}

// applyObsoleteCount parses `[-+]NUM[bcl]`. NUM is the count; b and c switch to
// bytes, scaling by 512 and 1 respectively, and l switches back to lines. A
// leading '+' counts forward from the start of the file instead of back from
// the end. Unlike head, no other trailing letter is accepted.
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

	opts.count, opts.useBytes, opts.fromStart = n, false, arg[0] == '+'
	for _, c := range arg[end:] {
		switch c {
		case 'b':
			opts.count, opts.useBytes = scale(n, 512), true
		case 'c':
			opts.count, opts.useBytes = n, true
		case 'l':
			opts.count, opts.useBytes = n, false
		default:
			fatal("option used in invalid context -- %c", arg[1])
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

// mustInt parses a -n/-c value, reporting whether it was the '+' form.
//
// The sign carries the meaning here, and getting it wrong is silent: `-n 3`
// asks for the last three lines while `-n +3` asks for everything from line
// three onwards. A leading '-' is accepted and ignored, since GNU reads
// `-n -3` and `-n 3` alike.
func mustInt(flag, s string) (int64, bool) {
	body, fromStart := s, false
	switch {
	case strings.HasPrefix(body, "+"):
		fromStart, body = true, body[1:]
	case strings.HasPrefix(body, "-"):
		body = body[1:]
	}
	if body == "" || strings.ContainsFunc(body, func(r rune) bool { return r < '0' || r > '9' }) {
		fatal("invalid number of %s: %q", unitName(flag), s)
		return 0, false
	}
	n, err := strconv.ParseInt(body, 10, 64)
	if err != nil {
		fatal("invalid number of %s: %q", unitName(flag), s)
		return 0, false
	}
	return n, fromStart
}

func unitName(flag string) string {
	if flag == "c" {
		return "bytes"
	}
	return "lines"
}

// ---- tail logic ----

// readAllLines reads all lines from r into memory, each keeping its own
// terminator so the output can be byte-for-byte what the input was.
func readAllLines(r io.Reader) ([][]byte, error) {
	sc := shared.NewLineScanner(r)
	var lines [][]byte
	for sc.Scan() {
		// The scanner reuses its buffer, so every retained line is a copy.
		lines = append(lines, append([]byte(nil), sc.Bytes()...))
	}
	return lines, sc.Err()
}

// startOffset turns a count into an index into a sequence of length total.
// With fromStart the count is a 1-based position to begin at (+0 and +1 both
// mean the beginning); otherwise it is how many items to keep from the end.
func startOffset(n int64, fromStart bool, total int64) int64 {
	var start int64
	if fromStart {
		start = n - 1
	} else {
		start = total - n
	}
	if start < 0 {
		return 0
	}
	if start > total {
		return total
	}
	return start
}

// tailLines prints the last n lines from r, or everything from line n onwards
// when fromStart is set. Each line goes out exactly as it came in: an
// unterminated last line stays unterminated, and CRLF stays CRLF.
func tailLines(r io.Reader, n int64, fromStart bool) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	for _, l := range lines[startOffset(n, fromStart, int64(len(lines))):] {
		if _, err := shared.Stdout.Write(l); err != nil {
			return err
		}
	}
	return nil
}

// tailBytes prints the last n bytes from r, or everything from byte n onwards
// when fromStart is set.
func tailBytes(r io.Reader, n int64, fromStart bool) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	shared.Stdout.Write(data[startOffset(n, fromStart, int64(len(data))):])
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
	if opts.useBytes {
		err = tailBytes(r, opts.count, opts.fromStart)
	} else {
		err = tailLines(r, opts.count, opts.fromStart)
	}
	if err != nil {
		warn("%s: %v", name, err)
	}
}

// ---- follow mode ----

// followFile watches a file and prints new lines as they are appended.
// Pressing Ctrl+C stops it.
func followFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		fatal("cannot open %q: %v", path, err)
	}
	defer f.Close()

	// Seek to end before starting follow.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		fatal("%v", err)
	}

	fmt.Fprintf(shared.Stderr, progName+": following %q — press Ctrl+C to stop\n", path)

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Fprint(shared.Stdout, line)
		}
		if err == io.EOF {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if err != nil {
			warn("follow: %v", err)
			return
		}
	}
}

// ---- usage ----

func usage() {
	fmt.Fprintf(shared.Stderr, `Usage: %s [OPTIONS] [FILE...]

Print the last lines of each FILE to standard output.
With no FILE, reads from standard input.
Default is 10 lines.

Options:
  -n N        Print the last N lines (default 10)
  -n +N       Print from line N onwards instead of from the end
  -c N        Print the last N bytes instead of lines
  -c +N       Print from byte N onwards
  -f          Follow: keep the file open and print new lines as appended
  -q          Never print file headers
  -v          Always print file headers
  --color     Force color output on
  --no-color  Force color output off

Obsolescent form (must stand alone, with at most one FILE):
  -N          Same as -n N, so %s -3 is %s -n 3
  +N          Same as -n +N
  -N[bc]      Bytes instead of lines, scaled by 512 and 1
  -N[l]       Lines (the default)

Color control (lowest to highest priority):
  Auto           Enabled when stdout is a terminal
  WINTAIL_COLOR  Set to always, never, or auto
  NO_COLOR       Any value disables color (https://no-color.org)
  --color        Force enable
  --no-color     Force disable

Exit codes:
  0  Success
  1  Error

Examples:
  %s file.txt
  %s -3 file.txt
  %s -n 20 file.txt
  %s -n +2 file.csv          skip the header row
  %s -f app.log
  %s -c 1024 file.bin
  %s -v *.log
  %s -n 5 file1.txt file2.txt
`, progName, progName, progName, progName, progName, progName, progName,
		progName, progName, progName, progName)
	os.Exit(0)
}

// ---- main ----

func main() {
	opts, rest := parseFlags(os.Args[1:])
	shared.ResolveColor("WINTAIL_COLOR", opts.flagColor, opts.flagNoColor)

	files := shared.ExpandGlobs(rest)

	// Follow mode only works with a single file.
	if opts.follow {
		if len(files) == 0 {
			fatal("-f requires a file argument")
		}
		if len(files) > 1 {
			fatal("-f: only one file supported in follow mode")
		}
		// Print the last N lines first, then follow.
		f, err := os.Open(files[0])
		if err != nil {
			fatal("cannot open %q: %v", files[0], err)
		}
		processReader(f, files[0], opts, false)
		f.Close()
		followFile(files[0])
		return
	}

	if len(files) == 0 {
		processReader(os.Stdin, "(standard input)", opts, false)
		return
	}

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
