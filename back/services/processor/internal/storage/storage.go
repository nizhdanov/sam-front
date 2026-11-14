package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"sam/pkg/apperr"
)

const fileName = "inventory.csv"

type Files struct{ root string }

func New(root string) (*Files, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &Files{root: root}, nil
}

func (f *Files) dir(importID uuid.UUID) string {
	return filepath.Join(f.root, importID.String())
}

func (f *Files) Save(importID uuid.UUID, r io.Reader, maxBytes int64) (string, int64, error) {
	dir := f.dir(importID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", 0, fmt.Errorf("create import dir: %w", err)
	}
	final := filepath.Join(dir, fileName)
	tmp := final + ".tmp"

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return "", 0, fmt.Errorf("create import file: %w", err)
	}
	size, copyErr := io.Copy(out, io.LimitReader(r, maxBytes+1))
	closeErr := out.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = os.RemoveAll(dir)
		return "", 0, fmt.Errorf("write import file: %w", err)
	}
	if size > maxBytes {
		_ = os.RemoveAll(dir)
		return "", 0, apperr.TooLarge(fmt.Sprintf("file exceeds %d bytes", maxBytes))
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.RemoveAll(dir)
		return "", 0, fmt.Errorf("finalize import file: %w", err)
	}
	return final, size, nil
}

func (f *Files) Open(importID uuid.UUID) (io.ReadCloser, error) {
	file, err := os.Open(filepath.Join(f.dir(importID), fileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, apperr.NotFound("import file not found")
	}
	return file, err
}

func (f *Files) Remove(importID uuid.UUID) error {
	return os.RemoveAll(f.dir(importID))
}
