package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrUnsafePath = errors.New("unsafe storage path")

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
	absolute, err := s.AbsolutePath(relativePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absolute); err == nil {
		return os.Remove(absolute)
	}
	return nil
}

func (s LocalStorage) AbsolutePath(relativePath string) (string, error) {
	relativePath = strings.TrimSpace(relativePath)
	relative := filepath.FromSlash(relativePath)
	if !filepath.IsLocal(relative) || filepath.VolumeName(relative) != "" || filepath.Clean(relative) != relative {
		return "", fmt.Errorf("%w: %s", ErrUnsafePath, relativePath)
	}

	base, err := filepath.Abs(s.BaseDir)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(filepath.Join(base, relative))
	if err != nil {
		return "", err
	}
	if err := ensureWithinBase(base, absolute, relativePath); err != nil {
		return "", err
	}

	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		if os.IsNotExist(err) {
			return absolute, nil
		}
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return absolute, nil
		}
		return "", err
	}
	if err := ensureWithinBase(resolvedBase, resolvedPath, relativePath); err != nil {
		return "", err
	}
	return absolute, nil
}

func ensureWithinBase(base, path, original string) error {
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUnsafePath, original)
	}
	if relative == "." {
		return nil
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("%w: %s", ErrUnsafePath, original)
	}
	return nil
}

func randomName() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
