package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fermat-tech/winheadtail/shared"
)

var progName = shared.ProgName()

// ---- options ----

type options struct {
	lines       int64 // -n
	bytes       int64 // -c (0 = not set)
	follow      bool  // -f
	quiet       bool  // -q
	verbose     bool  // -v
	flagColor   bool
	flagNoColor bool
}

// ---- fatal / warn ----

func fatal(format string, args ...any) {
	fmt.Fprintf(shared.Stderr, progName+": "+format+"\n", args...)
	os.Exit(1)
}

func warn(format string, args ...any) {
	fmt.Fprintf(shared.Stderr, progName+": "+format+"\n", args...)
}

// ---- flag parsing ----

func parseFlags(args []string) (*options, []string) {
	opts := &options{lines: 10}
	var rest []string

	i := 0
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
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			runes := []rune(arg[1:])
			j := 0
			for j < len(runes) {
				switch runes[j] {
				case 'n':
					j++
					opts.lines = parseCount("n", string(runes[j:]), &i, args)
					j = len(runes)
				case 'c':
					j++
					opts.bytes = parseCount("c", string(runes[j:]), &i, args)
					j = len(runes)
				case 'f':
					opts.follow = true
					j++
				case 'q':
					opts.quiet = true
					j++
				case 'v':
					opts.verbose = true
					j++
				default:
					fatal("unknown option -%c", runes[j])
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

func parseCount(flag, rem string, i *int, args []string) int64 {
	if rem != "" {
		return mustInt(flag, rem)
	}
	*i++
	if *i >= len(args) {
		fatal("-%s requires a numeric argument", flag)
	}
	return mustInt(flag, args[*i])
}

func mustInt(flag, s string) int64 {
	s = strings.TrimPrefix(s, "+")
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n < 0 {
		fatal("-%s: invalid number %q", flag, s)
	}
	return n
}

// ---- tail logic ----

// readAllLines reads all lines from r into memory.
func readAllLines(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// tailLines prints the last n lines from r.
func tailLines(r io.Reader, n int64) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	start := int64(len(lines)) - n
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:] {
		fmt.Fprintln(shared.Stdout, l)
	}
	return nil
}

// tailBytes prints the last n bytes from r.
func tailBytes(r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	start := int64(len(data)) - n
	if start < 0 {
		start = 0
	}
	shared.Stdout.Write(data[start:])
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
	if opts.bytes > 0 {
		err = tailBytes(r, opts.bytes)
	} else {
		err = tailLines(r, opts.lines)
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
  -c N        Print the last N bytes instead of lines
  -f          Follow: keep the file open and print new lines as appended
  -q          Never print file headers
  -v          Always print file headers
  --color     Force color output on
  --no-color  Force color output off

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
  %s -n 20 file.txt
  %s -f app.log
  %s -c 1024 file.bin
  %s -v *.log
  %s -n 5 file1.txt file2.txt
`, progName, progName, progName, progName, progName, progName, progName)
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
