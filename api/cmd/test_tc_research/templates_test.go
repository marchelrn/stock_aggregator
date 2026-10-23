package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
	"google.golang.org/api/gmail/v1"
)

const syntheticConfig = `{
 "schema_version":1,"required_markers":["SYNTHETIC"],
 "date":{"pattern":"Trade day: ([^\\r\\n]+)","group":1,"layout":"2006-01-02"},
 "table":{"start_pattern":"BEGIN\\r?\\n","end_pattern":"END",
 "row_pattern":"(?m)^([^|\\r\\n]+)\\|([^|\\r\\n]+)\\|([^|\\r\\n]+)\\|([^|\\r\\n]+)\\|([^|\\r\\n]+)\\|([^|\\r\\n]+)(?:\\r?\\n|$)",
 "fields":{"ticker":1,"lot":2,"quantity":3,"price":4,"buy_amount":5,"sell_amount":6}},
 "numbers":{"decimal_separator":".","thousands_separator":",","grouping_size":3},"shares_per_lot":100
}`

const syntheticText = "SYNTHETIC\nTrade day: 2026-10-19\nBEGIN\nASSA|44|4400|460.00|2024000.00|0.00\nBTPS|34|3400|860.00|000|2924000.00\nEND\n"

func researchTestPDF(text string) []byte {
	var stream strings.Builder
	stream.WriteString("BT /F1 10 Tf 14 TL 40 780 Td\n")
	for _, line := range strings.Split(text, "\n") {
		line = strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(line)
		fmt.Fprintf(&stream, "(%s) Tj T*\n", line)
	}
	stream.WriteString("ET\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", stream.Len(), stream.String()),
	}
	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return []byte(out.String())
}

func syntheticFixtures(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"real_value.csv": []byte(testCSV), originalFilename: researchTestPDF(syntheticText),
		editedFilename: researchTestPDF("UNKNOWN FORMAT"), "config.json": []byte(syntheticConfig),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, filepath.Join(dir, "config.json")
}

