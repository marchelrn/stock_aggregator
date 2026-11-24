package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
)

// ErrEmailNotApplicable means active templates exist, but none of their email
// selectors match. It does not indicate a PDF extraction or layout failure.
var ErrEmailNotApplicable = errors.New("email does not match any active PDF template selectors")

type TemplateStore interface {
	ListActive(context.Context) ([]models.PDFParsingTemplate, error)
}

type EmailMetadata struct {
	From       string
	Subject    string
	Filename   string
	FromDomain string
}

const (
	DocumentPDF       = "pdf"
	DocumentEmailBody = "email_body"

	KindTrade    = "trade"
	KindCash     = "cash"
	KindDividend = "dividend"

	ParseModeMetadata = "metadata"
	ParseModeLegacy   = "legacy"
)

// LegacyParserNames lists the built-in extractors a version 2 legacy template
// may reference. Routing lives in the database; extraction stays in code until
// the broker's metadata template reaches parity.
var LegacyParserNames = map[string]bool{
	"stockbit": true,
	"trimegah": true,
	"mandiri":  true,
	"eipo":     true,
}

// MetadataConfig is the JSON contract for templates. Version 1 keeps its
// original meaning: a trade table in a PDF parsed by the metadata engine.
// Version 2 adds an explicit document/kind/parse_mode so the database can also
// route legacy extractors and describe cash or dividend emails.
type MetadataConfig struct {
	Schema                 string          `json:"$schema,omitempty"`
	SchemaVersion          int             `json:"schema_version"`
	Document               string          `json:"document,omitempty"`
	Kind                   string          `json:"kind,omitempty"`
	ParseMode              string          `json:"parse_mode,omitempty"`
	LegacyParser           string          `json:"legacy_parser,omitempty"`
	DiscoveryOnly          bool            `json:"discovery_only,omitempty"`
	RequiredMarkers        []string        `json:"required_markers,omitempty"`
	Date                   MetadataDate    `json:"date"`
	Table                  MetadataTable   `json:"table"`
	Numbers                MetadataNumbers `json:"numbers"`
	SharesPerLot           int             `json:"shares_per_lot"`
	Email                  *MetadataEmail  `json:"email,omitempty"`
	RefTemplate            string          `json:"ref_template,omitempty"`
	SenderDomain           string          `json:"sender_domain,omitempty"`
	SubjectPattern         string          `json:"subject_pattern,omitempty"`
	Trade                  *MetadataTrade  `json:"trade,omitempty"`
	FilenamePattern        string          `json:"filename_pattern,omitempty"`
	ExcludeFilenamePattern string          `json:"exclude_filename_pattern,omitempty"`
}

// MetadataEmail describes how to read a cash or dividend email body. Only
// templates with document "email_body" use this section.
type MetadataEmail struct {
	Type             string            `json:"type"` // deposit, withdraw or dividend
	AmountPattern    string            `json:"amount_pattern"`
	AmountGroup      int               `json:"amount_group"`
	DatePattern      string            `json:"date_pattern,omitempty"`
	DateGroup        int               `json:"date_group,omitempty"`
	DateLayout       string            `json:"date_layout,omitempty"`
	RefPattern       string            `json:"ref_pattern,omitempty"`
	RefGroup         int               `json:"ref_group,omitempty"`
	TickerPattern    string            `json:"ticker_pattern,omitempty"`
	TickerGroup      int               `json:"ticker_group,omitempty"`
	Description      string            `json:"description,omitempty"`
	TickerSource     string            `json:"ticker_source,omitempty"`
	DateReplacements map[string]string `json:"date_replacements,omitempty"`
}

type MetadataDate struct {
	Pattern string `json:"pattern"`
	Group   int    `json:"group"`
	Layout  string `json:"layout"`
}

type MetadataTable struct {
	StartPattern string         `json:"start_pattern"`
	EndPattern   string         `json:"end_pattern"`
	RowPattern   string         `json:"row_pattern"`
	Fields       map[string]int `json:"fields"`
}

type MetadataNumbers struct {
	DecimalSeparator   string `json:"decimal_separator"`
	ThousandsSeparator string `json:"thousands_separator"`
	GroupingSize       int    `json:"grouping_size"`
}

