package presetstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadCache(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()

	if err := SaveCache(cacheDir, bundle); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	loaded, err := LoadCache(cacheDir)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if loaded.DataVersion != bundle.DataVersion {
		t.Fatalf("DataVersion = %q, want %q", loaded.DataVersion, bundle.DataVersion)
	}
	if len(loaded.Subscription.OriginTypes) != len(bundle.Subscription.OriginTypes) {
		t.Fatalf("OriginTypes 长度 = %d, want %d", len(loaded.Subscription.OriginTypes), len(bundle.Subscription.OriginTypes))
	}
}

func TestLoadCacheRejectsCorruption(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()
	if err := SaveCache(cacheDir, bundle); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	bundlePath := filepath.Join(cacheDir, bundleFileName)
	if err := os.WriteFile(bundlePath, []byte("{broken json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadCache(cacheDir); err == nil {
		t.Fatal("LoadCache() error = nil, want corruption error")
	}
}

func TestLoadCacheRejectsHashMismatch(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()
	if err := SaveCache(cacheDir, bundle); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	shaPath := filepath.Join(cacheDir, shaFileName)
	if err := os.WriteFile(shaPath, []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadCache(cacheDir); err == nil {
		t.Fatal("LoadCache() error = nil, want hash mismatch")
	}
}

func TestSaveCacheWritesCanonicalJSON(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()
	if err := SaveCache(cacheDir, bundle); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cacheDir, bundleFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["dataVersion"] != bundle.DataVersion {
		t.Fatalf("dataVersion = %v, want %q", decoded["dataVersion"], bundle.DataVersion)
	}
}

func TestSaveCacheRollsBackOnSidecarFailure(t *testing.T) {
	cacheDir := t.TempDir()
	original := validBundle()
	original.DataVersion = "2026.07.10-1"
	if err := SaveCache(cacheDir, original); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	next := validBundle()
	next.DataVersion = "2026.07.10-2"

	writer := atomicFileWriter
	defer func() { atomicFileWriter = writer }()
	calls := 0
	atomicFileWriter = func(path string, data []byte, perm os.FileMode) error {
		calls++
		if filepath.Base(path) == shaFileName {
			return fmt.Errorf("injected sidecar failure")
		}
		return writer(path, data, perm)
	}

	if err := SaveCache(cacheDir, next); err == nil {
		t.Fatal("SaveCache() error = nil, want injected sidecar failure")
	}
	loaded, err := LoadCache(cacheDir)
	if err != nil {
		t.Fatalf("LoadCache() after rollback error = %v", err)
	}
	if loaded.DataVersion != original.DataVersion {
		t.Fatalf("rolled back version = %q, want %q", loaded.DataVersion, original.DataVersion)
	}
	if calls < 2 {
		t.Fatalf("atomicFileWriter calls = %d, want at least 2", calls)
	}
}
