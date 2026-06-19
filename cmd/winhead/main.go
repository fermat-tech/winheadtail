package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fermat-tech/winheadtail/shared"
)

var progName = shared.ProgName()

// ---- options ----

type options struct {
	lines       int64 // -n
	bytes       int64 // -c  (0 = not set)
	quiet       bool  // -q: never print headers
	verbose     bool  // -v: always print headers
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
		case "--help", "-h":
			usage()
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
				switch runes[j] {
				case 'n':
					j++
					opts.lines = parseCount("n", string(runes[j:]), &i, args)
					j = len(runes)
				case 'c':
					j++
					opts.bytes = parseCount("c", string(runes[j:]), &i, args)
					j = len(runes)
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

// parseCount reads a count value either from the remainder of the current flag
// token or from the next argument.
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
	// Support +N syntax (some implementations use it)
	s = strings.TrimPrefix(s, "+")
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n < 0 {
		fatal("-%s: invalid number %q", flag, s)
	}
	return n
}

// ---- head logic ----

func headLines(r io.Reader, n int64) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var count int64
	for sc.Scan() {
		fmt.Fprintln(shared.Stdout, sc.Text())
		count++
		if count >= n {
			break
		}
	}
	return sc.Err()
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
	if opts.bytes > 0 {
		err = headBytes(r, opts.bytes)
	} else {
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
  -c N        Print the first N bytes instead of lines
  -q          Never print file headers
  -v          Always print file headers
  --color     Force color output on
  --no-color  Force color output off

Color control (lowest to highest priority):
  Auto           Enabled when stdout is a terminal
  WINHEAD_COLOR  Set to always, never, or auto
  NO_COLOR       Any value disables color (https://no-color.org)
  --color        Force enable
  --no-color     Force disable

Examples:
  %s file.txt
  %s -n 20 file.txt
  %s -c 512 file.bin
  %s -v *.log
  %s -n 5 file1.txt file2.txt
`, progName, progName, progName, progName, progName, progName)
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