type MetadataParser struct{ store TemplateStore }

func NewMetadataParser(store TemplateStore) *MetadataParser { return &MetadataParser{store: store} }
func (*MetadataParser) BrokerName() string                  { return "Metadata PDF" }

func (p *MetadataParser) Parse(data []byte) ([]dto.ParsedTrade, error) {
	return p.parse(context.Background(), data, nil)
}

func (p *MetadataParser) ParseEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	return p.parse(ctx, data, &meta)
}

// EmailSelectors loads and validates the current active templates and projects
// their selectors without requiring a PDF. Empty values are wildcards: operators
// should configure selectors before discovery across a broad mailbox. Subject
// values are substrings for local matching, not Gmail search expressions.
func (p *MetadataParser) EmailSelectors(ctx context.Context) ([]EmailMetadata, error) {
	return p.emailSelectors(ctx, nil)
}

// EmailSelectorsForKinds projects only templates whose kind is listed, so the
// trade query never picks up cash selectors and vice versa. An empty kinds list
// returns every active template, matching EmailSelectors.
func (p *MetadataParser) EmailSelectorsForKinds(ctx context.Context, kinds ...string) ([]EmailMetadata, error) {
	if len(kinds) == 0 {
		return p.emailSelectors(ctx, nil)
	}
	allowed := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		allowed[kind] = true
	}
	return p.emailSelectors(ctx, allowed)
}

func (p *MetadataParser) emailSelectors(ctx context.Context, kinds map[string]bool) ([]EmailMetadata, error) {
	templates, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	selectors := make([]EmailMetadata, 0, len(templates))
	seen := make(map[EmailMetadata]bool, len(templates))
	for _, template := range templates {
		if kinds != nil && !kinds[template.kind] {
			continue
		}
		selector := EmailMetadata{
			From:       template.template.EmailSender,
			Subject:    template.template.SubjectContains,
			Filename:   template.template.FilenameSuffix,
			FromDomain: template.config.SenderDomain,
		}
		if seen[selector] {
			continue
		}
		seen[selector] = true
		selectors = append(selectors, selector)
	}
	return selectors, nil
}

type compiledMetadata struct {
	template      models.PDFParsingTemplate
	config        MetadataConfig
	document      string
	kind          string
	parseMode     string
	legacyParser  string
	discoveryOnly bool

	markers []*regexp.Regexp
	date    *regexp.Regexp
	start   *regexp.Regexp
	end     *regexp.Regexp
	row     *regexp.Regexp

	emailAmount     *regexp.Regexp
	emailDate       *regexp.Regexp
	emailRef        *regexp.Regexp
	emailTicker     *regexp.Regexp
	emailType       string
	subject         *regexp.Regexp
	filename        *regexp.Regexp
	excludeFilename *regexp.Regexp
	tradeFees       []*regexp.Regexp
	tradeRefs       map[string]*regexp.Regexp
}

func (c *compiledMetadata) hasSelector() bool {
	t := c.template
	return t.EmailSender != "" || t.SubjectContains != "" || t.FilenameSuffix != "" || c.config.SenderDomain != "" || c.config.SubjectPattern != "" || c.config.FilenamePattern != ""
}

// parseableTrade reports whether the metadata engine is allowed to extract a
// trade table from a PDF for this template. Legacy and discovery-only rows are
// handled by the dispatcher instead.
func (c *compiledMetadata) parseableTrade() bool {
	return c.kind == KindTrade &&
		c.document == DocumentPDF &&
		c.parseMode == ParseModeMetadata &&
		!c.discoveryOnly
}

type brokerScopeKey struct{}

func WithBrokerScope(ctx context.Context, names map[string]int) context.Context {
	return context.WithValue(ctx, brokerScopeKey{}, names)
}

func BrokerScope(ctx context.Context) map[string]int {
	names, _ := ctx.Value(brokerScopeKey{}).(map[string]int)
	return names
}

