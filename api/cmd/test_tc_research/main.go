package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/service/parser"
)

const originalFilename = "TC_1789262-20261019_1792405283-1.pdf"
const editedFilename = "TC_EDITED.pdf"
const defaultQuery = "has:attachment filename:" + originalFilename

type options struct {
	fixturesDir, reportPath, query, messageID, tokenFile string
	gmail                                                bool
	configFile, brokerName                               string
	aiFallback                                           bool
	caseSelection                                        string
	aiTimeout                                            time.Duration

	aiMaxTokens             int
	aiProvider, geminiModel string
	includeAITrades         bool
}

type caseReport struct {
	Name             string          `json:"name"`
	Primary          string          `json:"primary"`
	Expected         int             `json:"expected"`
	Parsed           int             `json:"parsed"`
	Missing          int             `json:"missing"`
	Unexpected       int             `json:"unexpected"`
	FallbackRequired bool            `json:"fallback_required"`
	Fallback         *fallbackReport `json:"fallback,omitempty"`
}

type researchReport struct {
	Parser              string             `json:"parser"`
	TemplateSource      string             `json:"template_source"`
	Templates           []templateIdentity `json:"templates"`
	AIExecuted          bool               `json:"ai_executed"`
	ComparedFields      []string           `json:"compared_fields"`
	Cases               []caseReport       `json:"cases"`
	GmailSHA256Verified bool               `json:"gmail_sha256_verified"`
	OperationalError    string             `json:"operational_error,omitempty"`
}

func classify(name string, want []expectedTrade, got []dto.ParsedTrade, parseErr error) caseReport {
	c := caseReport{Name: name, Expected: len(want), Parsed: len(got), Primary: "match"}
	switch {
	case parseErr != nil:
		c.Primary = "parse_error"
	case len(got) == 0:
		c.Primary = "empty"
		c.Missing = len(want)
	default:
		c.Missing, c.Unexpected = compareTrades(want, got)
		if c.Missing != 0 || c.Unexpected != 0 {
			c.Primary = "mismatch"
		}
	}
	c.FallbackRequired = c.Primary != "match"
	return c
}

func execute(o options) (researchReport, error) {
	return executeWithTemplates(o, loadDatabaseTemplates)
}

