package parser

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPDFTextWithFallback tries ledongthuc/pdf first.
// If the result is suspiciously short (≤ 5 chars), it falls back to
// pdftotext (Poppler), which handles newer Stockbit PDF encodings correctly.
func extractPDFTextWithFallback(data []byte) (string, error) {
	content, err := extractPDFTextLedong(data)
	if err == nil && len(strings.TrimSpace(content)) > 5 {
		return content, nil
	}
	// Fallback: use pdftotext (Poppler)
	return extractPDFTextPoppler(data)
}

// extractPDFTextLedong extracts text using the ledongthuc/pdf library.
func extractPDFTextLedong(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("ledongthuc: %w", err)
	}
	var sb strings.Builder
	for i := 1; i <= pdfReader.NumPage(); i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("Warning: ledongthuc halaman %d: %v", i, err)
			continue
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// extractPDFTextPoppler extracts text by writing the PDF to a temp file
// and calling `pdftotext` (from poppler-utils).
func extractPDFTextPoppler(data []byte) (string, error) {
	// Write PDF bytes to a temp file
	tmp, err := os.CreateTemp("", "stockbit-*.pdf")
	if err != nil {
		return "", fmt.Errorf("poppler: create temp: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("poppler: write temp: %w", err)
	}
	tmp.Close()

	// pdftotext -layout <input> - (output to stdout)
	out, err := exec.Command("pdftotext", "-layout", tmp.Name(), "-").Output()
	if err != nil {
		return "", fmt.Errorf("poppler: pdftotext: %w", err)
	}
	return string(out), nil
}