func (p *MetadataParser) load(ctx context.Context) ([]compiledMetadata, error) {
	if p == nil || p.store == nil {
		return nil, errors.New("PDF template store is nil")
	}
	templates, err := p.store.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("load PDF templates: %w", err)
	}
	var result []compiledMetadata
	seen := make(map[string]map[int]bool)
	for _, template := range templates {
		if !template.Active || (BrokerScope(ctx) != nil && BrokerScope(ctx)[strings.ToLower(strings.TrimSpace(template.BrokerName))] == 0) {
			continue
		}
		c, err := compileMetadata(template)
		if err != nil {
			return nil, fmt.Errorf("invalid PDF template %q version %d: %w", template.Key, template.Version, err)
		}
		if seen[template.Key] == nil {
			seen[template.Key] = make(map[int]bool)
		}
		if seen[template.Key][template.Version] {
			return nil, errors.New("duplicate active PDF template key/version")
		}
		seen[template.Key][template.Version] = true
		result = append(result, c)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("no active PDF parsing templates configured")
	}
	return result, nil
}

func compileMetadata(t models.PDFParsingTemplate) (compiledMetadata, error) {
	c := compiledMetadata{template: t}
	decoder := json.NewDecoder(bytes.NewReader(t.Config))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c.config); err != nil {
		return c, fmt.Errorf("decode config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c, errors.New("config must contain exactly one JSON object")
	}
	cfg := c.config
	if strings.TrimSpace(t.Key) == "" || strings.TrimSpace(t.BrokerName) == "" || t.Version < 1 {
		return c, errors.New("key, broker name and positive template version required")
	}

	switch cfg.SchemaVersion {
	case 1:
		// Version 1 predates routing metadata: it always describes a trade
		// table parsed by the metadata engine. Explicit overrides are rejected
		// below instead of being silently ignored.
		if cfg.Document == "" {
			cfg.Document = DocumentPDF
		}
		if cfg.Kind == "" {
			cfg.Kind = KindTrade
		}
		if cfg.ParseMode == "" {
			cfg.ParseMode = ParseModeMetadata
		}
	case 2:
		if cfg.Document == "" {
			cfg.Document = DocumentPDF
		}
		if cfg.Kind == "" {
			cfg.Kind = KindTrade
		}
		if cfg.ParseMode == "" {
			cfg.ParseMode = ParseModeMetadata
		}
	default:
		return c, errors.New("schema_version must be 1 or 2")
	}

	c.config = cfg
	c.document = cfg.Document
	c.kind = cfg.Kind
	c.parseMode = cfg.ParseMode
	c.discoveryOnly = cfg.DiscoveryOnly
	c.legacyParser = strings.ToLower(strings.TrimSpace(cfg.LegacyParser))
	if cfg.SenderDomain != "" && !ValidSenderDomain(cfg.SenderDomain) {
		return c, errors.New("invalid sender_domain")
	}
	if cfg.SubjectPattern != "" {
		var err error
		c.subject, err = compilePattern(cfg.SubjectPattern)
		if err != nil {
			return c, fmt.Errorf("subject_pattern: %w", err)
		}
	}
	for _, field := range []struct {
		pattern string
		target  **regexp.Regexp
	}{
		{cfg.FilenamePattern, &c.filename}, {cfg.ExcludeFilenamePattern, &c.excludeFilename},
	} {
		if field.pattern != "" {
			var err error
			*field.target, err = compilePattern(field.pattern)
			if err != nil {
				return c, fmt.Errorf("filename pattern: %w", err)
			}
		}
	}

	if c.document != DocumentPDF && c.document != DocumentEmailBody {
		return c, errors.New("document must be pdf or email_body")
	}
	if c.kind != KindTrade && c.kind != KindCash && c.kind != KindDividend {
		return c, errors.New("kind must be trade, cash or dividend")
	}
	if c.parseMode != ParseModeMetadata && c.parseMode != ParseModeLegacy {
		return c, errors.New("parse_mode must be metadata or legacy")
	}
	if c.parseMode == ParseModeLegacy && !LegacyParserNames[c.legacyParser] {
		return c, errors.New("legacy_parser must name a built-in extractor")
	}
	if cfg.SchemaVersion == 1 && (c.document != DocumentPDF || c.kind != KindTrade || c.parseMode != ParseModeMetadata) {
		return c, errors.New("version 1 config cannot override document, kind or parse_mode")
	}
	if cfg.SchemaVersion < 2 && c.discoveryOnly {
		return c, errors.New("discovery_only requires schema_version 2")
	}
	if c.hasSelector() {
		if t.EmailSender != "" {
			address, err := mail.ParseAddress(t.EmailSender)
			if err != nil || !strings.EqualFold(address.Address, t.EmailSender) {
				return c, errors.New("email_sender must be a bare mailbox address")
			}
		}
	} else if cfg.SchemaVersion >= 2 && !c.discoveryOnly {
		// Version 2 rows take part in routing, so they need at least one hook.
		return c, errors.New("at least one email selector required")
	}

	if c.discoveryOnly {
		return c, nil
	}
	if c.kind != KindTrade && c.document != DocumentEmailBody {
		return c, errors.New("cash and dividend templates must use document email_body")
	}
	if c.kind == KindTrade && c.document != DocumentPDF {
		return c, errors.New("trade templates must use document pdf")
	}
	if c.parseMode == ParseModeLegacy {
		if c.kind != KindTrade {
			return c, errors.New("legacy parsing is only available for trade PDFs")
		}
		// Extraction stays in the built-in parser; only routing and discovery
		// come from this template.
		return c, nil
	}
	if c.kind != KindTrade {
		return c.compileEmail()
	}
	return c.compileTrade()
}

