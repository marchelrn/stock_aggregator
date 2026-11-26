package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ajaibCaptureKey struct{}

type ajaibCapture struct {
	dir     string
	expires time.Time
}

func withAjaibDebugCapture(ctx context.Context, userID uint, broker string) context.Context {
	if os.Getenv("ENV") != "testing" || !strings.EqualFold(strings.TrimSpace(broker), ajaibBroker) {
		return ctx
	}
	allowed, err := strconv.ParseUint(os.Getenv("AJAIB_DEBUG_USER_ID"), 10, 64)
	if err != nil || allowed == 0 || uint64(userID) != allowed {
		return ctx
	}
	dir := os.Getenv("AJAIB_DEBUG_DIR")
	expires, err := time.Parse(time.RFC3339, os.Getenv("AJAIB_DEBUG_UNTIL"))
	if err != nil || !time.Now().Before(expires) || !filepath.IsAbs(dir) {
		return ctx
	}
	return context.WithValue(ctx, ajaibCaptureKey{}, ajaibCapture{dir: dir, expires: expires})
}

func captureFailedAjaibPDF(ctx context.Context, broker string, reviewID uint, pdf []byte) {
	capture, ok := ctx.Value(ajaibCaptureKey{}).(ajaibCapture)
	if !ok || !strings.EqualFold(broker, ajaibBroker) || !time.Now().Before(capture.expires) || len(pdf) == 0 || len(pdf) > maxReviewBytes {
		return
	}
	digest := sha256.Sum256(pdf)
	saved, err := saveAjaibDebugPDF(capture.dir, hex.EncodeToString(digest[:])+".pdf", pdf)
	if err != nil {
		log.Printf("Ajaib debug review=%d: salinan PDF gagal disimpan secara lokal.", reviewID)
	} else if saved {
		log.Printf("Ajaib debug review=%d: salinan dokumen yang gagal diproses sudah disimpan lokal (sha256=%x).", reviewID, digest)
	}
}

func saveAjaibDebugPDF(dir, name string, pdf []byte) (bool, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return false, errors.New("debug directory must be private")
	}
	file, err := os.CreateTemp(dir, ".capture-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(pdf)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return false, err
	}
	if err := os.Link(file.Name(), filepath.Join(dir, name)); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
