package parser

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/models"
	"golang.org/x/net/html"
)

// CashEmailParser is a validated snapshot, shared by messages in one sync.
// Reloading at the start of the next sync picks up database edits.
type CashEmailParser struct {
	templates []compiledMetadata
}

var ErrCashAmountNotFound = errors.New("cash amount not found")

var errEmailFieldNotFound = errors.New("required field not found")

type ParsedCashMutation struct {
	BrokerName  string                  `json:"broker_name"`
	Type        models.CashMutationType `json:"type"`
	Amount      float64                 `json:"amount"`
	Date        time.Time               `json:"date"`
	RefNo       string                  `json:"ref_no,omitempty"`
	Description string                  `json:"description,omitempty"`
}

func (p *MetadataParser) CashParser(ctx context.Context) (*CashEmailParser, error) {
	all, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	result := &CashEmailParser{}
	for _, c := range all {
		if c.document == DocumentEmailBody && (c.kind == KindCash || c.kind == KindDividend) && !c.discoveryOnly {
			result.templates = append(result.templates, c)
		}
	}
	if len(result.templates) == 0 {
		return nil, errors.New("no active cash or dividend templates")
	}
	return result, nil
}

func (d *BrokerDispatcherParser) CashParser(ctx context.Context) (*CashEmailParser, error) {
	if d.metadata == nil {
		return nil, errors.New("metadata parser is not configured")
	}
	return d.metadata.CashParser(ctx)
}

func (p *CashEmailParser) Parse(ctx context.Context, meta EmailMetadata, emailDate time.Time, body string) (*ParsedCashMutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var selected *compiledMetadata
	for i := range p.templates {
		c := &p.templates[i]
		if !c.matchesEmail(meta) {
			continue
		}
		if selected != nil {
			return nil, errors.New("ambiguous cash email: multiple active templates match")
		}
		selected = c
	}
	if selected == nil {
		return nil, nil
	}
	if emailDate.IsZero() {
		return nil, errors.New("cash email date is required")
	}
	// Both text and HTML are normalized to the same representation. Executable
	// and style content is ignored rather than treated as financial data.
	text := normalizeEmailBody(body)
	if !domesticCashNotice(meta.Subject, text) {
		return nil, nil
	}
	for _, marker := range selected.markers {
		if !marker.MatchString(text) {
			return nil, fmt.Errorf("cash template %q: required marker missing", selected.template.Key)
		}
	}
	result, err := selected.parseCash(meta, emailDate, text)
	if err != nil {
		return nil, fmt.Errorf("cash template %q version %d: %w", selected.template.Key, selected.template.Version, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeEmailBody(body string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(body))
	var text strings.Builder
	hidden := 0
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.Join(strings.Fields(text.String()), " ")
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "script" || string(name) == "style" {
				hidden++
			}
			text.WriteByte(' ')
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if (string(name) == "script" || string(name) == "style") && hidden > 0 {
				hidden--
			}
			text.WriteByte(' ')
		case html.SelfClosingTagToken:
			text.WriteByte(' ')
		case html.TextToken:
			if hidden == 0 {
				text.Write(tokenizer.Text())
			}
		}
	}
}

// A repeated label is safe only when every occurrence has the same value.
// This handles duplicated mobile/desktop email layouts without silently
// accepting conflicting amounts or references.
func captureEmailField(pattern *regexp.Regexp, group int, text string, required bool) (string, error) {
	if pattern == nil {
		return "", nil
	}
	var value string
	for _, match := range pattern.FindAllStringSubmatch(text, -1) {
		if group >= len(match) || strings.TrimSpace(match[group]) == "" {
			return "", errors.New("empty capture")
		}
		next := strings.TrimSpace(match[group])
		if value != "" && value != next {
			return "", errors.New("conflicting field values")
		}
		value = next
	}
	if value == "" && required {
		return "", errEmailFieldNotFound
	}
	return value, nil
}

func (c *compiledMetadata) parseCash(meta EmailMetadata, emailDate time.Time, body string) (*ParsedCashMutation, error) {
	email := c.config.Email
	raw, err := captureEmailField(c.emailAmount, email.AmountGroup, body, true)
	if err != nil {
		if errors.Is(err, errEmailFieldNotFound) {
			return nil, fmt.Errorf("%w: amount: %w", ErrCashAmountNotFound, err)
		}
		return nil, fmt.Errorf("amount: %w", err)
	}
	amount, err := c.config.Numbers.parse(raw)
	if err != nil || amount.Sign() <= 0 {
		return nil, errors.New("amount must be positive with configured numeric separators")
	}
	value, _ := amount.Float64()
	if math.IsInf(value, 0) || value == 0 {
		return nil, errors.New("amount exceeds numeric range")
	}
	date := emailDate
	if c.emailDate != nil {
		rawDate, err := captureEmailField(c.emailDate, email.DateGroup, body, false)
		if err != nil {
			return nil, fmt.Errorf("date: %w", err)
		}
		if rawDate != "" {
			words := strings.Fields(rawDate)
			for i, word := range words {
				if replacement, ok := email.DateReplacements[strings.ToLower(word)]; ok {
					words[i] = replacement
				}
			}
			normalized := strings.Join(words, " ")
			date, err = time.Parse(email.DateLayout, normalized)
			if err != nil {
				return nil, errors.New("invalid payment date")
			}
		}
	}
	tickerText := meta.Subject
	if email.TickerSource == "body" {
		tickerText = body
	}
	ticker, err := captureEmailField(c.emailTicker, email.TickerGroup, tickerText, strings.Contains(c.config.RefTemplate, "{ticker}"))
	if err != nil {
		return nil, fmt.Errorf("ticker: %w", err)
	}
	ticker = strings.ToUpper(ticker)
	ref, err := captureEmailField(c.emailRef, email.RefGroup, body, strings.Contains(c.config.RefTemplate, "{ref}"))
	if err != nil {
		return nil, fmt.Errorf("reference: %w", err)
	}
	replace := strings.NewReplacer("{ticker}", ticker, "{paydate}", date.Format("20060102"), "{ref}", ref)
	description := replace.Replace(email.Description)
	if c.config.RefTemplate != "" {
		ref = replace.Replace(c.config.RefTemplate)
	}
	return &ParsedCashMutation{
		BrokerName: c.template.BrokerName,
		Type:       models.CashMutationType(strings.ToUpper(c.emailType)),
		Amount:     value, Date: date, RefNo: ref, Description: description,
	}, nil
}