func (c *compiledMetadata) compileTrade() (compiledMetadata, error) {
	cfg := c.config
	if len(cfg.RequiredMarkers) == 0 || cfg.SharesPerLot <= 0 {
		return *c, errors.New("markers and positive shares_per_lot required")
	}
	var err error
	for _, marker := range cfg.RequiredMarkers {
		r, err := compilePattern(marker)
		if err != nil {
			return *c, fmt.Errorf("required marker: %w", err)
		}
		c.markers = append(c.markers, r)
	}
	patterns := []struct {
		name    string
		pattern string
		target  **regexp.Regexp
	}{
		{"date", cfg.Date.Pattern, &c.date},
		{"table start", cfg.Table.StartPattern, &c.start},
		{"table end", cfg.Table.EndPattern, &c.end},
		{"table row", cfg.Table.RowPattern, &c.row},
	}
	for _, pattern := range patterns {
		*pattern.target, err = compilePattern(pattern.pattern)
		if err != nil {
			return *c, fmt.Errorf("%s: %w", pattern.name, err)
		}
	}
	if cfg.Date.Group < 1 || cfg.Date.Group > c.date.NumSubexp() || cfg.Date.Layout == "" {
		return *c, errors.New("invalid date capture group or layout")
	}
	if err := validateDateLayout(cfg.Date.Layout); err != nil {
		return *c, err
	}
	if cfg.Trade != nil {
		return c.compileGenericTrade()
	}
	fields := []string{"ticker", "lot", "quantity", "price", "buy_amount", "sell_amount"}
	if len(cfg.Table.Fields) != len(fields) {
		return *c, errors.New("exactly six trade field mappings required")
	}
	used := make(map[int]bool)
	for _, field := range fields {
		group := cfg.Table.Fields[field]
		if group < 1 || group > c.row.NumSubexp() || used[group] {
			return *c, fmt.Errorf("invalid or duplicate capture group for %s", field)
		}
		used[group] = true
	}
	if err := cfg.Numbers.validate(); err != nil {
		return *c, err
	}
	return *c, nil
}

