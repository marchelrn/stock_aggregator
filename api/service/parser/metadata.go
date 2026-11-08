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
	From     string
	Subject  string
	Filename string
}

// MetadataConfig is the version 1 JSON contract. Patterns use Go/RE2 syntax;
// capture indexes are one-based. Every non-whitespace table byte must belong
// to a complete row, so malformed rows cannot be silently skipped.
type MetadataConfig struct {
	Schema          string          `json:"$schema,omitempty"`
	SchemaVersion   int             `json:"schema_version"`
	RequiredMarkers []string        `json:"required_markers"`
	Date            MetadataDate    `json:"date"`
	Table           MetadataTable   `json:"table"`
	Numbers         MetadataNumbers `json:"numbers"`
	SharesPerLot    int             `json:"shares_per_lot"`
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
	templates, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	selectors := make([]EmailMetadata, 0, len(templates))
	for _, template := range templates {
		selectors = append(selectors, EmailMetadata{
			From:     template.template.EmailSender,
			Subject:  template.template.SubjectContains,
			Filename: template.template.FilenameSuffix,
		})
	}
	return selectors, nil
}

type compiledMetadata struct {
	template models.PDFParsingTemplate
	config   MetadataConfig
	markers  []*regexp.Regexp
	date     *regexp.Regexp
	start    *regexp.Regexp
	end      *regexp.Regexp
	row      *regexp.Regexp
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
		if !template.Active {
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
	if cfg.SchemaVersion != 1 || len(cfg.RequiredMarkers) == 0 || cfg.SharesPerLot <= 0 {
		return c, errors.New("schema_version must be 1; markers and positive shares_per_lot required")
	}
	compile := func(pattern string) (*regexp.Regexp, error) {
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
	var err error
	for _, marker := range cfg.RequiredMarkers {
		r, err := compile(marker)
		if err != nil {
			return c, fmt.Errorf("required marker: %w", err)
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
		*pattern.target, err = compile(pattern.pattern)
		if err != nil {
			return c, fmt.Errorf("%s: %w", pattern.name, err)
		}
	}
	if cfg.Date.Group < 1 || cfg.Date.Group > c.date.NumSubexp() || cfg.Date.Layout == "" {
		return c, errors.New("invalid date capture group or layout")
	}
	// A layout must encode a full calendar date, not silently default components.
	a := time.Date(2001, 2, 3, 0, 0, 0, 0, time.UTC)
	b := time.Date(2004, 11, 23, 0, 0, 0, 0, time.UTC)
	for _, value := range []time.Time{a, b} {
		parsed, e := time.Parse(cfg.Date.Layout, value.Format(cfg.Date.Layout))
		if e != nil || parsed.Year() != value.Year() || parsed.Month() != value.Month() || parsed.Day() != value.Day() {
			return c, errors.New("date layout must encode year, month and day")
		}
	}
	fields := []string{"ticker", "lot", "quantity", "price", "buy_amount", "sell_amount"}
	if len(cfg.Table.Fields) != len(fields) {
		return c, errors.New("exactly six trade field mappings required")
	}
	used := make(map[int]bool)
	for _, field := range fields {
		group := cfg.Table.Fields[field]
		if group < 1 || group > c.row.NumSubexp() || used[group] {
			return c, fmt.Errorf("invalid or duplicate capture group for %s", field)
		}
		used[group] = true
	}
	if err := cfg.Numbers.validate(); err != nil {
		return c, err
	}
	if t.EmailSender != "" {
		address, err := mail.ParseAddress(t.EmailSender)
		if err != nil || !strings.EqualFold(address.Address, t.EmailSender) {
			return c, errors.New("email_sender must be a bare mailbox address")
		}
	}
	return c, nil
}

func (p *MetadataParser) parse(ctx context.Context, data []byte, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	templates, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
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
	content, err := extractMetadataPDF(ctx, data)
	if err != nil {
		return nil, err
	}
	return parseMetadataContent(ctx, templates, content, meta)
}

func extractMetadataPDF(ctx context.Context, data []byte) (content string, err error) {
	// Malformed PDFs can panic inside the extractor. Do not return partial data
	// or include extractor errors that could contain document text.
	defer func() {
		if recover() != nil {
			content, err = "", errors.New("PDF extraction failed")
		}
	}()
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
		matches := meta == nil || c.matchesEmail(*meta)
		for _, marker := range c.markers {
			matches = matches && marker.MatchString(content)
		}
		if !matches {
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
	trades, err := selected.parseTrades(content)
	if err != nil {
		return nil, fmt.Errorf("PDF template %q version %d: %w", selected.template.Key, selected.template.Version, err)
	}
	return trades, nil
}

func (c *compiledMetadata) matchesEmail(meta EmailMetadata) bool {
	t := c.template
	if t.EmailSender != "" {
		from, err := mail.ParseAddress(meta.From)
		if err != nil || !strings.EqualFold(from.Address, t.EmailSender) {
			return false
		}
	}
	return strings.Contains(strings.ToLower(meta.Subject), strings.ToLower(t.SubjectContains)) &&
		strings.HasSuffix(strings.ToLower(meta.Filename), strings.ToLower(t.FilenameSuffix))
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
