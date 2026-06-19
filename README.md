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
| `-c N` | Print the first N bytes instead of lines |
| `-q` | Never print file headers |
| `-v` | Always print file headers |
| `--color` | Force color output on |
| `--no-color` | Force color output off |

### Examples

```powershell
winhead file.txt
winhead -n 20 file.txt
winhead -c 512 file.bin
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
| `-c N` | Print the last N bytes instead of lines |
| `-f` | Follow: watch the file and print new lines as they are appended |
| `-q` | Never print file headers |
| `-v` | Always print file headers |
| `--color` | Force color output on |
| `--no-color` | Force color output off |

### Examples

```powershell
wintail file.txt
wintail -n 20 file.txt
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

## Common behaviour

Both commands:
- Read from **stdin** when no file is given (use `-` to mix stdin with files)
- Expand **glob patterns** automatically (Windows shells don't)
- Show file headers (`==> filename <==`) automatically when multiple files are given
- Derive the **program name** from the executable stem — rename the binary and the help/errors update automatically

## License

MIT