func (c *compiledMetadata) compileEmail() (compiledMetadata, error) {
	cfg := c.config
	if cfg.Email == nil {
		return *c, errors.New("email section required for cash and dividend templates")
	}
	email := cfg.Email
	for _, marker := range cfg.RequiredMarkers {
		pattern, err := compilePattern(marker)
		if err != nil {
			return *c, fmt.Errorf("required marker: %w", err)
		}
		c.markers = append(c.markers, pattern)
	}
	if email.TickerSource != "" && email.TickerSource != "subject" && email.TickerSource != "body" {
		return *c, errors.New("ticker_source must be subject or body")
	}
	for key, value := range email.DateReplacements {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return *c, errors.New("date replacements must be nonempty")
		}
	}
	switch email.Type {
	case "deposit", "withdraw", "dividend":
	default:
		return *c, errors.New("email type must be deposit, withdraw or dividend")
	}
	if c.kind == KindDividend && email.Type != "dividend" {
		return *c, errors.New("dividend templates require email type dividend")
	}
	if c.kind == KindCash && email.Type == "dividend" {
		return *c, errors.New("cash templates require email type deposit or withdraw")
	}
	var err error
	c.emailAmount, err = compilePattern(email.AmountPattern)
	if err != nil {
		return *c, fmt.Errorf("email amount_pattern: %w", err)
	}
	if email.AmountGroup < 1 || email.AmountGroup > c.emailAmount.NumSubexp() {
		return *c, errors.New("invalid email amount_group")
	}
	if email.DatePattern != "" {
		c.emailDate, err = compilePattern(email.DatePattern)
		if err != nil {
			return *c, fmt.Errorf("email date_pattern: %w", err)
		}
		if email.DateGroup < 1 || email.DateGroup > c.emailDate.NumSubexp() {
			return *c, errors.New("invalid email date_group")
		}
		if err := validateDateLayout(email.DateLayout); err != nil {
			return *c, err
		}
	} else if email.DateGroup != 0 || email.DateLayout != "" {
		return *c, errors.New("email date_group and date_layout require date_pattern")
	}
	if email.RefPattern != "" {
		c.emailRef, err = compilePattern(email.RefPattern)
		if err != nil {
			return *c, fmt.Errorf("email ref_pattern: %w", err)
		}
		if email.RefGroup < 1 || email.RefGroup > c.emailRef.NumSubexp() {
			return *c, errors.New("invalid email ref_group")
		}
	} else if email.RefGroup != 0 {
		return *c, errors.New("email ref_group requires ref_pattern")
	}
	if email.TickerPattern != "" {
		c.emailTicker, err = compilePattern(email.TickerPattern)
		if err != nil {
			return *c, fmt.Errorf("email ticker_pattern: %w", err)
		}
		if email.TickerGroup < 1 || email.TickerGroup > c.emailTicker.NumSubexp() {
			return *c, errors.New("invalid email ticker_group")
		}
	} else if email.TickerGroup != 0 {
		return *c, errors.New("email ticker_group requires ticker_pattern")
	}
	if c.config.RefTemplate != "" {
		if err := validateRefTemplate(c.config.RefTemplate); err != nil {
			return *c, err
		}
		if strings.Contains(c.config.RefTemplate, "{ticker}") && c.emailTicker == nil {
			return *c, errors.New("ref_template ticker requires ticker_pattern")
		}
		if strings.Contains(c.config.RefTemplate, "{ref}") && c.emailRef == nil {
			return *c, errors.New("ref_template ref requires ref_pattern")
		}
	}
	c.emailType = email.Type
	if err := cfg.Numbers.validate(); err != nil {
		return *c, err
	}
	return *c, nil
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, errors.New("empty pattern")
	}
	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.New("invalid regular expression")
	}
	if r.MatchString("") {
		return nil, errors.New("pattern must not match empty text")
	}
	return r, nil
}

// validateDateLayout requires a layout that encodes a full calendar date, so a
// typo cannot silently default year, month or day.
func validateDateLayout(layout string) error {
	if layout == "" {
		return errors.New("invalid date layout")
	}
	a := time.Date(2001, 2, 3, 0, 0, 0, 0, time.UTC)
	b := time.Date(2004, 11, 23, 0, 0, 0, 0, time.UTC)
	for _, value := range []time.Time{a, b} {
		parsed, e := time.Parse(layout, value.Format(layout))
		if e != nil || parsed.Year() != value.Year() || parsed.Month() != value.Month() || parsed.Day() != value.Day() {
			return errors.New("date layout must encode year, month and day")
		}
	}
	return nil
}

func validateRefTemplate(template string) error {
	if len(template) > 120 {
		return errors.New("ref_template is too long")
	}
	remainder := strings.NewReplacer("{ticker}", "", "{paydate}", "", "{ref}", "").Replace(template)
	if strings.ContainsAny(remainder, "{}") {
		return errors.New("unknown or malformed ref_template placeholder")
	}
	return nil
}

