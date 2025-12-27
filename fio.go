package fio

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------- Errors ----------

// ErrSizeExceedsLimit is returned when file size exceeds the specified limit.
var ErrSizeExceedsLimit = errors.New("fio: file size exceeds limit")

// ---------- Read ----------

// Read reads the entire file into memory.
func Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadLimit reads up to limit bytes from file.
// If limit <= 0, reads entire file.
// Returns ErrSizeExceedsLimit if file exceeds limit.
func ReadLimit(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if limit <= 0 {
		return io.ReadAll(f)
	}

	lr := io.LimitedReader{R: f, N: limit + 1}
	data, err := io.ReadAll(&lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrSizeExceedsLimit
	}
	return data, nil
}

// ReadAt reads length bytes starting at offset.
// Returns actual bytes read (may be less than length at EOF).
func ReadAt(path string, offset, length int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

// ReadString reads entire file as string.
func ReadString(path string) (string, error) {
	b, err := Read(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadLines reads file line by line, calling fn for each line.
// Stops and returns error if fn returns error.
func ReadLines(path string, fn func(line string) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := fn(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// ReadJSON reads JSON file into v (loads entire file into memory first).
func ReadJSON(path string, v any) error {
	data, err := Read(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ReadJSONStream reads JSON file into v using streaming decoder.
// More memory efficient for large files.
func ReadJSONStream(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(v)
}

// ReadStream opens file and calls fn with reader.
// File is automatically closed after fn returns.
func ReadStream(path string, fn func(r io.Reader) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return fn(f)
}

// ---------- Write ----------

// Write writes data to file (creates parent dir if needed).
func Write(path string, data []byte, perm fs.FileMode) error {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// WriteString writes string to file.
func WriteString(path, s string, perm fs.FileMode) error {
	return Write(path, []byte(s), perm)
}

// WriteJSON writes v as indented JSON to file.
func WriteJSON(path string, v any, perm fs.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return Write(path, data, perm)
}

// SafeWrite atomically writes data via temp file + fsync + rename.
// Ensures file is either fully written or unchanged on failure.
func SafeWrite(path string, data []byte, perm fs.FileMode) error {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}

// Append appends data to file (creates file if not exists).
func Append(path string, data []byte, perm fs.FileMode) error {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// AppendLine appends line with trailing newline.
func AppendLine(path, line string, perm fs.FileMode) error {
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	return Append(path, []byte(line), perm)
}

// ---------- Temp ----------

// CreateTemp creates empty temp file and returns its path.
// Caller is responsible for removing the file.
func CreateTemp(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	return name, nil
}

// WriteTemp writes data to new temp file and returns its path.
// Caller is responsible for removing the file.
func WriteTemp(dir, pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}

	name := f.Name()
	return name, f.Close()
}

// ---------- Info ----------

// Exists reports whether path exists (file or directory).
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ExistsWithError returns (exists, error).
// Not-exist returns (false, nil), other errors return (false, err).
func ExistsWithError(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsDir reports whether path is a directory.
// Returns false if path does not exist.
func IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// IsFile reports whether path is a regular file.
// Returns false if path does not exist.
func IsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// IsSymlink reports whether path is a symbolic link.
// Returns false if path does not exist.
func IsSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}

// Size returns file size in bytes.
func Size(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ModTime returns file modification time.
func ModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// FileInfo returns os.FileInfo for path.
func FileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// ---------- Directory ----------

// EnsureDir creates directory and parents if needed (mkdir -p).
// No-op if path is empty or ".".
func EnsureDir(path string, perm fs.FileMode) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, perm)
}

// ListDir returns directory entries (files and subdirectories).
func ListDir(dir string) ([]fs.DirEntry, error) {
	return os.ReadDir(dir)
}

// WalkFiles walks directory recursively, calling fn for each file (not directory).
// Stops and returns error if fn returns error.
func WalkFiles(root string, fn func(path string, info fs.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return fn(path, info)
	})
}

// Glob returns paths matching shell pattern.
func Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// ---------- Copy & Move ----------

// Copy copies file from src to dst (creates parent dir for dst).
// Preserves file mode. Returns number of bytes copied.
func Copy(dst, src string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return 0, err
	}
	if err := EnsureDir(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = out.Close()
	}()

	n, err := io.Copy(out, in)
	return n, err
}

// CopyDir recursively copies directory from src to dst.
// Preserves file modes.
func CopyDir(dst, src string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return EnsureDir(target, info.Mode())
		}

		_, err = Copy(target, path)
		return err
	})
}

// Move moves/renames file from src to dst.
// Falls back to copy+remove if rename fails (cross-device move).
func Move(dst, src string) error {
	if err := EnsureDir(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// cross-device: copy + remove
	if _, err := Copy(dst, src); err != nil {
		_ = os.Remove(dst) // clean up partial write
		return err
	}
	return os.Remove(src)
}

// ---------- Remove ----------

// Remove deletes a single file or empty directory.
func Remove(path string) error {
	return os.Remove(path)
}

// RemoveAll recursively deletes path and all contents.
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// ---------- Symlink ----------

// Symlink creates symbolic link pointing to target.
func Symlink(target, link string) error {
	if err := EnsureDir(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, link)
}

// ReadLink returns the destination of symbolic link.
func ReadLink(path string) (string, error) {
	return os.Readlink(path)
}

// ---------- Path ----------

// Ext returns file extension including dot (e.g., ".txt").
func Ext(path string) string {
	return filepath.Ext(path)
}

// Base returns filename without directory.
func Base(path string) string {
	return filepath.Base(path)
}

// BaseName returns filename without directory and extension.
func BaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// Dir returns directory portion of path.
func Dir(path string) string {
	return filepath.Dir(path)
}

// Clean returns cleaned absolute path.
func Clean(path string) (string, error) {
	return filepath.Abs(filepath.Clean(path))
}

// ---------- Misc ----------

// Touch creates empty file or updates modification time if exists.
func Touch(path string) error {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return err
	}
	f.Close()

	now := time.Now()
	return os.Chtimes(path, now, now)
}
