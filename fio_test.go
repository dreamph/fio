package fio

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------- Test Helpers ----------

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "fio-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func tempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------- Read Tests ----------

func TestRead(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello world")

	data, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", data, "hello world")
	}
}

func TestRead_NotFound(t *testing.T) {
	_, err := Read("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadLimit(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello world")

	// Within limit
	data, err := ReadLimit(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", data, "hello world")
	}

	// Exceeds limit
	_, err = ReadLimit(path, 5)
	if !errors.Is(err, ErrSizeExceedsLimit) {
		t.Errorf("got %v, want ErrSizeExceedsLimit", err)
	}

	// Zero limit (read all)
	data, err = ReadLimit(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", data, "hello world")
	}
}

func TestReadAt(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello world")

	data, err := ReadAt(path, 6, 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "world" {
		t.Errorf("got %q, want %q", data, "world")
	}
}

func TestReadAt_EOF(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	// Request more than available
	data, err := ReadAt(path, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

func TestReadString(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello world")

	s, err := ReadString(path)
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello world" {
		t.Errorf("got %q, want %q", s, "hello world")
	}
}

func TestReadLines(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "line1\nline2\nline3")

	var lines []string
	err := ReadLines(path, func(line string) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("got %v", lines)
	}
}

func TestReadLines_StopOnError(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "line1\nline2\nline3")

	stopErr := errors.New("stop")
	count := 0
	err := ReadLines(path, func(line string) error {
		count++
		if count == 2 {
			return stopErr
		}
		return nil
	})
	if !errors.Is(err, stopErr) {
		t.Errorf("got %v, want stopErr", err)
	}
	if count != 2 {
		t.Errorf("got %d, want 2", count)
	}
}

func TestReadJSON(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.json", `{"name":"test","value":42}`)

	var data struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	err := ReadJSON(path, &data)
	if err != nil {
		t.Fatal(err)
	}
	if data.Name != "test" || data.Value != 42 {
		t.Errorf("got %+v", data)
	}
}

func TestReadJSONStream(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.json", `{"name":"test","value":42}`)

	var data struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	err := ReadJSONStream(path, &data)
	if err != nil {
		t.Fatal(err)
	}
	if data.Name != "test" || data.Value != 42 {
		t.Errorf("got %+v", data)
	}
}