func (p *MetadataParser) parse(ctx context.Context, data []byte, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	all, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	templates := make([]compiledMetadata, 0, len(all))
	for _, template := range all {
		if template.parseableTrade() {
			templates = append(templates, template)
		}
	}
	if len(templates) == 0 {
		if meta != nil {
			return nil, ErrEmailNotApplicable
		}
		return nil, errors.New("no active template matches")
	}
	if meta != nil {
		var eligible []compiledMetadata
		for _, template := range templates {
			if template.matchesEmail(*meta) {
				eligible = append(eligible, template)
			}
		}
		if len(eligible) == 0 {
			return nil, ErrEmailNotApplicable
		}
		templates = eligible
	}
	return parseTradePDF(ctx, data, templates, meta)
}

func parseTradePDF(ctx context.Context, data []byte, templates []compiledMetadata, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	type extractKey struct{ mode, passwordBroker string }
	contents := make(map[extractKey]string)
	var trades []dto.ParsedTrade
	var parseErr error
	var diagnostics []string
	var selected *compiledMetadata
	for i := range templates {
		c := &templates[i]
		modes := []string{"plain"}
		passwordBroker := ""
		if t := c.config.Trade; t != nil {
			if t.Extraction == "plain_then_layout" {
				modes = []string{"plain", "layout"}
			} else {
				modes = []string{t.Extraction}
			}
			if t.PasswordSource != "" {
				passwordBroker = c.template.BrokerName
			}
		}
		for n, mode := range modes {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key := extractKey{mode, passwordBroker}
			content, ok := contents[key]
			if !ok {
				var err error
				content, err = c.extractPDF(ctx, data, mode)
				if err != nil {
					if n+1 < len(modes) {
						continue
					}
					return nil, err
				}
				contents[key] = content
			}
			if missing := c.firstMissingMarker(content); missing != 0 {
				if len(templates) == 1 {
					diagnostics = append(diagnostics, fmt.Sprintf("%s %d byte, penanda %d/%d belum terbaca", mode, len(content), missing, len(c.markers)))
				}
				continue
			}
			if selected != nil && selected != c {
				return nil, errors.New("ambiguous PDF: multiple active templates match")
			}
			selected = c
			trades, parseErr = parseMetadataContent(ctx, templates[i:i+1], content, meta)
			if parseErr == nil {
				break
			}
		}
	}
	if selected == nil {
		if len(diagnostics) > 0 {
			return nil, fmt.Errorf("unmatched PDF: no active template matches (%s)", strings.Join(diagnostics, ", "))
		}
		return nil, errors.New("unmatched PDF: no active template matches")
	}
	return trades, parseErr
}

func (c *compiledMetadata) firstMissingMarker(content string) int {
	for i, marker := range c.markers {
		if !marker.MatchString(content) {
			return i + 1
		}
	}
	return 0
}

func extractMetadataPDF(ctx context.Context, data []byte) (content string, err error) {
	// Malformed PDFs can panic inside the extractor. Do not return partial data
	// or include extractor errors that could contain document text.
	defer func() {
		if recover() != nil {
			content, err = "", errors.New("PDF extraction failed")
		}
	}()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Template-based PDFs store text inside a Form XObject that ledongthuc/pdf
	// cannot reach; extractPDFTextWithFallback retries with pdftotext first
	// when the library yields almost nothing.
	if text, ferr := extractPDFTextWithFallback(data); ferr == nil && len(strings.TrimSpace(text)) > 5 {
		return text, nil
	}
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("cannot read PDF")
	}
	var b strings.Builder
	for page := 1; page <= r.NumPage(); page++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		p := r.Page(page)
		if p.V.IsNull() {
			return "", fmt.Errorf("PDF page %d is missing", page)
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("cannot extract PDF page %d", page)
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	if strings.TrimSpace(b.String()) == "" {
		return "", errors.New("PDF contains no extractable text")
	}
	return b.String(), nil
}

