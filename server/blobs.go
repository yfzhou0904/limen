package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// blobs are filesystem-backed under blobsRoot. Keys are slash-separated
// relative paths, e.g. "materials/<id>/content.md".

func blobPath(key string) (string, error) {
	clean := filepath.Clean("/" + key)
	if strings.Contains(clean, "..") {
		return "", errors.New("invalid blob key")
	}
	return filepath.Join(blobsRoot, clean), nil
}

func blobPut(key string, data []byte) error {
	p, err := blobPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func blobGet(key string) ([]byte, error) {
	p, err := blobPath(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func blobDeleteDir(key string) error {
	p, err := blobPath(key)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}