func TestReadStream(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello world")

	var buf bytes.Buffer
	err := ReadStream(path, func(r io.Reader) error {
		_, err := io.Copy(&buf, r)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello world" {
		t.Errorf("got %q, want %q", buf.String(), "hello world")
	}
}

// ---------- Write Tests ----------

func TestWrite(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "subdir", "test.txt")

	err := Write(path, []byte("hello"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	data, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

func TestWriteString(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")

	err := WriteString(path, "hello", 0o644)
	if err != nil {
		t.Fatal(err)
	}

	s, err := ReadString(path)
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Errorf("got %q, want %q", s, "hello")
	}
}

func TestWriteJSON(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.json")

	data := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{Name: "test", Value: 42}

	err := WriteJSON(path, data, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ReadString(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `"name": "test"`) {
		t.Errorf("got %q", content)
	}
}

func TestSafeWrite(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")

	// Initial write
	err := SafeWrite(path, []byte("version1"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Overwrite
	err = SafeWrite(path, []byte("version2"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	data, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "version2" {
		t.Errorf("got %q, want %q", data, "version2")
	}

	// Temp file should not exist
	if Exists(path + ".tmp") {
		t.Error("temp file should be removed")
	}
}

func TestAppend(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")

	err := Append(path, []byte("hello"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = Append(path, []byte(" world"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	data, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", data, "hello world")
	}
}

func TestAppendLine(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")

	err := AppendLine(path, "line1", 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = AppendLine(path, "line2\n", 0o644) // Already has newline
	if err != nil {
		t.Fatal(err)
	}

	data, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "line1\nline2\n" {
		t.Errorf("got %q, want %q", data, "line1\nline2\n")
	}
}

// ---------- Temp Tests ----------

func TestCreateTemp(t *testing.T) {
	path, err := CreateTemp("", "fio-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(path)

	if !Exists(path) {
		t.Error("temp file should exist")
	}
	if !strings.Contains(path, "fio-test-") {
		t.Errorf("path %q should contain pattern", path)
	}
}

func TestWriteTemp(t *testing.T) {
	path, err := WriteTemp("", "fio-test-*.txt", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(path)

	data, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

// ---------- Info Tests ----------

func TestExists(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	if !Exists(path) {
		t.Error("file should exist")
	}
	if Exists(filepath.Join(dir, "nonexistent")) {
		t.Error("file should not exist")
	}
	if !Exists(dir) {
		t.Error("directory should exist")
	}
}

func TestExistsWithError(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	exists, err := ExistsWithError(path)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("file should exist")
	}

	exists, err = ExistsWithError(filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("file should not exist")
	}
}

func TestIsDir(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	isDir, err := IsDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !isDir {
		t.Error("should be directory")
	}

	isDir, err = IsDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if isDir {
		t.Error("should not be directory")
	}

	isDir, err = IsDir(filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if isDir {
		t.Error("nonexistent should return false")
	}
}

func TestIsFile(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	isFile, err := IsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !isFile {
		t.Error("should be file")
	}

	isFile, err = IsFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if isFile {
		t.Error("directory should not be file")
	}
}

func TestIsSymlink(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")
	link := filepath.Join(dir, "link")

	err := os.Symlink(path, link)
	if err != nil {
		t.Skip("symlinks not supported")
	}

	isSymlink, err := IsSymlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if !isSymlink {
		t.Error("should be symlink")
	}

	isSymlink, err = IsSymlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if isSymlink {
		t.Error("regular file should not be symlink")
	}
}

func TestSize(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	size, err := Size(path)
	if err != nil {
		t.Fatal(err)
	}
	if size != 5 {
		t.Errorf("got %d, want 5", size)
	}
}

func TestModTime(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	modTime, err := ModTime(path)
	if err != nil {
		t.Fatal(err)
	}

	// Should be recent
	if time.Since(modTime) > time.Minute {
		t.Error("mod time should be recent")
	}
}

func TestFileInfo(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	info, err := FileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("got %q, want %q", info.Name(), "test.txt")
	}
	if info.Size() != 5 {
		t.Errorf("got %d, want 5", info.Size())
	}
}

// ---------- Directory Tests ----------

func TestEnsureDir(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "a", "b", "c")

	err := EnsureDir(path, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	isDir, err := IsDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if !isDir {
		t.Error("should create directory")
	}

	// Empty path should be no-op
	err = EnsureDir("", 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// "." should be no-op
	err = EnsureDir(".", 0o755)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListDir(t *testing.T) {
	dir := tempDir(t)
	tempFile(t, dir, "a.txt", "a")
	tempFile(t, dir, "b.txt", "b")
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	entries, err := ListDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
}

func TestWalkFiles(t *testing.T) {
	dir := tempDir(t)
	tempFile(t, dir, "a.txt", "a")
	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0o755)
	tempFile(t, subdir, "b.txt", "b")

	var files []string
	err := WalkFiles(dir, func(path string, info fs.FileInfo) error {
		files = append(files, info.Name())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}
}

func TestGlob(t *testing.T) {
	dir := tempDir(t)
	tempFile(t, dir, "a.txt", "a")
	tempFile(t, dir, "b.txt", "b")
	tempFile(t, dir, "c.json", "c")

	matches, err := Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2", len(matches))
	}
}

// ---------- Copy & Move Tests ----------

func TestCopy(t *testing.T) {
	dir := tempDir(t)
	src := tempFile(t, dir, "src.txt", "hello world")
	dst := filepath.Join(dir, "subdir", "dst.txt")

	n, err := Copy(dst, src)
	if err != nil {
		t.Fatal(err)
	}
	if n != 11 {
		t.Errorf("got %d bytes, want 11", n)
	}

	data, err := Read(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", data, "hello world")
	}
}

func TestCopyDir(t *testing.T) {
	dir := tempDir(t)

	// Create source structure
	srcDir := filepath.Join(dir, "src")
	os.Mkdir(srcDir, 0o755)
	tempFile(t, srcDir, "a.txt", "a")
	subdir := filepath.Join(srcDir, "sub")
	os.Mkdir(subdir, 0o755)
	tempFile(t, subdir, "b.txt", "b")

	// Copy
	dstDir := filepath.Join(dir, "dst")
	err := CopyDir(dstDir, srcDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify
	if !Exists(filepath.Join(dstDir, "a.txt")) {
		t.Error("a.txt should exist")
	}
	if !Exists(filepath.Join(dstDir, "sub", "b.txt")) {
		t.Error("sub/b.txt should exist")
	}

	data, err := Read(filepath.Join(dstDir, "sub", "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "b" {
		t.Errorf("got %q, want %q", data, "b")
	}
}

func TestMove(t *testing.T) {
	dir := tempDir(t)
	src := tempFile(t, dir, "src.txt", "hello")
	dst := filepath.Join(dir, "dst.txt")

	err := Move(dst, src)
	if err != nil {
		t.Fatal(err)
	}

	if Exists(src) {
		t.Error("source should not exist after move")
	}

	data, err := Read(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

// ---------- Remove Tests ----------

func TestRemove(t *testing.T) {
	dir := tempDir(t)
	path := tempFile(t, dir, "test.txt", "hello")

	err := Remove(path)
	if err != nil {
		t.Fatal(err)
	}

	if Exists(path) {
		t.Error("file should be removed")
	}
}

func TestRemoveAll(t *testing.T) {
	dir := tempDir(t)
	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0o755)
	tempFile(t, subdir, "test.txt", "hello")

	err := RemoveAll(subdir)
	if err != nil {
		t.Fatal(err)
	}

	if Exists(subdir) {
		t.Error("directory should be removed")
	}
}

// ---------- Symlink Tests ----------

func TestSymlink(t *testing.T) {
	dir := tempDir(t)
	target := tempFile(t, dir, "target.txt", "hello")
	link := filepath.Join(dir, "subdir", "link")

	err := Symlink(target, link)
	if err != nil {
		t.Skip("symlinks not supported")
	}

	isSymlink, err := IsSymlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if !isSymlink {
		t.Error("should be symlink")
	}
}

func TestReadLink(t *testing.T) {
	dir := tempDir(t)
	target := tempFile(t, dir, "target.txt", "hello")
	link := filepath.Join(dir, "link")

	err := os.Symlink(target, link)
	if err != nil {
		t.Skip("symlinks not supported")
	}

	got, err := ReadLink(link)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
}

// ---------- Path Tests ----------

func TestExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"file.txt", ".txt"},
		{"file.tar.gz", ".gz"},
		{"file", ""},
		{"/path/to/file.json", ".json"},
	}

	for _, tt := range tests {
		got := Ext(tt.path)
		if got != tt.want {
			t.Errorf("Ext(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestBase(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.txt", "file.txt"},
		{"file.txt", "file.txt"},
		{"/path/to/", "to"},
	}

	for _, tt := range tests {
		got := Base(tt.path)
		if got != tt.want {
			t.Errorf("Base(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestBaseName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.txt", "file"},
		{"file.tar.gz", "file.tar"},
		{"file", "file"},
	}

	for _, tt := range tests {
		got := BaseName(tt.path)
		if got != tt.want {
			t.Errorf("BaseName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.txt", "/path/to"},
		{"file.txt", "."},
	}

	for _, tt := range tests {
		got := Dir(tt.path)
		if got != tt.want {
			t.Errorf("Dir(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestClean(t *testing.T) {
	path, err := Clean("./test/../file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path %q should be absolute", path)
	}
	if !strings.HasSuffix(path, "file.txt") {
		t.Errorf("path %q should end with file.txt", path)
	}
}

// ---------- Misc Tests ----------

func TestTouch(t *testing.T) {
	dir := tempDir(t)

	// Create new file
	path := filepath.Join(dir, "subdir", "touch.txt")
	err := Touch(path)
	if err != nil {
		t.Fatal(err)
	}
	if !Exists(path) {
		t.Error("file should exist")
	}

	// Update existing file
	oldTime, _ := ModTime(path)
	time.Sleep(10 * time.Millisecond)

	err = Touch(path)
	if err != nil {
		t.Fatal(err)
	}

	newTime, _ := ModTime(path)
	if !newTime.After(oldTime) {
		t.Error("mod time should be updated")
	}
}

// ---------- Benchmark ----------

func BenchmarkRead(b *testing.B) {
	dir, _ := os.MkdirTemp("", "fio-bench-*")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "bench.txt")
	data := bytes.Repeat([]byte("x"), 1024)
	os.WriteFile(path, data, 0o644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Read(path)
	}
}

func BenchmarkWrite(b *testing.B) {
	dir, _ := os.MkdirTemp("", "fio-bench-*")
	defer os.RemoveAll(dir)

	data := bytes.Repeat([]byte("x"), 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, "bench.txt")
		Write(path, data, 0o644)
	}
}

func BenchmarkSafeWrite(b *testing.B) {
	dir, _ := os.MkdirTemp("", "fio-bench-*")
	defer os.RemoveAll(dir)

	data := bytes.Repeat([]byte("x"), 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, "bench.txt")
		SafeWrite(path, data, 0o644)
	}
}

func BenchmarkCopy(b *testing.B) {
	dir, _ := os.MkdirTemp("", "fio-bench-*")
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "src.txt")
	data := bytes.Repeat([]byte("x"), 1024*1024) // 1MB
	os.WriteFile(src, data, 0o644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := filepath.Join(dir, "dst.txt")
		Copy(dst, src)
	}
}
