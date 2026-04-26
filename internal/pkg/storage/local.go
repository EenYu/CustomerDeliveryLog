package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SavedFile struct {
	FileName      string
	RelativePath  string
	ThumbnailPath string
	Size          int64
	Ext           string
}

type LocalStorage struct {
	BaseDir string
}

func (s LocalStorage) Save(file multipart.File, header *multipart.FileHeader) (SavedFile, error) {
	now := time.Now()
	dir := filepath.Join(
		s.BaseDir,
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", int(now.Month())),
		fmt.Sprintf("%02d", now.Day()),
	)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SavedFile{}, err
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	fileName := randomName() + ext
	absolute := filepath.Join(dir, fileName)

	dst, err := os.Create(absolute)
	if err != nil {
		return SavedFile{}, err
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		return SavedFile{}, err
	}

	rel, err := filepath.Rel(s.BaseDir, absolute)
	if err != nil {
		return SavedFile{}, err
	}

	return SavedFile{
		FileName:     fileName,
		RelativePath: filepath.ToSlash(rel),
		Size:         size,
		Ext:          strings.TrimPrefix(ext, "."),
	}, nil
}

func (s LocalStorage) Delete(relativePath string) error {
	if relativePath == "" {
		return nil
	}
	absolute := filepath.Join(s.BaseDir, filepath.FromSlash(relativePath))
	if _, err := os.Stat(absolute); err == nil {
		return os.Remove(absolute)
	}
	return nil
}

func (s LocalStorage) AbsolutePath(relativePath string) string {
	return filepath.Join(s.BaseDir, filepath.FromSlash(relativePath))
}

func randomName() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
