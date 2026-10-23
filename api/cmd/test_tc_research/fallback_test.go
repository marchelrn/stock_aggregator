package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/service/parser"
)

const validAI = `{"trades":[{"reference":null,"ticker":"BBCA","side":"BUY","trade_date":"2026-10-19","lot":1,"quantity":100,"price":"100.00","gross_amount":"10000.00"}]}`

func TestFallbackTrigger(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, true}, {parser.ErrEmailNotApplicable, false}, {errors.New("invalid PDF template private config"), false},
		{errors.New("load PDF templates: unavailable"), false}, {errors.New("PDF template store is nil"), false},
		{errors.New("unmatched PDF: no active template matches"), true}, {errors.New(`PDF template "key" version 1: invalid quantity`), true},
		{errors.New("cannot read PDF"), true}, {context.DeadlineExceeded, false},
		{errors.New("ambiguous PDF: multiple active templates match"), false},
	} {
		if got := shouldFallback(nil, tc.err); got != tc.want {
			t.Fatalf("trigger %v: %v", tc.err, got)
		}
	}
	if shouldFallback([]dto.ParsedTrade{{Ticker: "WRONG"}}, nil) {
		t.Fatal("CSV mismatch must not trigger AI")
	}
}

func TestAIValidation(t *testing.T) {
	if _, err := validateAITrades(validAI); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{
		validAI + ` {}`, strings.Replace(validAI, `"lot":1`, `"lot":null`, 1),
		strings.Replace(validAI, `"lot":1,`, "", 1), strings.Replace(validAI, `"quantity":100`, `"quantity":101`, 1),
		strings.Replace(validAI, `"price":"100.00"`, `"price":"100,00"`, 1),
		strings.Replace(validAI, `"gross_amount":"10000.00"`, `"gross_amount":"9999.99"`, 1),
		strings.Replace(validAI, `2026-10-19`, `2026-02-30`, 1), strings.Replace(validAI, `"BUY"`, `"buy"`, 1),
		strings.Replace(validAI, `"lot":1`, `"lot":1.0`, 1), strings.Replace(validAI, `"lot":1`, `"lot":-1`, 1),
		strings.Replace(validAI, `"reference":null`, `"summary":"PRIVATE","reference":null`, 1),
		`{"trades":[]}`, `{"trades":null}`, "```json\n" + validAI + "\n```",
	} {
		if _, err := validateAITrades(content); err == nil || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatalf("accepted invalid or leaked output: %v", err)
		}
	}
	// Reference is not an evaluated transaction field.
	if _, err := validateAITrades(strings.Replace(validAI, `"reference":null,`, "", 1)); err != nil {
		t.Fatal(err)
	}
}

func TestFallbackExitCodes(t *testing.T) {
	for status, code := range map[string]int{"match": 0, "mismatch": 1, "invalid_output": 1, "request_error": 2} {
		report := researchReport{Cases: []caseReport{{Name: "edited", Primary: "parse_error", Fallback: &fallbackReport{Attempted: true, Status: status}}}}
		if resultCode(report) != code {
			t.Fatalf("%s exit code", status)
		}
	}
}
