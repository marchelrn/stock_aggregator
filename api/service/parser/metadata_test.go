package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/models"
)

type metadataFakeStore struct {
	templates []models.PDFParsingTemplate
	err       error
	calls     int
	ctx       context.Context
}

func (s *metadataFakeStore) ListActive(ctx context.Context) ([]models.PDFParsingTemplate, error) {
	s.calls++
	s.ctx = ctx
	return s.templates, s.err
}

func metadataTestConfig() MetadataConfig {
	return MetadataConfig{
		SchemaVersion:   1,
		RequiredMarkers: []string{"SYNTHETIC"},
		Date:            MetadataDate{Pattern: `Trade day: ([^\r\n]+)`, Group: 1, Layout: "2006-01-02"},
		Table: MetadataTable{
			StartPattern: `BEGIN\r?\n`, EndPattern: `END`,
			RowPattern: `(?m)^([^|\r\n]+)\|([^|\r\n]+)\|([^|\r\n]+)\|([^|\r\n]+)\|([^|\r\n]+)\|([^|\r\n]+)(?:\r?\n|$)`,
			Fields:     map[string]int{"ticker": 1, "lot": 2, "quantity": 3, "price": 4, "buy_amount": 5, "sell_amount": 6},
		},
		Numbers:      MetadataNumbers{DecimalSeparator: ".", ThousandsSeparator: ",", GroupingSize: 3},
		SharesPerLot: 100,
	}
}

func metadataTestTemplate(t *testing.T, cfg MetadataConfig) models.PDFParsingTemplate {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return models.PDFParsingTemplate{Key: "synthetic", BrokerName: "Test broker", Version: 1, Active: true, Config: data}
}

const metadataValidText = "SYNTHETIC\nTrade day: 2026-10-19\nBEGIN\nAAAA|2|200|1.25|250.00|0.00\nBBBB|3|300|2|0|600\nEND\n"

func metadataParseText(t *testing.T, store *metadataFakeStore, text string, meta *EmailMetadata) (int, error) {
	t.Helper()
	p := NewMetadataParser(store)
	configs, err := p.load(context.Background())
	if err != nil {
		return 0, err
	}
	trades, err := parseMetadataContent(context.Background(), configs, text, meta)
	if err != nil && trades != nil {
		t.Fatal("must not return partial trades on error")
	}
	return len(trades), err
}

// A tiny in-memory PDF keeps public-API tests independent of private fixtures.
func metadataTestPDF(text string) []byte {
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

func TestMetadataPublicParseAndFreshConfig(t *testing.T) {
	cfg := metadataTestConfig()
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{metadataTestTemplate(t, cfg)}}
	p := NewMetadataParser(store)
	data := metadataTestPDF(metadataValidText)
	trades, err := p.Parse(data)
	if err != nil {
		text, _ := extractMetadataPDF(context.Background(), data)
		t.Fatalf("%v; synthetic extraction: %q", err, text)
	}
	if len(trades) != 2 || trades[0].Type != "BUY" || trades[1].Type != "SELL" ||
		trades[0].Lot != 2 || trades[1].Lot != 3 || trades[0].Price != 1.25 ||
		trades[0].AmountDone != 250 || trades[1].AmountDone != 600 ||
		trades[0].Date.Format("2006-01-02") != "2026-10-19" || trades[0].BrokerName != "Test broker" {
		t.Fatal("incorrect BUY/SELL values")
	}
	// Simulates editing JSONB in the DB while retaining the same parser instance.
	cfg.Table.Fields["buy_amount"], cfg.Table.Fields["sell_amount"] = 6, 5
	store.templates[0] = metadataTestTemplate(t, cfg)
	trades, err = p.Parse(data)
	if err != nil || len(trades) != 2 || trades[0].Type != "SELL" || trades[1].Type != "BUY" {
		t.Fatalf("changed DB mapping not applied: %v", err)
	}
	cfg.RequiredMarkers = []string{"DIFFERENT LAYOUT"}
	store.templates[0] = metadataTestTemplate(t, cfg)
	if _, err := p.Parse(data); err == nil || !strings.Contains(err.Error(), "unmatched") {
		t.Fatal("changed DB markers not applied")
	}
	if store.calls != 3 || p.BrokerName() == "" {
		t.Fatal("expected fresh store query on every call and generic broker label")
	}
}