func TestExplicitPreviewCLIWithoutDatabase(t *testing.T) {
	dir, config := syntheticFixtures(t)
	t.Setenv("DB_URL", "invalid-private-dsn")
	t.Setenv("ENV", "production")
	path := filepath.Join(dir, "report.json")
	var out, errOut bytes.Buffer
	if code := run([]string{"--fixtures-dir", dir, "--config-file", config, "--broker-name", "Custom broker", "--report", path}, &out, &errOut); code != 0 {
		t.Fatalf("preview failed: code=%d %s", code, errOut.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report researchReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	assertMetadataCases(t, report, "preview")
	if !strings.Contains(out.String(), "template_source=preview") || strings.Contains(string(data), "row_pattern") || strings.Contains(string(data), "invalid-private-dsn") {
		t.Fatal("source not explicit or private configuration leaked")
	}
}

func assertMetadataCases(t *testing.T, report researchReport, source string) {
	t.Helper()
	if report.TemplateSource != source || report.Parser != "Metadata PDF" || len(report.Templates) != 1 || report.Templates[0].Source != source || report.Templates[0].Version < 1 {
		t.Fatal("wrong metadata identity/source")
	}
	if len(report.Cases) != 2 || report.Cases[0].Primary != "match" || report.Cases[0].Parsed != 2 || report.Cases[0].FallbackRequired || report.Cases[1].Primary != "parse_error" || !report.Cases[1].FallbackRequired || report.AIExecuted {
		t.Fatal("original including SELL must match; unknown edited layout must fail closed")
	}
}

func TestTemplateSourcesAndNoImplicitFallback(t *testing.T) {
	dir, config := syntheticFixtures(t)
	calls := 0
	loadDB := func(ctx context.Context) ([]models.PDFParsingTemplate, error) {
		calls++
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > templateReadTimeout {
			t.Fatal("missing DB read deadline")
		}
		return []models.PDFParsingTemplate{{Key: "database-key", Version: 7, BrokerName: "DB broker", Active: true, Config: json.RawMessage(syntheticConfig)}}, nil
	}
	report, err := executeWithTemplates(options{fixturesDir: dir}, loadDB)
	if err != nil {
		t.Fatal(err)
	}
	assertMetadataCases(t, report, "database")
	if calls != 1 || report.Templates[0].Key != "database-key" || report.Templates[0].Version != 7 {
		t.Fatal("not a single identified database snapshot")
	}
	_, err = executeWithTemplates(options{fixturesDir: dir, configFile: config}, loadDB)
	if err != nil || calls != 1 {
		t.Fatal("preview accessed database")
	}
	// Even a seed-looking default file cannot become an implicit preview/fallback.
	seedDir := filepath.Join(dir, "migrations", "templates")
	if err := os.MkdirAll(seedDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "stockbit_v1.json"), []byte(syntheticConfig), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for _, templates := range [][]models.PDFParsingTemplate{nil, {{Active: false}}} {
		failed, err := executeWithTemplates(options{fixturesDir: dir}, func(context.Context) ([]models.PDFParsingTemplate, error) { return templates, nil })
		if err == nil || len(failed.Cases) != 0 || failed.TemplateSource != "database" {
			t.Fatal("empty DB silently fell back")
		}
	}
	failed, err := executeWithTemplates(options{fixturesDir: dir}, func(context.Context) ([]models.PDFParsingTemplate, error) {
		return nil, errors.New("postgres://private:password@host/db")
	})
	if err == nil || strings.Contains(err.Error(), "password") || len(failed.Cases) != 0 {
		t.Fatal("DB failure not sanitized or silently fell back")
	}
	for _, bad := range []string{filepath.Join(dir, "absent.json"), config} {
		if bad == config {
			if err := os.WriteFile(config, []byte(`{"invalid":`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := executeWithTemplates(options{fixturesDir: dir, configFile: bad}, loadDB); err == nil || calls != 1 {
			t.Fatal("bad preview fell back to DB")
		}
	}
}

func TestPreviewBrokerAndNoLegacyParserFallback(t *testing.T) {
	dir, config := syntheticFixtures(t)
	for _, broker := range []string{"", "Custom label"} {
		store, err := loadTemplates(context.Background(), options{configFile: config, brokerName: broker}, func(context.Context) ([]models.PDFParsingTemplate, error) {
			t.Fatal("preview accessed DB")
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		want := broker
		if want == "" {
			want = "Preview broker"
		}
		got, err := parser.NewMetadataParser(store).Parse(researchTestPDF(syntheticText))
		if err != nil || len(got) != 2 || got[1].Type != "SELL" || got[0].BrokerName != want {
			t.Fatal("preview broker/SELL not driven by metadata")
		}
	}
	// A valid JSON object with an invalid metadata contract must not invoke legacy regex.
	if err := os.WriteFile(config, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := execute(options{fixturesDir: dir, configFile: config})
	if err != nil || len(report.Cases) != 2 || report.Cases[0].Primary != "parse_error" {
		t.Fatal("invalid metadata did not fail closed")
	}
}

func TestPrivateLoggingSuppressedAndRestored(t *testing.T) {
	dir, _ := syntheticFixtures(t)
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	_, err := executeWithTemplates(options{fixturesDir: dir}, func(context.Context) ([]models.PDFParsingTemplate, error) {
		log.Print("private template/connection details")
		return nil, errors.New("private DSN")
	})
	if err == nil || output.Len() != 0 {
		t.Fatal("private logging not suppressed")
	}
	log.Print("restored")
	if !strings.Contains(output.String(), "restored") {
		t.Fatal("logging was not restored after failure")
	}
}

func TestFetchedMetadataSelectsTemplate(t *testing.T) {
	pdf := researchTestPDF(syntheticText)
	root := &gmail.MessagePart{
		Headers: []*gmail.MessagePartHeader{{Name: "From", Value: "Research <sender@example.test>"}, {Name: "Subject", Value: "Actual confirmation subject"}},
		Parts:   []*gmail.MessagePart{{Filename: originalFilename, Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString(pdf)}}},
	}
	baseline, found, err := matchingPDF(context.Background(), root, "id", pdf, nil)
	if err != nil || !found {
		t.Fatal("synthetic Gmail baseline failed")
	}
	store := templateSnapshot{{Key: "email-selected", Version: 1, Active: true, BrokerName: "Test", Config: json.RawMessage(syntheticConfig), EmailSender: "sender@example.test", SubjectContains: "confirmation", FilenameSuffix: ".pdf"}}
	p := parser.NewMetadataParser(store)
	if got, err := p.ParseEmail(context.Background(), baseline.Data, baseline.Metadata); err != nil || len(got) != 2 {
		t.Fatal("fetched headers did not select template")
	}
	for _, field := range []string{"sender", "subject", "filename"} {
		meta := baseline.Metadata
		switch field {
		case "sender":
			meta.From = "other@example.test"
		case "subject":
			meta.Subject = "wrong"
		case "filename":
			meta.Filename = "wrong.txt"
		}
		if _, err := p.ParseEmail(context.Background(), baseline.Data, meta); err == nil {
			t.Fatalf("wrong %s did not fail selection", field)
		}
	}
}
