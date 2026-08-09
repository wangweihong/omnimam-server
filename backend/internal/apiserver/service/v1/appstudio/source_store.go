package appstudio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SourceContentStore 保存 Source Service 私有正文；AppStudio 数据库只保存索引与 digest。
type SourceContentStore interface {
	WriteRevision(context.Context, string, int64, map[string][]byte) error
	ReadFile(context.Context, string, int64, string) ([]byte, error)
}

// LocalSourceContentStore 使用不可变目录实现单机持久化 Source Provider。
type LocalSourceContentStore struct{ baseDir string }

func NewLocalSourceContentStore(baseDir string) (*LocalSourceContentStore, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, fmt.Errorf("appstudio source directory is required")
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &LocalSourceContentStore{baseDir: abs}, nil
}

func (s *LocalSourceContentStore) WriteRevision(ctx context.Context, workspaceID string, revision int64, files map[string][]byte) error {
	if err := validateStoreID(workspaceID); err != nil {
		return err
	}
	target := filepath.Join(s.baseDir, workspaceID, strconv.FormatInt(revision, 10))
	if _, err := os.Stat(target); err == nil {
		return revisionContentMatches(ctx, target, files)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Join(s.baseDir), ".revision-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for path, content := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		clean, err := cleanSourcePath(path)
		if err != nil {
			return err
		}
		name := filepath.Join(temporary, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(name, content, 0o640); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		if _, statErr := os.Stat(target); statErr == nil {
			return revisionContentMatches(ctx, target, files)
		}
		return err
	}
	return nil
}

func revisionContentMatches(ctx context.Context, target string, expected map[string][]byte) error {
	actual := make(map[string][]byte, len(expected))
	err := filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("source revision contains an unsupported entry")
		}
		relative, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		actual[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("source revision content conflicts with an existing immutable revision")
	}
	for path, content := range expected {
		stored, ok := actual[path]
		if !ok || !bytes.Equal(stored, content) {
			return fmt.Errorf("source revision content conflicts with an existing immutable revision")
		}
	}
	return nil
}

func (s *LocalSourceContentStore) ReadFile(ctx context.Context, workspaceID string, revision int64, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateStoreID(workspaceID); err != nil {
		return nil, err
	}
	clean, err := cleanSourcePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(s.baseDir, workspaceID, strconv.FormatInt(revision, 10), filepath.FromSlash(clean)))
}

func validateStoreID(value string) error {
	if value == "" || strings.ContainsAny(value, `/\\`) || value == "." || value == ".." {
		return fmt.Errorf("invalid source store id")
	}
	return nil
}

func cleanSourcePath(value string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || strings.ContainsRune(clean, '\x00') {
		return "", fmt.Errorf("invalid source path")
	}
	return clean, nil
}

var _ SourceContentStore = (*LocalSourceContentStore)(nil)