func TestMetadataRejectsIncompleteAndTamperedTrades(t *testing.T) {
	cases := map[string]string{
		"missing date":           strings.ReplaceAll(metadataValidText, "Trade day: 2026-10-19\n", ""),
		"invalid date":           strings.ReplaceAll(metadataValidText, "2026-10-19", "2026-02-30"),
		"duplicate date":         strings.ReplaceAll(metadataValidText, "SYNTHETIC", "SYNTHETIC\nTrade day: 2026-10-19"),
		"missing table start":    strings.ReplaceAll(metadataValidText, "BEGIN", "OTHER"),
		"missing table end":      strings.ReplaceAll(metadataValidText, "END", "OTHER"),
		"duplicate table":        metadataValidText + "BEGIN\nEND\n",
		"empty table":            "SYNTHETIC\nTrade day: 2026-10-19\nBEGIN\nEND",
		"partial first row":      strings.ReplaceAll(metadataValidText, "AAAA|2|200|1.25|250.00|0.00", "AAAA|2|200"),
		"partial last row":       strings.ReplaceAll(metadataValidText, "BBBB|3|300|2|0|600", "BBBB|3|300|2|0"),
		"garbage between rows":   strings.ReplaceAll(metadataValidText, "BBBB", "broken trade\nBBBB"),
		"bad numeric":            strings.ReplaceAll(metadataValidText, "250.00", "oops"),
		"exponent":               strings.ReplaceAll(metadataValidText, "250.00", "2.5e2"),
		"negative":               strings.ReplaceAll(metadataValidText, "250.00", "-250.00"),
		"malformed grouping":     strings.ReplaceAll(metadataValidText, "250.00", "2,50.00"),
		"NaN":                    strings.ReplaceAll(metadataValidText, "250.00", "NaN"),
		"Inf":                    strings.ReplaceAll(metadataValidText, "250.00", "Inf"),
		"quantity mismatch":      strings.ReplaceAll(metadataValidText, "|200|", "|201|"),
		"fractional lot":         strings.ReplaceAll(metadataValidText, "|2|200|", "|2.5|200|"),
		"fractional quantity":    strings.ReplaceAll(metadataValidText, "|200|", "|200.5|"),
		"zero lot":               strings.ReplaceAll(metadataValidText, "|2|200|", "|0|200|"),
		"zero quantity":          strings.ReplaceAll(metadataValidText, "|200|", "|0|"),
		"zero price":             strings.ReplaceAll(metadataValidText, "|1.25|", "|0|"),
		"arithmetic tamper":      strings.ReplaceAll(metadataValidText, "250.00", "250.01"),
		"tiny arithmetic tamper": strings.ReplaceAll(metadataValidText, "250.00", "250.000000000000000001"),
		"both sides zero":        strings.ReplaceAll(metadataValidText, "250.00", "0.00"),
		"both sides positive":    strings.ReplaceAll(metadataValidText, "250.00|0.00", "250.00|250.00"),
		"empty ticker":           strings.ReplaceAll(metadataValidText, "AAAA|", " |"),
		"missing zero cell":      strings.ReplaceAll(metadataValidText, "|250.00|0.00", "|250.00"),
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			store := &metadataFakeStore{templates: []models.PDFParsingTemplate{metadataTestTemplate(t, metadataTestConfig())}}
			if _, err := metadataParseText(t, store, text, nil); err == nil {
				t.Fatal("expected fail-closed error")
			}
		})
	}
}