func executeWithTemplates(o options, loadDB databaseTemplateLoader) (researchReport, error) {
	source := "database"
	if o.configFile != "" {
		source = "preview"
	}
	report := researchReport{Parser: "Metadata PDF", TemplateSource: source, Templates: []templateIdentity{}, ComparedFields: []string{"date", "ticker", "side", "lot", "price", "amount"}, Cases: []caseReport{}}
	if o.caseSelection == "" {
		o.caseSelection = "all"
	}
	if o.caseSelection != "all" && o.caseSelection != "original" && o.caseSelection != "edited" {
		return report, fmt.Errorf("--case must be all, original or edited")
	}
	if o.aiProvider != "" && o.aiProvider != "gemini" {
		return report, fmt.Errorf("--ai-provider must be gemini")
	}
	csvFile, err := os.Open(filepath.Join(o.fixturesDir, "real_value.csv"))
	if err != nil {
		return report, fmt.Errorf("cannot open ground-truth CSV")
	}
	want, err := loadGroundTruth(csvFile)
	closeErr := csvFile.Close()
	if err != nil {
		return report, err
	}
	if closeErr != nil {
		return report, fmt.Errorf("cannot close ground-truth CSV")
	}
	var original []byte
	var fixtures []struct {
		name string
		data []byte
	}
	for _, fixture := range []struct{ name, filename string }{{"original", originalFilename}, {"edited", editedFilename}} {
		selected := o.caseSelection == "all" || o.caseSelection == fixture.name
		if !selected && !(o.gmail && fixture.name == "original") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(o.fixturesDir, fixture.filename))
		if err != nil {
			return report, fmt.Errorf("cannot read %s PDF", fixture.name)
		}
		if fixture.name == "original" {
			original = data
		}
		if selected {
			fixtures = append(fixtures, struct {
				name string
				data []byte
			}{fixture.name, data})
		}
	}
	// Keep parser/setup logs private, including any future trade-level logging.
	// The research command emits only template identities and classifications/counts.
	previousLogOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(previousLogOutput)
	ctx, cancel := context.WithTimeout(context.Background(), templateReadTimeout)
	defer cancel()
	templates, err := loadTemplates(ctx, o, loadDB)
	if err != nil {
		return report, err
	}
	for _, template := range templates {
		report.Templates = append(report.Templates, templateIdentity{Key: template.Key, Version: template.Version, Source: source})
	}
	p := parser.NewMetadataParser(templates)
	addCase := func(name string, data []byte, got []dto.ParsedTrade, parseErr error) {
		c := classify(name, want, got, parseErr)
		if o.aiFallback && shouldFallback(got, parseErr) {
			fallback, executed := runGeminiFallback(context.Background(), o, data, want, http.DefaultClient)
			c.Fallback = &fallback
			report.AIExecuted = report.AIExecuted || executed
		}
		report.Cases = append(report.Cases, c)
	}
	for _, fixture := range fixtures {
		got, parseErr := p.Parse(fixture.data)
		addCase(fixture.name, fixture.data, got, parseErr)
	}
	if o.gmail {
		baseline, err := fetchGmailBaseline(o, original)
		if err != nil {
			return report, err
		}
		report.GmailSHA256Verified = true
		got, parseErr := p.ParseEmail(context.Background(), baseline.Data, baseline.Metadata)
		addCase("gmail_original", baseline.Data, got, parseErr)
	}
	return report, nil
}

func resultCode(report researchReport) int {
	if report.OperationalError != "" {
		return 2
	}
	code := 0
	for _, c := range report.Cases {
		if c.Fallback != nil && c.Fallback.Attempted {
			switch c.Fallback.Status {
			case "request_error":
				return 2
			case "match":
				continue
			default:
				code = 1
			}
		} else if c.Name != "edited" && c.Primary != "match" {
			code = 1
		}
	}
	return code
}

func writeReport(path string, report researchReport) error {
	// Never overwrite a fixture, token, or an existing private report.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("cannot create report (destination must not exist)")
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(report)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("cannot finish writing report")
	}
	return nil
}

