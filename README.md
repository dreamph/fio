# fio

A simple, lightweight Go library for common file operations. Designed for ease of use and low resource consumption.

## Installation

```bash
go get github.com/dreamph/fio
```

## Features

- Simple API for common file operations
- Automatic parent directory creation
- Atomic file writes with `SafeWrite`
- Memory-efficient streaming for large files
- Cross-device move support
- Zero external dependencies

## Quick Start

```go
package main

import (
	"fmt"
	"github.com/dreamph/fio"
)

func main() {
	// Write file
	fio.WriteString("/tmp/hello.txt", "Hello, World!", 0o644)

	// Read file
	content, _ := fio.ReadString("/tmp/hello.txt")
	fmt.Println(content)

	// Check existence
	if fio.Exists("/tmp/hello.txt") {
		fmt.Println("File exists!")
	}
}
```

## API Reference

### Read

| Function | Description |
|----------|-------------|
| `Read(path) ([]byte, error)` | Read entire file into memory |
| `ReadLimit(path, limit) ([]byte, error)` | Read file with size limit |
| `ReadAt(path, offset, length) ([]byte, error)` | Read partial file at offset |
| `ReadString(path) (string, error)` | Read file as string |
| `ReadLines(path, fn) error` | Read file line by line (memory efficient) |
| `ReadJSON(path, v) error` | Read JSON file into struct |
| `ReadJSONStream(path, v) error` | Read JSON using streaming decoder |
| `ReadStream(path, fn) error` | Stream read with callback (auto close) |

```go
// Read entire file
data, err := fio.Read("/path/to/file")

// Read with size limit (returns ErrSizeExceedsLimit if too large)
data, err := fio.ReadLimit("/path/to/file", 1024*1024) // 1MB limit

// Read partial file
chunk, err := fio.ReadAt("/path/to/file", 100, 50) // 50 bytes from offset 100

// Read line by line (memory efficient for large files)
err := fio.ReadLines("/path/to/file", func(line string) error {
    fmt.Println(line)
    return nil
})

// Read JSON
var config Config
err := fio.ReadJSON("/path/to/config.json", &config)

// Stream read (memory efficient, auto close)
err := fio.ReadStream("/path/to/large.bin", func(r io.Reader) error {
    _, err := io.Copy(dst, r)
    return err
})
```

### Write

| Function | Description |
|----------|-------------|
| `Write(path, data, perm) error` | Write bytes to file |
| `WriteString(path, s, perm) error` | Write string to file |
| `WriteJSON(path, v, perm) error` | Write struct as indented JSON |
| `SafeWrite(path, data, perm) error` | Atomic write (temp + fsync + rename) |
| `Append(path, data, perm) error` | Append bytes to file |
| `AppendLine(path, line, perm) error` | Append line with newline |

```go
// Write file (creates parent directories automatically)
err := fio.Write("/path/to/file", []byte("content"), 0o644)

// Write string
err := fio.WriteString("/path/to/file", "content", 0o644)

// Write JSON (pretty printed)
err := fio.WriteJSON("/path/to/config.json", config, 0o644)

// Atomic write (safe for config files)
err := fio.SafeWrite("/path/to/config.json", data, 0o644)

// Append to file
err := fio.Append("/path/to/log", []byte("log entry"), 0o644)

// Append line
err := fio.AppendLine("/path/to/log", "log entry", 0o644)
```

### Temp

| Function | Description |
|----------|-------------|
| `CreateTemp(dir, pattern) (string, error)` | Create empty temp file |
| `WriteTemp(dir, pattern, data) (string, error)` | Create temp file with content |

```go
// Create temp file
path, err := fio.CreateTemp("", "myapp-*.txt")
defer fio.Remove(path)

// Create temp file with content
path, err := fio.WriteTemp("", "data-*.json", jsonData)
defer fio.Remove(path)
```

### Info