func TestMetadataSelectionAndEmail(t *testing.T) {
	template := metadataTestTemplate(t, metadataTestConfig())
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{template}}
	if _, err := metadataParseText(t, store, "unknown", nil); err == nil || !strings.Contains(err.Error(), "unmatched") {
		t.Fatal("unknown document must not match")
	}
	second := template
	second.Version = 2
	store.templates = append(store.templates, second)
	if _, err := metadataParseText(t, store, metadataValidText, nil); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal("multiple versions must not choose one implicitly")
	}
	store.templates[1].Active = false
	if count, err := metadataParseText(t, store, metadataValidText, nil); err != nil || count != 2 {
		t.Fatal("inactive templates must not match")
	}
	store.templates[0].EmailSender = "trades@example.com"
	store.templates[0].SubjectContains = "confirmation"
	store.templates[0].FilenameSuffix = ".pdf"
	store.templates[1] = second
	store.templates[1].EmailSender = "other@example.com"
	meta := EmailMetadata{From: "Trade Desk <TRADES@example.com>", Subject: "Daily Confirmation", Filename: "trade.PDF"}
	p := NewMetadataParser(store)
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, true)
	if trades, err := p.ParseEmail(ctx, metadataTestPDF(metadataValidText), meta); err != nil || len(trades) != 2 || store.ctx != ctx {
		t.Fatalf("email selection/context failed: %v", err)
	}
	for _, wrong := range []EmailMetadata{
		{From: "spoof-trades@example.com", Subject: meta.Subject, Filename: meta.Filename},
		{From: meta.From, Subject: "newsletter", Filename: meta.Filename},
		{From: meta.From, Subject: meta.Subject, Filename: "trade.pdf.exe"},
		{},
	} {
		if _, err := p.ParseEmail(context.Background(), metadataTestPDF(metadataValidText), wrong); !errors.Is(err, ErrEmailNotApplicable) {
			t.Fatalf("expected selector-ineligible sentinel, got %v", err)
		}
	}
	// Parse has no email metadata: document markers still apply, never selectors alone.
	if _, err := p.Parse(metadataTestPDF(metadataValidText)); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal("document-only selection should remain ambiguous")
	}
}

func TestMetadataEmailApplicabilityBeforeExtraction(t *testing.T) {
	template := metadataTestTemplate(t, metadataTestConfig())
	template.EmailSender = "trades@example.com"
	template.SubjectContains = "confirmation"
	template.FilenameSuffix = ".pdf"
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{template}}
	p := NewMetadataParser(store)
	eligible := EmailMetadata{From: "trades@example.com", Subject: "Daily confirmation", Filename: "trade.pdf"}
	ineligible := eligible
	ineligible.From = "other@example.com"
	malformed := []byte("not a PDF")
	if trades, err := p.ParseEmail(context.Background(), malformed, ineligible); trades != nil || !errors.Is(err, ErrEmailNotApplicable) {
		t.Fatalf("ineligible malformed PDF must fail before extraction: %v", err)
	}
	if trades, err := p.ParseEmail(context.Background(), malformed, eligible); trades != nil || err == nil || errors.Is(err, ErrEmailNotApplicable) || !strings.Contains(err.Error(), "cannot read PDF") {
		t.Fatalf("eligible malformed PDF must remain an extraction error: %v", err)
	}
	if trades, err := p.ParseEmail(context.Background(), metadataTestPDF("unknown layout"), eligible); trades != nil || err == nil || errors.Is(err, ErrEmailNotApplicable) || !strings.Contains(err.Error(), "unmatched") {
		t.Fatalf("eligible unknown layout must remain unmatched: %v", err)
	}
	// Validate every active config before deciding that an email is ineligible.
	store.templates[0].Config = json.RawMessage(`{}`)
	if _, err := p.ParseEmail(context.Background(), malformed, ineligible); err == nil || errors.Is(err, ErrEmailNotApplicable) || !strings.Contains(err.Error(), "invalid PDF template") {
		t.Fatalf("invalid config must take precedence over selector rejection: %v", err)
	}
	for _, templates := range [][]models.PDFParsingTemplate{nil, {{Active: false}}} {
		store.templates = templates
		if _, err := p.ParseEmail(context.Background(), malformed, ineligible); err == nil || errors.Is(err, ErrEmailNotApplicable) || !strings.Contains(err.Error(), "no active") {
			t.Fatalf("no active templates must remain a configuration error: %v", err)
		}
	}
	store.err = errors.New("store unavailable")
	if _, err := p.ParseEmail(context.Background(), malformed, ineligible); !errors.Is(err, store.err) || errors.Is(err, ErrEmailNotApplicable) {
		t.Fatalf("store error must take precedence over selector rejection: %v", err)
	}
}

