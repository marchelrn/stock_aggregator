package service

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAjaibDebugCaptureScope(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV", "testing")
	t.Setenv("AJAIB_DEBUG_DIR", dir)
	t.Setenv("AJAIB_DEBUG_USER_ID", "42")
	t.Setenv("AJAIB_DEBUG_UNTIL", time.Now().Add(time.Hour).Format(time.RFC3339))
	pdf := []byte("%PDF-1.4 synthetic")
	ctx := t.Context()

	captureFailedAjaibPDF(withAjaibDebugCapture(ctx, 99, ajaibBroker), ajaibBroker, 1, pdf)
	captureFailedAjaibPDF(withAjaibDebugCapture(ctx, 42, ""), ajaibBroker, 1, pdf)
	captureFailedAjaibPDF(withAjaibDebugCapture(ctx, 42, ajaibBroker), "Stockbit", 1, pdf)
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected capture outside scope: %v %v", files, err)
	}

	scoped := withAjaibDebugCapture(ctx, 42, ajaibBroker)
	captureFailedAjaibPDF(scoped, ajaibBroker, 1, pdf)
	captureFailedAjaibPDF(scoped, ajaibBroker, 2, pdf)
	name := filepath.Join(dir, fmt.Sprintf("%x.pdf", sha256.Sum256(pdf)))
	data, err := os.ReadFile(name)
	if err != nil || string(data) != string(pdf) {
		t.Fatalf("capture differs from source: %v", err)
	}
	info, err := os.Stat(name)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("capture is not private: %v", err)
	}
	files, err = os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("duplicate capture: %v %v", files, err)
	}

	t.Setenv("AJAIB_DEBUG_UNTIL", time.Now().Add(-time.Minute).Format(time.RFC3339))
	captureFailedAjaibPDF(withAjaibDebugCapture(ctx, 42, ajaibBroker), ajaibBroker, 3, []byte("%PDF-1.4 another fixture"))
	files, err = os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("unexpected capture after expiry: %v %v", files, err)
	}
}
