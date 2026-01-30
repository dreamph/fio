package fio_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dreamph/fio"
)

type sourceFactory struct {
	name    string
	makeFio func() fio.Source
	open    func() (io.ReadCloser, error)
	cleanup func()
}

func newSourceFactory(b *testing.B, kind string, data []byte) sourceFactory {
	b.Helper()

	switch kind {
	case "bytes":
		return sourceFactory{
			name:    "bytes",
			makeFio: func() fio.Source { return fio.BytesSource(data) },
			open:    func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil },
			cleanup: func() {
				// no-op
			},
		}
	case "file":
		dir := b.TempDir()
		path := filepath.Join(dir, "input.bin")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			b.Fatalf("WriteFile: %v", err)
		}
		return sourceFactory{
			name:    "file",
			makeFio: func() fio.Source { return fio.PathSource(path) },
			open:    func() (io.ReadCloser, error) { return os.Open(path) },
			cleanup: func() {
				// temp dir cleanup handled by testing
			},
		}
	case "url":
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			_, _ = io.Copy(w, bytes.NewReader(data))
		}))
		return sourceFactory{
			name:    "url",
			makeFio: func() fio.Source { return fio.URLSource(srv.URL) },
			open: func() (io.ReadCloser, error) {
				resp, err := http.Get(srv.URL)
				if err != nil {
					return nil, err
				}
				return resp.Body, nil
			},
			cleanup: func() {
				srv.Close()
			},
		}
	default:
		b.Fatalf("unknown source kind: %s", kind)
	}
	return sourceFactory{}
}

type nopWriteCloser struct{ io.Writer }

func (n nopWriteCloser) Close() error { return nil }

func benchNormalReadOnly(b *testing.B, size int, src sourceFactory) {
	b.Helper()

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r, err := src.open()
		if err != nil {
			b.Fatalf("open: %v", err)
		}

		_, err = io.Copy(io.Discard, r)
		_ = r.Close()
		if err != nil {
			b.Fatalf("copy: %v", err)
		}
	}
}

func benchFioReadOnly(b *testing.B, size int, src sourceFactory) {
	b.Helper()

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		err := fio.Read(ctx, src.makeFio(), func(r io.Reader) error {
			_, err := io.Copy(io.Discard, r)
			return err
		})
		if err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}

func benchNormalCopy(b *testing.B, size int, storage string, src sourceFactory, opsPerSession int) {
	b.Helper()

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	dir := ""
	if storage == "file" {
		dir = b.TempDir()
	}

	for i := 0; i < b.N; {
		for j := 0; j < opsPerSession && i < b.N; j++ {
			r, err := src.open()
			if err != nil {
				b.Fatalf("open: %v", err)
			}

			var (
				w       io.WriteCloser
				cleanup func()
			)
			switch storage {
			case "memory":
				w = nopWriteCloser{Writer: &bytes.Buffer{}}
			case "file":
				f, err := os.CreateTemp(dir, "normal-*")
				if err != nil {
					_ = r.Close()
					b.Fatalf("CreateTemp: %v", err)
				}
				cleanup = func() { _ = os.Remove(f.Name()) }
				w = f
			default:
				_ = r.Close()
				b.Fatalf("unknown storage: %s", storage)
			}

			_, err = io.Copy(w, r)
			_ = r.Close()
			_ = w.Close()
			if cleanup != nil {
				cleanup()
			}
			if err != nil {
				b.Fatalf("copy: %v", err)
			}
			i++
		}
	}
}

func benchFioDo(b *testing.B, size int, src sourceFactory, mgr fio.IoManager) {
	b.Helper()

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	// Create session once and reuse
	ses, err := mgr.NewSession()
	if err != nil {
		b.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = ses.Cleanup() }()

	ctx := fio.WithSession(context.Background(), ses)

	for i := 0; i < b.N; i++ {
		// Use fio.Copy to leverage fast paths for bytes/file sources
		out, err := fio.Copy(ctx, src.makeFio(), fio.Out(fio.Txt))
		if err != nil {
			b.Fatalf("Copy: %v", err)
		}
		_ = out
	}
}

func BenchmarkCompareFio(b *testing.B) {
	useMmap := envBool("FIO_BENCH_USE_MMAP", false)

	opsPerSessionList := []int{1}
	sizes := []int{
		1 << 10,   // 1KB
		1 << 20,   // 1MB
		10 << 20,  // 10MB
		100 << 20, // 100MB
	}
	sourceKinds := []string{"bytes", "file"}
	storages := []struct {
		name string
		fio  fio.StorageType
	}{
		{name: "memory", fio: fio.Memory},
		{name: "file", fio: fio.File},
	}

	// Create fio managers once per storage type
	fioManagers := make(map[fio.StorageType]fio.IoManager)
	for _, storage := range storages {
		mgr, err := fio.NewIoManager(
			"",
			storage.fio,
			fio.WithMaxPreallocate(0),
			fio.WithSpillThreshold(0),
			fio.WithThreshold(0),
			fio.WithMmap(useMmap),
		)
		if err != nil {
			b.Fatalf("NewIoManager(fio): %v", err)
		}
		fioManagers[storage.fio] = mgr
		defer func(m fio.IoManager) { _ = m.Cleanup() }(mgr)
	}

	for _, size := range sizes {
		data := bytes.Repeat([]byte{'a'}, size)
		sizeLabel := strconv.Itoa(size)

		// Read-only benchmarks (no output)
		for _, sourceKind := range sourceKinds {
			b.Run("normal/"+sourceKind+"/read-only/"+sizeLabel, func(b *testing.B) {
				src := newSourceFactory(b, sourceKind, data)
				defer src.cleanup()
				benchNormalReadOnly(b, size, src)
			})
			b.Run("fio/"+sourceKind+"/read-only/"+sizeLabel, func(b *testing.B) {
				src := newSourceFactory(b, sourceKind, data)
				defer src.cleanup()
				benchFioReadOnly(b, size, src)
			})
		}

		// Copy benchmarks (with output)
		for _, sourceKind := range sourceKinds {
			for _, storage := range storages {
				for _, opsPerSession := range opsPerSessionList {
					label := fmt.Sprintf("ops%d", opsPerSession)
					if sourceKind != "url" {
						b.Run("normal/"+sourceKind+"/storage-"+storage.name+"/"+sizeLabel+"/"+label, func(b *testing.B) {
							src := newSourceFactory(b, sourceKind, data)
							defer src.cleanup()
							benchNormalCopy(b, size, storage.name, src, opsPerSession)
						})
					}
					b.Run("fio/"+sourceKind+"/storage-"+storage.name+"/"+sizeLabel+"/"+label, func(b *testing.B) {
						src := newSourceFactory(b, sourceKind, data)
						defer src.cleanup()
						benchFioDo(b, size, src, fioManagers[storage.fio])
					})
				}
			}
		}
	}
}

func envBool(name string, def bool) bool {
	val := os.Getenv(name)
	if val == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}