func TestMetadataEmailSelectorsFreshConfig(t *testing.T) {
	template := metadataTestTemplate(t, metadataTestConfig())
	template.EmailSender = "trades@example.com"
	template.SubjectContains = "Daily confirmation"
	template.FilenameSuffix = ".PDF"
	wildcard := metadataTestTemplate(t, metadataTestConfig())
	wildcard.Key = "wildcard"
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{template, wildcard, {Active: false}}}
	p := NewMetadataParser(store)
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, true)
	selectors, err := p.EmailSelectors(ctx)
	want := EmailMetadata{From: template.EmailSender, Subject: template.SubjectContains, Filename: template.FilenameSuffix}
	if err != nil || len(selectors) != 2 || selectors[0] != want || selectors[1] != (EmailMetadata{}) || store.ctx != ctx {
		t.Fatalf("active selector projection/context failed: %v", err)
	}
	store.templates[0].EmailSender = "updated@example.com"
	store.templates[0].SubjectContains = "Updated substring"
	store.templates[0].FilenameSuffix = "confirmation.pdf"
	selectors, err = p.EmailSelectors(ctx)
	want = EmailMetadata{From: "updated@example.com", Subject: "Updated substring", Filename: "confirmation.pdf"}
	if err != nil || len(selectors) != 2 || selectors[0] != want || store.calls != 2 {
		t.Fatalf("selector edits must be visible without recreating parser: %v", err)
	}
	store.templates[0].Active = false
	selectors, err = p.EmailSelectors(ctx)
	if err != nil || len(selectors) != 1 || selectors[0] != (EmailMetadata{}) {
		t.Fatalf("activation changes must be visible immediately: %v", err)
	}
	store.templates[1].Config = json.RawMessage(`{}`)
	if selectors, err := p.EmailSelectors(ctx); selectors != nil || err == nil || errors.Is(err, ErrEmailNotApplicable) {
		t.Fatal("fresh invalid configs must fail rather than return cached selectors")
	}
}

func TestMetadataEmailSelectorsErrors(t *testing.T) {
	valid := metadataTestTemplate(t, metadataTestConfig())
	invalid := valid
	invalid.Key, invalid.Config = "invalid", json.RawMessage(`{}`)
	for name, store := range map[string]TemplateStore{
		"nil store":               nil,
		"store error":             &metadataFakeStore{err: errors.New("store unavailable")},
		"no active":               &metadataFakeStore{},
		"inactive only":           &metadataFakeStore{templates: []models.PDFParsingTemplate{{Active: false}}},
		"invalid alongside valid": &metadataFakeStore{templates: []models.PDFParsingTemplate{valid, invalid}},
	} {
		t.Run(name, func(t *testing.T) {
			selectors, err := NewMetadataParser(store).EmailSelectors(context.Background())
			if selectors != nil || err == nil || errors.Is(err, ErrEmailNotApplicable) {
				t.Fatalf("expected config/store error and no partial selectors: %v", err)
			}
		})
	}
	storeErr := errors.New("store unavailable")
	if _, err := NewMetadataParser(&metadataFakeStore{err: storeErr}).EmailSelectors(context.Background()); !errors.Is(err, storeErr) {
		t.Fatal("store error must remain discoverable through errors.Is")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if selectors, err := NewMetadataParser(&metadataFakeStore{templates: []models.PDFParsingTemplate{valid}}).EmailSelectors(ctx); selectors != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("selector loading must respect cancellation: %v", err)
	}
}