func run(args []string, out, errOut io.Writer) int {
	var o options
	flags := flag.NewFlagSet("test_tc_research", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&o.fixturesDir, "fixtures-dir", "testing", "directory containing the original PDF, TC_EDITED.pdf, and real_value.csv")
	flags.StringVar(&o.reportPath, "report", "", "optional JSON destination (must not exist; no report saved by default)")
	flags.StringVar(&o.configFile, "config-file", "", "explicit offline preview JSON config; default reads active templates from database")
	flags.StringVar(&o.brokerName, "broker-name", "", "preview broker label (requires --config-file; default: Preview broker)")
	flags.StringVar(&o.caseSelection, "case", "all", "fixture selection: all, original or edited")
	flags.BoolVar(&o.aiFallback, "ai-fallback", false, "opt in to AI on document parse failure (Gemini uploads PDF to Google)")
	flags.StringVar(&o.aiProvider, "ai-provider", "gemini", "fallback provider: gemini only")
	flags.StringVar(&o.geminiModel, "gemini-model", "", "Gemini API model ID; defaults to GEMINI_MODEL from .env/environment")
	flags.BoolVar(&o.includeAITrades, "include-ai-trades", false, "include validated AI transaction objects in the private JSON report")

	flags.IntVar(&o.aiMaxTokens, "ai-max-tokens", 4096, "maximum AI output tokens (includes reasoning on some providers)")
	flags.DurationVar(&o.aiTimeout, "ai-timeout", 180*time.Second, "per-case AI timeout")
	flags.BoolVar(&o.gmail, "gmail", false, "opt in to readonly Gmail baseline verification")
	flags.StringVar(&o.query, "query", defaultQuery, "Gmail query; searches at most 20 messages across at most 4 pages")
	flags.StringVar(&o.messageID, "message-id", "", "explicit Gmail message ID instead of search (requires --gmail)")
	flags.StringVar(&o.tokenFile, "token-file", ".gmail_token.json", "existing OAuth token file; no interactive login")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return 2
	}
	if !o.gmail && (o.messageID != "" || o.query != defaultQuery || o.tokenFile != ".gmail_token.json") {
		fmt.Fprintln(errOut, "--message-id, --query and --token-file require --gmail")
		return 2
	}
	if o.brokerName != "" && (o.configFile == "" || strings.TrimSpace(o.brokerName) == "") {
		fmt.Fprintln(errOut, "--broker-name requires --config-file and a nonempty label")
		return 2
	}
	configExplicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "config-file" {
			configExplicit = true
		}
	})
	if configExplicit && strings.TrimSpace(o.configFile) == "" {
		fmt.Fprintln(errOut, "--config-file requires an explicit nonempty path")
		return 2
	}
	if o.caseSelection != "all" && o.caseSelection != "original" && o.caseSelection != "edited" {
		fmt.Fprintln(errOut, "--case must be all, original or edited")
		return 2
	}
	if o.aiProvider != "gemini" {
		fmt.Fprintln(errOut, "--ai-provider must be gemini")
		return 2
	}
	if o.aiMaxTokens <= 0 || o.aiTimeout <= 0 {
		fmt.Fprintln(errOut, "--ai-max-tokens and --ai-timeout must be positive")
		return 2
	}
	report, err := execute(o)
	if err != nil {
		report.OperationalError = err.Error()
	}
	fmt.Fprintf(out, "parser=%s template_source=%s\n", report.Parser, report.TemplateSource)
	for _, template := range report.Templates {
		fmt.Fprintf(out, "template: key=%q version=%d source=%s\n", template.Key, template.Version, template.Source)
	}
	for _, c := range report.Cases {
		fmt.Fprintf(out, "%s: primary=%s fallback_required=%t expected=%d parsed=%d missing=%d unexpected=%d\n", c.Name, c.Primary, c.FallbackRequired, c.Expected, c.Parsed, c.Missing, c.Unexpected)
	}
	if report.AIExecuted {
		fmt.Fprintln(out, "AI request attempted; raw model reasoning and document contents are not logged.")
	} else {
		fmt.Fprintln(out, "AI not executed.")
	}
	for _, c := range report.Cases {
		if f := c.Fallback; f != nil {
			fmt.Fprintf(out, "%s: ai_attempted=%t ai_status=%s parsed=%d missing=%d unexpected=%d duration_ms=%d\n", c.Name, f.Attempted, f.Status, f.Parsed, f.Missing, f.Unexpected, f.DurationMS)
			if d := f.Diagnostics; d != nil {
				fmt.Fprintf(out, "%s: ai_envelope=%s completion_reason=%s choices=%d content_bytes=%d reasoning_bytes=%d\n", c.Name, d.Envelope, d.CompletionReason, d.ChoiceCount, d.ContentBytes, d.ReasoningBytes)
			}
			if f.Error != "" {
				fmt.Fprintln(errOut, "AI fallback:", f.Error)
			}
		}
	}
	fmt.Fprintln(out, "Compared: date, ticker, side, lot, price, amount (one-cent tolerance for money). REF#, Board, company name and broker are not compared.")
	if o.gmail && report.GmailSHA256Verified {
		fmt.Fprintln(out, "Gmail original: exact filename and SHA256 baseline verified.")
	}
	if report.OperationalError != "" {
		fmt.Fprintln(errOut, "Operational failure:", report.OperationalError)
	}
	if o.reportPath != "" {
		if err := writeReport(o.reportPath, report); err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
	}
	return resultCode(report)
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