| Function | Description |
|----------|-------------|
| `Exists(path) bool` | Check if path exists |
| `ExistsWithError(path) (bool, error)` | Check existence with error handling |
| `IsDir(path) (bool, error)` | Check if path is directory |
| `IsFile(path) (bool, error)` | Check if path is regular file |
| `IsSymlink(path) (bool, error)` | Check if path is symbolic link |
| `Size(path) (int64, error)` | Get file size in bytes |
| `ModTime(path) (time.Time, error)` | Get modification time |
| `FileInfo(path) (os.FileInfo, error)` | Get full file info |

```go
// Check existence
if fio.Exists("/path/to/file") {
    // file exists
}

// Get file size
size, err := fio.Size("/path/to/file")

// Get modification time
modTime, err := fio.ModTime("/path/to/file")

// Check type
isDir, err := fio.IsDir("/path")
isFile, err := fio.IsFile("/path")
isSymlink, err := fio.IsSymlink("/path")
```

### Directory

| Function | Description |
|----------|-------------|
| `EnsureDir(path, perm) error` | Create directory and parents (mkdir -p) |
| `ListDir(dir) ([]fs.DirEntry, error)` | List directory contents |
| `WalkFiles(root, fn) error` | Walk directory, call fn for each file |
| `Glob(pattern) ([]string, error)` | Find files matching pattern |

```go
// Create directory
err := fio.EnsureDir("/path/to/dir", 0o755)

// List directory
entries, err := fio.ListDir("/path/to/dir")
for _, entry := range entries {
    fmt.Println(entry.Name(), entry.IsDir())
}

// Walk files (skips directories)
err := fio.WalkFiles("/path/to/dir", func(path string, info fs.FileInfo) error {
    fmt.Printf("%s: %d bytes\n", path, info.Size())
    return nil
})

// Glob
matches, err := fio.Glob("/path/to/*.txt")
```

### Copy & Move

| Function | Description |
|----------|-------------|
| `Copy(dst, src) (int64, error)` | Copy file |
| `CopyDir(dst, src) error` | Copy directory recursively |
| `Move(dst, src) error` | Move file (cross-device supported) |

```go
// Copy file
n, err := fio.Copy("/dst/file", "/src/file")

// Copy directory
err := fio.CopyDir("/dst/dir", "/src/dir")

// Move file (works across devices)
err := fio.Move("/dst/file", "/src/file")
```

### Remove

| Function | Description |
|----------|-------------|
| `Remove(path) error` | Delete file or empty directory |
| `RemoveAll(path) error` | Delete recursively |

```go
// Remove file
err := fio.Remove("/path/to/file")

// Remove directory recursively
err := fio.RemoveAll("/path/to/dir")
```

### Symlink

| Function | Description |
|----------|-------------|
| `Symlink(target, link) error` | Create symbolic link |
| `ReadLink(path) (string, error)` | Read symlink target |

```go
// Create symlink
err := fio.Symlink("/path/to/target", "/path/to/link")

// Read symlink target
target, err := fio.ReadLink("/path/to/link")
```

### Path

| Function | Description |
|----------|-------------|
| `Ext(path) string` | Get extension (e.g., ".txt") |
| `Base(path) string` | Get filename without directory |
| `BaseName(path) string` | Get filename without directory and extension |
| `Dir(path) string` | Get directory portion |
| `Clean(path) (string, error)` | Get cleaned absolute path |

```go
path := "/home/user/docs/report.pdf"

fio.Ext(path)      // ".pdf"
fio.Base(path)     // "report.pdf"
fio.BaseName(path) // "report"
fio.Dir(path)      // "/home/user/docs"

abs, _ := fio.Clean("../file.txt") // "/absolute/path/file.txt"
```

### Misc

| Function | Description |
|----------|-------------|
| `Touch(path) error` | Create empty file or update mod time |

```go
// Create empty file or update modification time
err := fio.Touch("/path/to/file")
```

## Error Handling

```go
// Check for size limit error
data, err := fio.ReadLimit("/path/to/file", 1024)
if errors.Is(err, fio.ErrSizeExceedsLimit) {
    // file too large
}
```

## License

MIT License