func parseMetadataContent(ctx context.Context, templates []compiledMetadata, content string, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	var selected *compiledMetadata
	for i := range templates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c := &templates[i]
		if (meta != nil && !c.matchesEmail(*meta)) || c.firstMissingMarker(content) != 0 {
			continue
		}
		if selected != nil {
			return nil, errors.New("ambiguous PDF: multiple active templates match")
		}
		selected = c
	}
	if selected == nil {
		return nil, errors.New("unmatched PDF: no active template matches")
	}
	var trades []dto.ParsedTrade
	var err error
	if selected.config.Trade != nil {
		trades, err = selected.parseGenericTrades(content, meta)
	} else {
		trades, err = selected.parseTrades(content)
	}
	if err != nil {
		return nil, fmt.Errorf("PDF template %q version %d: %w", selected.template.Key, selected.template.Version, err)
	}
	return trades, nil
}

func (c *compiledMetadata) matchesEmail(meta EmailMetadata) bool {
	return c.matchesEmailSelectors(meta) && !c.excludesFilename(meta.Filename)
}

func (c *compiledMetadata) excludesFilename(name string) bool {
	return c.excludeFilename != nil && c.excludeFilename.MatchString(name)
}

func (c *compiledMetadata) matchesEmailSelectors(meta EmailMetadata) bool {
	t := c.template
	if c.filename != nil && !c.filename.MatchString(meta.Filename) {
		return false
	}
	if c.config.SenderDomain != "" {
		from, err := mail.ParseAddress(meta.From)
		if err != nil {
			return false
		}
		at := strings.LastIndex(from.Address, "@")
		if at < 0 || !strings.EqualFold(from.Address[at+1:], c.config.SenderDomain) {
			return false
		}
	}
	if c.subject != nil && !c.subject.MatchString(meta.Subject) {
		return false
	}
	if t.EmailSender != "" {
		from, err := mail.ParseAddress(meta.From)
		if err != nil || !strings.EqualFold(from.Address, t.EmailSender) {
			return false
		}
	}
	return strings.Contains(strings.ToLower(meta.Subject), strings.ToLower(t.SubjectContains)) &&
		strings.HasSuffix(strings.ToLower(meta.Filename), strings.ToLower(t.FilenameSuffix))
}

