package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/dto"
)

func TestClassificationAndExitCodes(t *testing.T) {
	want := testTrades(t)
	for _, tc := range []struct {
		name   string
		got    []dto.ParsedTrade
		err    error
		status string
	}{
		{"match", []dto.ParsedTrade{want[1].Trade, want[0].Trade}, nil, "match"},
		{"empty", nil, nil, "empty"},
		{"parse error", nil, errors.New("private parser detail"), "parse_error"},
		{"partial error", []dto.ParsedTrade{want[0].Trade}, errors.New("partial failure"), "parse_error"},
		{"mismatch", []dto.ParsedTrade{want[0].Trade}, nil, "mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classify("edited", want, tc.got, tc.err)
			if c.Primary != tc.status || c.FallbackRequired != (tc.status != "match") {
				t.Fatalf("bad classification: %+v", c)
			}
			if code := resultCode(researchReport{Cases: []caseReport{{Name: "original", Primary: "match"}, c}}); code != 0 {
				t.Fatalf("edited observation forced failure: %d", code)
			}
			c.Name = "original"
			wantCode := 0
			if tc.status != "match" {
				wantCode = 1
			}
			if code := resultCode(researchReport{Cases: []caseReport{c}}); code != wantCode {
				t.Fatalf("original exit code: %d", code)
			}
			c.Name = "gmail_original"
			if code := resultCode(researchReport{Cases: []caseReport{c}}); code != wantCode {
				t.Fatalf("Gmail baseline exit code: %d", code)
			}
		})
	}
	if resultCode(researchReport{OperationalError: "failed"}) != 2 {
		t.Fatal("operational failure must be nonzero")
	}
}

func TestReportDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	report := researchReport{Cases: []caseReport{{Name: "edited", Primary: "empty", FallbackRequired: true}}}
	if err := writeReport(path, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded researchReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.AIExecuted || !decoded.Cases[0].FallbackRequired {
		t.Fatal("misleading report")
	}
	if err := writeReport(path, researchReport{}); err == nil {
		t.Fatal("existing report overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("existing file modified")
	}
}

func TestCLIValidationWithoutNetwork(t *testing.T) {
	for _, args := range [][]string{{"--message-id", "id"}, {"--query", "custom"}, {"--token-file", "other"}, {"unexpected"}, {"--unknown"}, {"--broker-name", "Broker"}, {"--config-file=", "--fixtures-dir", "missing"}, {"--config-file", " "}, {"--case", "invalid"}, {"--case="}, {"--ai-provider", "ollama"}, {"--ai-provider", "openai"}, {"--ollama-url", "http://localhost"}, {"--ollama-model", "test"}, {"--render-python", "python"}, {"--render-script", "script.py"}, {"--ai-context-size", "123"}, {"--ai-max-tokens", "0"}, {"--ai-max-tokens", "-1"}, {"--ai-max-tokens", "invalid"}, {"--ai-timeout", "0s"}} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 2 {
			t.Fatalf("args %v returned %d", args, code)
		}
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"--help"}, &out, &errOut); code != 0 {
		t.Fatal("help failed")
	}
	if !strings.Contains(errOut.String(), "fixtures-dir") {
		t.Fatal("missing flag help")
	}
}

func TestSelectedFixturesWithoutNetwork(t *testing.T) {
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: geminiTestTransport(func(*http.Request) (*http.Response, error) {
		t.Error("metadata-only selection attempted a network request")
		return nil, errors.New("network forbidden")
	})}
	defer func() { http.DefaultClient = previous }()
	for _, provider := range [][]string{nil, {"--ai-provider", "gemini"}} {
		dir, config := syntheticFixtures(t)
		if err := os.Remove(filepath.Join(dir, editedFilename)); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		args := append([]string{"--fixtures-dir", dir, "--config-file", config, "--case", "original", "--ai-fallback"}, provider...)
		if code := run(args, &out, &errOut); code != 0 {
			t.Fatalf("original-only failed: %d %s", code, errOut.String())
		}
		if !strings.Contains(out.String(), "original: primary=match") || strings.Contains(out.String(), "edited:") || !strings.Contains(out.String(), "AI not executed.") {
			t.Fatalf("unexpected selected output: %s", out.String())
		}
	}
	dir, config := syntheticFixtures(t)
	if err := os.Remove(filepath.Join(dir, originalFilename)); err != nil {
		t.Fatal(err)
	}
	report, err := executeWithTemplates(options{fixturesDir: dir, configFile: config, caseSelection: "edited"}, nil)
	if err != nil || len(report.Cases) != 1 || report.Cases[0].Name != "edited" || report.AIExecuted || resultCode(report) != 0 {
		t.Fatalf("edited-only observation failed: %+v %v", report, err)
	}
	_, err = executeWithTemplates(options{fixturesDir: dir, configFile: config, caseSelection: "edited", gmail: true}, nil)
	if err == nil || err.Error() != "cannot read original PDF" {
		t.Fatal("Gmail must require the original SHA baseline before network access")
	}
	_, err = executeWithTemplates(options{caseSelection: "invalid"}, nil)
	if err == nil || !strings.Contains(err.Error(), "--case") {
		t.Fatal("programmatic invalid case accepted")
	}
}

func TestOfflineOperationalFailureReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	var out, errOut bytes.Buffer
	if code := run([]string{"--fixtures-dir", dir, "--report", path}, &out, &errOut); code != 2 {
		t.Fatalf("missing fixtures returned %d", code)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report researchReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.OperationalError == "" || report.AIExecuted || len(report.Cases) != 0 {
		t.Fatal("invalid operational report")
	}
}
