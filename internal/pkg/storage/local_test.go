package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAbsolutePathAllowsStoredRelativePath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "uploads")
	storage := LocalStorage{BaseDir: base}

	path, err := storage.AbsolutePath("2026/04/25/file.txt")
	if err != nil {
		t.Fatalf("AbsolutePath returned error: %v", err)
	}

	want := filepath.Join(base, "2026", "04", "25", "file.txt")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

func TestAbsolutePathRejectsUnsafeRelativePaths(t *testing.T) {
	base := filepath.Join(t.TempDir(), "uploads")
	storage := LocalStorage{BaseDir: base}

	tests := []string{
		"../outside.txt",
		"2026/04/../file.txt",
		filepath.ToSlash(filepath.Join("..", "outside.txt")),
	}
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			if _, err := storage.AbsolutePath(test); !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("expected ErrUnsafePath, got %v", err)
			}
		})
	}
}

func TestAbsolutePathRejectsAbsolutePath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "uploads")
	storage := LocalStorage{BaseDir: base}
	unsafe := filepath.Join(base, "2026", "04", "25", "file.txt")

	if _, err := storage.AbsolutePath(unsafe); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("expected ErrUnsafePath, got %v", err)
	}
}

func TestDeleteRejectsUnsafePathWithoutRemovingOutsideFile(t *testing.T) {
	tempDir := t.TempDir()
	base := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	outside := filepath.Join(tempDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatalf("create outside file: %v", err)
	}

	storage := LocalStorage{BaseDir: base}
	err := storage.Delete("../outside.txt")
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("expected ErrUnsafePath, got %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file should remain: %v", err)
	}
}