func TestMetadataConfigValidation(t *testing.T) {
	mutations := map[string]func(*MetadataConfig){
		"unsupported schema":     func(c *MetadataConfig) { c.SchemaVersion = 2 },
		"no markers":             func(c *MetadataConfig) { c.RequiredMarkers = nil },
		"bad marker":             func(c *MetadataConfig) { c.RequiredMarkers = []string{"["} },
		"empty marker":           func(c *MetadataConfig) { c.RequiredMarkers = []string{".*"} },
		"bad date regex":         func(c *MetadataConfig) { c.Date.Pattern = "[" },
		"bad date group":         func(c *MetadataConfig) { c.Date.Group = 99 },
		"missing date layout":    func(c *MetadataConfig) { c.Date.Layout = "" },
		"incomplete date layout": func(c *MetadataConfig) { c.Date.Layout = "01-02" },
		"literal date layout":    func(c *MetadataConfig) { c.Date.Layout = "not a date" },
		"empty start":            func(c *MetadataConfig) { c.Table.StartPattern = "" },
		"empty end":              func(c *MetadataConfig) { c.Table.EndPattern = "" },
		"bad row":                func(c *MetadataConfig) { c.Table.RowPattern = "[" },
		"empty matching row":     func(c *MetadataConfig) { c.Table.RowPattern = ".*" },
		"missing field":          func(c *MetadataConfig) { delete(c.Table.Fields, "price") },
		"extra field":            func(c *MetadataConfig) { c.Table.Fields["extra"] = 1 },
		"duplicate group":        func(c *MetadataConfig) { c.Table.Fields["price"] = 1 },
		"bad group":              func(c *MetadataConfig) { c.Table.Fields["price"] = 99 },
		"no lot size":            func(c *MetadataConfig) { c.SharesPerLot = 0 },
		"same separators":        func(c *MetadataConfig) { c.Numbers.ThousandsSeparator = "." },
		"digit separator":        func(c *MetadataConfig) { c.Numbers.DecimalSeparator = "1" },
		"long separator":         func(c *MetadataConfig) { c.Numbers.DecimalSeparator = ".." },
		"bad grouping":           func(c *MetadataConfig) { c.Numbers.GroupingSize = 0 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			cfg := metadataTestConfig()
			mutate(&cfg)
			store := &metadataFakeStore{templates: []models.PDFParsingTemplate{metadataTestTemplate(t, cfg)}}
			if _, err := metadataParseText(t, store, metadataValidText, nil); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	for _, raw := range []string{"null", "{}", "{", "{} {}", `{"unknown":true}`} {
		template := metadataTestTemplate(t, metadataTestConfig())
		template.Config = json.RawMessage(raw)
		if _, err := compileMetadata(template); err == nil {
			t.Fatal("invalid JSON config accepted")
		}
	}
	// Fail closed even if another active template matches and this one's selectors do not.
	good := metadataTestTemplate(t, metadataTestConfig())
	bad := good
	bad.Key, bad.EmailSender, bad.Config = "invalid", "other@example.com", json.RawMessage(`{}`)
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{good, bad}}
	if _, err := metadataParseText(t, store, metadataValidText, &EmailMetadata{}); err == nil {
		t.Fatal("invalid active template ignored")
	}
}

func TestMetadataLocaleAndExactArithmetic(t *testing.T) {
	cfg := metadataTestConfig()
	cfg.Numbers.DecimalSeparator, cfg.Numbers.ThousandsSeparator = ",", "."
	cfg.Date.Pattern, cfg.Date.Layout = `Booked ([^\r\n]+)`, "02.01.2006"
	cfg.RequiredMarkers = []string{"NEW DOCUMENT"}
	cfg.SharesPerLot = 10
	text := "NEW DOCUMENT\nBooked 19.10.2026\nBEGIN\nAAAA|100|1.000,00|1,23|1.230,00|0,00\nEND"
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{metadataTestTemplate(t, cfg)}}
	if count, err := metadataParseText(t, store, text, nil); err != nil || count != 1 {
		t.Fatalf("config-driven locale/date/lot size failed: %v", err)
	}
	for _, raw := range []string{"", ".", "1,", "1.00,00", "+1", " 1", "1/2", "1,0,0", "1.0000,00", "1,2e3"} {
		if _, err := cfg.Numbers.parse(raw); err == nil {
			t.Fatal("malformed localized number accepted")
		}
	}
	cfg.Numbers.ThousandsSeparator = ""
	if _, err := cfg.Numbers.parse("1000,00"); err != nil {
		t.Fatal("ungrouped locale without thousands separator should work")
	}
}

