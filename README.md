# winheadtail

Unix-like `head` and `tail` commands for Windows, written in Go.

Two binaries in one repo — `winhead` and `wintail` — sharing common file-reading, glob expansion, and color logic.

Rename either binary to anything you like — all usage and error messages derive from the executable name automatically.

## Installation

### go install (recommended)

Requires [Go](https://golang.org) 1.21+.

```powershell
go install github.com/fermat-tech/winheadtail/cmd/winhead@latest
go install github.com/fermat-tech/winheadtail/cmd/wintail@latest
```

Both binaries land in `%USERPROFILE%\go\bin`, which should already be on your `PATH`.

### Build from source

```powershell
git clone https://github.com/fermat-tech/winheadtail.git
cd winheadtail
go build -o winhead.exe ./cmd/winhead
go build -o wintail.exe ./cmd/wintail
```

### Download

Grab the latest binaries from [Releases](https://github.com/fermat-tech/winheadtail/releases).

---

## winhead

Print the **first** lines of each file (default: 10 lines).

### Usage

```
winhead [OPTIONS] [FILE...]
```

### Options

| Flag | Description |
|------|-------------|
| `-n N` | Print the first N lines (default 10) |
| `-n -N` | Print all but the last N lines |
| `-c N` | Print the first N bytes instead of lines |
| `-c -N` | Print all but the last N bytes |
| `-q` | Never print file headers |
| `-v` | Always print file headers |
| `--color` | Force color output on |
| `--no-color` | Force color output off |
| `--version` | Print version information and exit |
| `-h`, `--help` | Print usage and exit |

The sign matters: `-n 3` is the **first** three lines, while `-n -3` is
**everything but the last three**. `-n -0` removes nothing. Both forms stream —
the negative form holds back only N lines (or bytes), so trimming a trailer off
a multi-gigabyte log does not read it into memory. A `+` is accepted and
ignored, and GNU allows the two signs together in that order, so `-n -+3` is
`-n -3` while `-n +-3` is an error.

### The obsolescent `-NUM` form

`winhead -3` is `winhead -n 3`. As in GNU head, this form is only recognized as
the **first** argument — `winhead -3 -q f` is fine, `winhead -q -3 f` is an
error — and the digits may be followed by letters that pick the unit or set a
header flag:

| Form | Meaning |
|------|---------|
| `-N` | The first N lines |
| `-Nc` | The first N bytes |
| `-Nb` | The first N × 512 bytes |
| `-Nk` | The first N × 1024 bytes |
| `-Nm` | The first N × 1048576 bytes |
| `-Nl` | The first N lines (the default) |
| `-Nq`, `-Nv` | Combine the count with `-q` or `-v` |

Letters are read left to right and the last unit wins, so `-3cl` is three lines
and `-3lc` is three bytes.

### Examples

```powershell
winhead file.txt
winhead -3 file.txt
winhead -n 20 file.txt
winhead -n -1 file.txt        # everything except the last line
winhead -c 512 file.bin
winhead -3v file.txt
winhead -v *.log
winhead -n 5 file1.txt file2.txt
```

---

## wintail

Print the **last** lines of each file (default: 10 lines).

### Usage

```
wintail [OPTIONS] [FILE...]
```

### Options

| Flag | Description |
|------|-------------|
| `-n N` | Print the last N lines (default 10) |
| `-n +N` | Print from line N onwards, counting forward from the start |
| `-c N` | Print the last N bytes instead of lines |
| `-c +N` | Print from byte N onwards |
| `-f` | Follow: watch the file and print new lines as they are appended |
| `-q` | Never print file headers |
| `-v` | Always print file headers |
| `--color` | Force color output on |
| `--no-color` | Force color output off |
| `--version` | Print version information and exit |
| `-h`, `--help` | Print usage and exit |

The sign matters: `-n 3` is the **last** three lines, while `-n +3` is
**everything from line 3 on**. `+0` and `+1` both mean the whole file. A leading
`-` is accepted and ignored, so `-n -3` and `-n 3` are the same request.

### The obsolescent `-NUM` form

`wintail -3` is `wintail -n 3`, and `wintail +3` is `wintail -n +3`. GNU tail is
much stricter about this form than head: it must stand alone, as the first
argument, with no other option and at most one file — `tail -3 a b` is genuinely
ambiguous, so it is rejected rather than guessed at. wintail matches that,
reporting `option used in invalid context`.

| Form | Meaning |
|------|---------|
| `-N` | The last N lines |
| `+N` | From line N onwards |
| `-Nc` | The last N bytes |
| `-Nb` | The last N × 512 bytes |
| `-Nl` | The last N lines (the default) |

### Examples

```powershell
wintail file.txt
wintail -3 file.txt
wintail -n 20 file.txt
wintail -n +2 data.csv        # skip the header row
wintail -f app.log
wintail -c 1024 file.bin
wintail -v *.log
wintail -n 5 file1.txt file2.txt
```

---

## Color output

Color works on all Windows terminals including the old Command Prompt (cmd.exe).

| Method | Description |
|--------|-------------|
| `--color` | Force on |
| `--no-color` | Force off |
| `WINHEAD_COLOR=always\|never\|auto` | Persistent preference for winhead |
| `WINTAIL_COLOR=always\|never\|auto` | Persistent preference for wintail |
| `NO_COLOR=1` | Disable color for both ([no-color.org](https://no-color.org)) |
| Auto (default) | On when stdout is a terminal, off when piped |

---

## Common behavior

Both commands:
- Read from **stdin** when no file is given (use `-` to mix stdin with files)
- Expand **glob patterns** automatically (Windows shells don't)
- Show file headers (`==> filename <==`) automatically when multiple files are given
- Derive the **program name** from the executable stem — rename the binary and the help/errors update automatically
- Treat a count of `0` as a real count: `-n 0` and `-c 0` print nothing
- Emit exactly the bytes they read (see below)

## Byte fidelity

Both tools sit in the middle of pipelines, so what they write is what they read.
Lines are split on `\n` alone and passed through with their terminators intact,
which matters in two ways that are easy to get wrong:

- A final line that arrived **without** a newline does not acquire one.
  `printf 'a\nb' | winhead -n 2` is 3 bytes, not 4.
- **CRLF stays CRLF.** A `\r\n` file does not come out `\n`-only — which on
  Windows is the more likely of the two to bite you.

A lone `\r` is data, not a terminator; blank lines are preserved rather than
collapsed; and NUL bytes pass through. The test suites pin all of this down.

## Compatibility

Option handling is matched against GNU coreutils 8.32: the `-n` / `-c` sign
forms, the obsolescent `-NUM` forms with their position and context rules, the
byte-scaling letters, and each error message and exit status.

Output is verified byte for byte against GNU, including unterminated final
lines, CRLF input, the multi-file header separators, and empty files.

One deliberate difference: error messages quote with ASCII `"` where GNU uses
typographic quotes, matching the convention across this tool family.

## License

MIT