func ValidSenderDomain(domain string) bool {
	if len(domain) > 253 || !strings.Contains(domain, ".") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

func (c *compiledMetadata) parseTrades(content string) ([]dto.ParsedTrade, error) {
	dates := c.date.FindAllStringSubmatch(content, -1)
	if len(dates) != 1 {
		return nil, errors.New("exactly one trade date required")
	}
	rawDate := strings.TrimSpace(dates[0][c.config.Date.Group])
	date, err := time.Parse(c.config.Date.Layout, rawDate)
	if err != nil || date.IsZero() || date.Format(c.config.Date.Layout) != rawDate {
		return nil, errors.New("invalid trade date")
	}
	starts := c.start.FindAllStringIndex(content, -1)
	if len(starts) != 1 {
		return nil, errors.New("exactly one table start required")
	}
	remainder := content[starts[0][1]:]
	ends := c.end.FindAllStringIndex(remainder, -1)
	if len(ends) != 1 {
		return nil, errors.New("exactly one table end required")
	}
	table := remainder[:ends[0][0]]
	rows := c.row.FindAllStringSubmatchIndex(table, -1)
	if len(rows) == 0 {
		return nil, errors.New("empty or incomplete trade table")
	}
	var trades []dto.ParsedTrade
	position := 0
	for i, row := range rows {
		if strings.TrimSpace(table[position:row[0]]) != "" {
			return nil, fmt.Errorf("incomplete or unrecognized trade before row %d", i+1)
		}
		fields := make(map[string]string)
		for name, group := range c.config.Table.Fields {
			start, end := row[2*group], row[2*group+1]
			if start < 0 || end <= start {
				return nil, fmt.Errorf("row %d: missing %s", i+1, name)
			}
			fields[name] = strings.TrimSpace(table[start:end])
		}
		trade, err := c.trade(fields, date)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		trades = append(trades, trade)
		position = row[1]
	}
	if strings.TrimSpace(table[position:]) != "" {
		return nil, errors.New("incomplete or unrecognized trailing trade")
	}
	return trades, nil
}

func (c *compiledMetadata) trade(fields map[string]string, date time.Time) (dto.ParsedTrade, error) {
	var result dto.ParsedTrade
	if fields["ticker"] == "" {
		return result, errors.New("empty ticker")
	}
	numbers := make(map[string]*big.Rat)
	for _, field := range []string{"lot", "quantity", "price", "buy_amount", "sell_amount"} {
		n, err := c.config.Numbers.parse(fields[field])
		if err != nil {
			return result, fmt.Errorf("invalid %s", field)
		}
		numbers[field] = n
	}
	lot, quantity, price := numbers["lot"], numbers["quantity"], numbers["price"]
	buy, sell := numbers["buy_amount"], numbers["sell_amount"]
	if !lot.IsInt() || !quantity.IsInt() || lot.Sign() <= 0 || quantity.Sign() <= 0 || price.Sign() <= 0 {
		return result, errors.New("lot/quantity must be positive integers and price must be positive")
	}
	if !lot.Num().IsInt64() || !lot.Num().IsUint64() || lot.Num().Uint64() > uint64(^uint(0)>>1) {
		return result, errors.New("lot exceeds integer range")
	}
	expectedQuantity := new(big.Rat).Mul(lot, new(big.Rat).SetInt64(int64(c.config.SharesPerLot)))
	if quantity.Cmp(expectedQuantity) != 0 {
		return result, errors.New("quantity does not equal lot times shares_per_lot")
	}
	var side string
	var amount *big.Rat
	switch {
	case buy.Sign() > 0 && sell.Sign() == 0:
		side, amount = "BUY", buy
	case sell.Sign() > 0 && buy.Sign() == 0:
		side, amount = "SELL", sell
	default:
		return result, errors.New("exactly one buy/sell amount must be positive; the other must be zero")
	}
	if new(big.Rat).Mul(quantity, price).Cmp(amount) != 0 {
		return result, errors.New("amount does not equal quantity times price")
	}
	priceFloat, _ := price.Float64()
	amountFloat, _ := amount.Float64()
	if math.IsInf(priceFloat, 0) || math.IsInf(amountFloat, 0) || priceFloat == 0 || amountFloat == 0 {
		return result, errors.New("numeric value exceeds DTO range")
	}
	return dto.ParsedTrade{
		Ticker: fields["ticker"], Lot: int(lot.Num().Int64()), Price: priceFloat,
		Type: side, Date: date, BrokerName: c.template.BrokerName, AmountDone: amountFloat,
	}, nil
}

func (n MetadataNumbers) validate() error {
	valid := func(s string) bool {
		if utf8.RuneCountInString(s) != 1 {
			return false
		}
		for _, r := range s {
			if unicode.IsDigit(r) || unicode.IsSpace(r) || unicode.IsLetter(r) || strings.ContainsRune("+-/", r) {
				return false
			}
		}
		return true
	}
	if !valid(n.DecimalSeparator) || (n.ThousandsSeparator != "" && !valid(n.ThousandsSeparator)) || n.DecimalSeparator == n.ThousandsSeparator {
		return errors.New("invalid numeric separators")
	}
	if n.GroupingSize < 1 || n.GroupingSize > 9 {
		return errors.New("grouping_size must be between 1 and 9")
	}
	return nil
}

func (n MetadataNumbers) parse(raw string) (*big.Rat, error) {
	invalid := errors.New("invalid numeric value")
	if len(raw) == 0 || len(raw) > 256 {
		return nil, invalid
	}
	digits := func(s string) bool {
		if s == "" {
			return false
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	parts := strings.Split(raw, n.DecimalSeparator)
	if len(parts) > 2 || (len(parts) == 2 && !digits(parts[1])) {
		return nil, invalid
	}
	integer := parts[0]
	if n.ThousandsSeparator != "" && strings.Contains(integer, n.ThousandsSeparator) {
		groups := strings.Split(integer, n.ThousandsSeparator)
		for i, group := range groups {
			if !digits(group) || (i == 0 && len(group) > n.GroupingSize) || (i > 0 && len(group) != n.GroupingSize) {
				return nil, invalid
			}
		}
		integer = strings.Join(groups, "")
	}
	if !digits(integer) {
		return nil, invalid
	}
	normalized := integer
	if len(parts) == 2 {
		normalized += "." + parts[1]
	}
	value, ok := new(big.Rat).SetString(normalized)
	if !ok {
		return nil, invalid
	}
	return value, nil
}