func TestMetadataStoreAndPDFErrors(t *testing.T) {
	for _, store := range []*metadataFakeStore{{err: errors.New("store unavailable")}, {}} {
		p := NewMetadataParser(store)
		if _, err := p.Parse(metadataTestPDF(metadataValidText)); err == nil {
			t.Fatal("store failure/no templates must fail")
		}
	}
	if _, err := NewMetadataParser(nil).Parse(nil); err == nil {
		t.Fatal("nil store accepted")
	}
	store := &metadataFakeStore{templates: []models.PDFParsingTemplate{metadataTestTemplate(t, metadataTestConfig())}}
	p := NewMetadataParser(store)
	for _, data := range [][]byte{nil, []byte("not PDF"), metadataTestPDF("")} {
		if _, err := p.Parse(data); err == nil {
			t.Fatal("bad/empty PDF accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.ParseEmail(ctx, nil, EmailMetadata{}); !errors.Is(err, context.Canceled) {
		t.Fatal("context cancellation not propagated")
	}
}

func TestMetadataStockbitFixtures(t *testing.T) {
	config, err := os.ReadFile("../../migrations/templates/stockbit_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	template := models.PDFParsingTemplate{Key: "stockbit", BrokerName: "Stockbit Sekuritas", Version: 1, Active: true, Config: config}
	if _, err := compileMetadata(template); err != nil {
		t.Fatalf("seed config is invalid: %v", err)
	}
	p := NewMetadataParser(&metadataFakeStore{templates: []models.PDFParsingTemplate{template}})
	t.Run("original", func(t *testing.T) {
		paths, err := filepath.Glob("../../testing/TC_*.pdf")
		if err != nil {
			t.Fatal(err)
		}
		var original string
		for _, path := range paths {
			if !strings.Contains(strings.ToUpper(filepath.Base(path)), "EDITED") {
				original = path
				break
			}
		}
		if original == "" {
			t.Skip("optional original PDF fixture is absent from ../../testing")
		}
		data, err := os.ReadFile(original)
		if err != nil {
			t.Fatal("cannot read original fixture")
		}
		trades, err := p.Parse(data)
		if err != nil {
			t.Fatalf("original fixture rejected: %v", err)
		}
		if len(trades) != 2 {
			t.Fatalf("expected two trades, got %d", len(trades))
		}
		if trades[0].Ticker != "ASSA" || trades[0].Type != "BUY" || trades[0].Lot != 44 || trades[0].Price != 460 || trades[0].AmountDone != 2024000 ||
			trades[1].Ticker != "BTPS" || trades[1].Type != "SELL" || trades[1].Lot != 34 || trades[1].Price != 860 || trades[1].AmountDone != 2924000 {
			t.Fatal("original fixture trade values are incorrect")
		}
		for _, trade := range trades {
			if trade.Date.Format("2006-01-02") != "2026-10-19" || trade.BrokerName != template.BrokerName {
				t.Fatal("incorrect fixture date/broker")
			}
		}
	})
	t.Run("edited", func(t *testing.T) {
		data, err := os.ReadFile("../../testing/TC_EDITED.pdf")
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("optional edited PDF fixture is absent from ../../testing")
		}
		if err != nil {
			t.Fatal("cannot read edited fixture")
		}
		trades, err := p.Parse(data)
		if err == nil || trades != nil || !strings.Contains(err.Error(), "unmatched") {
			t.Fatal("edited/unknown layout must fail unmatched without partial trades")
		}
	})
}
