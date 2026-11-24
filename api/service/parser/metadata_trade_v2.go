package parser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
)

// MetadataTrade contains broker-independent policies for rows, references and
// fee allocation. Labels, capture groups and rates belong to the DB template.
type MetadataTrade struct {
	IgnorePatterns   []string                     `json:"ignore_patterns,omitempty"`
	PasswordSource   string                       `json:"password_source,omitempty"`
	Extraction       string                       `json:"extraction"`
	DateReplacements map[string]string            `json:"date_replacements,omitempty"`
	SideValues       map[string]string            `json:"side_values,omitempty"`
	ReferenceFields  map[string]MetadataTextField `json:"reference_fields,omitempty"`
	RefTemplate      string                       `json:"ref_template,omitempty"`
	Fees             []MetadataFee                `json:"fees,omitempty"`
	FeeAllocation    string                       `json:"fee_allocation"`
	BuyFallbackRate  float64                      `json:"buy_fallback_rate"`
	SellFallbackRate float64                      `json:"sell_fallback_rate"`
	SellAmount       string                       `json:"sell_amount"`
}

type pdfPasswordsContextKey struct{}

func WithPDFPasswords(ctx context.Context, passwords map[string]string) context.Context {
	copy := make(map[string]string, len(passwords))
	for broker, password := range passwords {
		copy[strings.ToLower(strings.TrimSpace(broker))] = password
	}
	return context.WithValue(ctx, pdfPasswordsContextKey{}, copy)
}

type MetadataTextField struct {
	Source  string `json:"source"`
	Pattern string `json:"pattern"`
	Group   int    `json:"group"`
}

type MetadataFee struct {
	Pattern    string `json:"pattern"`
	BuyGroup   int    `json:"buy_group,omitempty"`
	SellGroup  int    `json:"sell_group,omitempty"`
	TotalGroup int    `json:"total_group,omitempty"`
}

func (c *compiledMetadata) extractPDF(ctx context.Context, data []byte, mode string) (string, error) {
	if mode != "layout" {
		return extractMetadataPDF(ctx, data)
	}
	args := []string{"-layout"}
	if c.config.Trade.PasswordSource == "broker" {
		passwords, _ := ctx.Value(pdfPasswordsContextKey{}).(map[string]string)
		password := passwords[strings.ToLower(strings.TrimSpace(c.template.BrokerName))]
		if password == "" {
			return "", errors.New("password PDF broker belum dikonfigurasi")
		}
		args = append(args, "-upw", password)
	}
	args = append(args, "-", "-")
	cmd := exec.CommandContext(ctx, "pdftotext", args...)
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("PDF layout extraction failed")
	}
	return string(output), nil
}

func (c *compiledMetadata) compileGenericTrade() (compiledMetadata, error) {
	cfg := c.config
	t := cfg.Trade
	for _, pattern := range t.IgnorePatterns {
		if _, err := compilePattern(pattern); err != nil {
			return *c, fmt.Errorf("trade ignore pattern: %w", err)
		}
	}
	if t.PasswordSource != "" && t.PasswordSource != "broker" {
		return *c, errors.New("unsupported PDF password source")
	}
	if t.PasswordSource != "" && t.Extraction != "layout" {
		return *c, errors.New("PDF password requires layout extraction")
	}
	if cfg.SchemaVersion != 2 || (t.Extraction != "layout" && t.Extraction != "plain" && t.Extraction != "plain_then_layout") {
		return *c, errors.New("generic trade requires schema_version 2 and a supported extraction mode")
	}
	for _, field := range []string{"ticker", "quantity", "price"} {
		if cfg.Table.Fields[field] < 1 {
			return *c, fmt.Errorf("missing trade field %s", field)
		}
	}
	allowed := map[string]bool{"ticker": true, "quantity": true, "price": true, "lot": true, "buy_amount": true, "sell_amount": true, "side": true, "amount": true, "ref": true}
	for field, group := range cfg.Table.Fields {
		if !allowed[field] || group < 1 || group > c.row.NumSubexp() {
			return *c, errors.New("invalid trade field mapping")
		}
	}
	if cfg.Table.Fields["side"] == 0 && (cfg.Table.Fields["buy_amount"] == 0 || cfg.Table.Fields["sell_amount"] == 0) {
		return *c, errors.New("trade requires side or buy/sell fields")
	}
	for _, side := range t.SideValues {
		if side != "BUY" && side != "SELL" {
			return *c, errors.New("invalid side mapping")
		}
	}
	if t.FeeAllocation != "side" && t.FeeAllocation != "all" {
		return *c, errors.New("invalid fee allocation")
	}
	if t.SellAmount != "add" && t.SellAmount != "subtract" {
		return *c, errors.New("invalid sell amount policy")
	}
	for _, rate := range []float64{t.BuyFallbackRate, t.SellFallbackRate} {
		if rate < 0 || rate > 1 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return *c, errors.New("invalid fee fallback rate")
		}
	}
	for _, fee := range t.Fees {
		r, err := compilePattern(fee.Pattern)
		if err != nil {
			return *c, fmt.Errorf("fee pattern: %w", err)
		}
		if fee.BuyGroup == 0 && fee.SellGroup == 0 && fee.TotalGroup == 0 {
			return *c, errors.New("fee capture group required")
		}
		for _, group := range []int{fee.BuyGroup, fee.SellGroup, fee.TotalGroup} {
			if group < 0 || group > r.NumSubexp() {
				return *c, errors.New("invalid fee capture group")
			}
		}
		c.tradeFees = append(c.tradeFees, r)
	}
	c.tradeRefs = make(map[string]*regexp.Regexp)
	for name, field := range t.ReferenceFields {
		if name == "" || strings.ContainsAny(name, "{}") {
			return *c, errors.New("invalid reference field name")
		}
		if field.Source != "body" && field.Source != "subject" && field.Source != "filename" {
			return *c, errors.New("invalid reference field source")
		}
		r, err := compilePattern(field.Pattern)
		if err != nil || field.Group < 1 || field.Group > r.NumSubexp() {
			return *c, errors.New("invalid reference field")
		}
		c.tradeRefs[name] = r
	}
	rest := strings.NewReplacer("{date}", "", "{ticker}", "", "{sequence}", "", "{ref}", "").Replace(t.RefTemplate)
	for name := range t.ReferenceFields {
		rest = strings.ReplaceAll(rest, "{"+name+"}", "")
	}
	if strings.ContainsAny(rest, "{}") {
		return *c, errors.New("unknown trade reference placeholder")
	}
	if err := cfg.Numbers.validate(); err != nil {
		return *c, err
	}
	return *c, nil
}

func (c *compiledMetadata) parseGenericTrades(content string, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	cfg, policy := c.config, c.config.Trade
	for _, pattern := range policy.IgnorePatterns {
		r, err := compilePattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("trade ignore pattern: %w", err)
		}
		content = r.ReplaceAllString(content, "\n")
	}
	rawDate, err := captureEmailField(c.date, cfg.Date.Group, content, true)
	if err != nil {
		return nil, fmt.Errorf("trade date: %w", err)
	}
	words := strings.Fields(rawDate)
	for i, word := range words {
		if v, ok := policy.DateReplacements[strings.ToLower(word)]; ok {
			words[i] = v
		}
	}
	date, err := time.Parse(cfg.Date.Layout, strings.Join(words, " "))
	if err != nil {
		return nil, errors.New("invalid trade date")
	}
	starts := c.start.FindAllStringIndex(content, -1)
	if len(starts) != 1 {
		return nil, errors.New("exactly one trade table start required")
	}
	rest := content[starts[0][1]:]
	end := c.end.FindStringIndex(rest)
	if end == nil {
		return nil, errors.New("trade table end missing")
	}
	table := rest[:end[0]]
	rows := c.row.FindAllStringSubmatchIndex(table, -1)
	if len(rows) == 0 {
		return nil, errors.New("empty trade table")
	}
	var trades []dto.ParsedTrade
	position := 0
	for _, row := range rows {
		if strings.TrimSpace(table[position:row[0]]) != "" {
			return nil, errors.New("unrecognized trade table content")
		}
		fields := map[string]string{}
		for name, group := range cfg.Table.Fields {
			a, b := row[group*2], row[group*2+1]
			if a < 0 || b <= a {
				return nil, errors.New("missing trade field")
			}
			fields[name] = strings.TrimSpace(table[a:b])
		}
		numbers := map[string]float64{}
		for _, name := range []string{"quantity", "price", "lot", "buy_amount", "sell_amount", "amount"} {
			if fields[name] == "" {
				continue
			}
			n, err := cfg.Numbers.parse(fields[name])
			if err != nil {
				return nil, fmt.Errorf("invalid trade %s", name)
			}
			numbers[name], _ = n.Float64()
		}
		quantity, price := numbers["quantity"], numbers["price"]
		lots := quantity / float64(cfg.SharesPerLot)
		if quantity <= 0 || price <= 0 || math.Trunc(lots) != lots || lots > float64(^uint(0)>>1) || math.IsInf(lots, 0) {
			return nil, errors.New("invalid trade quantity or price")
		}
		if fields["lot"] != "" && numbers["lot"] != lots {
			return nil, errors.New("trade lot/quantity mismatch")
		}
		side := policy.SideValues[strings.ToUpper(fields["side"])]
		gross := quantity * price
		if math.IsInf(gross, 0) {
			return nil, errors.New("trade amount exceeds numeric range")
		}
		if fields["side"] == "" {
			switch {
			case numbers["buy_amount"] > 0 && numbers["sell_amount"] == 0:
				side = "BUY"
			case numbers["sell_amount"] > 0 && numbers["buy_amount"] == 0:
				side = "SELL"
			default:
				return nil, errors.New("invalid buy/sell amounts")
			}
			amount := numbers["buy_amount"] + numbers["sell_amount"]
			if math.Abs(amount-gross) > 0.005 {
				return nil, errors.New("trade gross amount mismatch")
			}
		}
		if side != "BUY" && side != "SELL" {
			return nil, errors.New("unmapped trade side")
		}
		if fields["amount"] != "" && math.Abs(numbers["amount"]-gross) > 0.005 {
			return nil, errors.New("trade gross amount mismatch")
		}
		ref := fields["ref"]
		if policy.RefTemplate != "" {
			replace := []string{"{date}", date.Format("20060102"), "{ticker}", fields["ticker"], "{sequence}", strconv.Itoa(len(trades) + 1), "{ref}", ref}
			for name, field := range policy.ReferenceFields {
				source := content
				if field.Source != "body" {
					if meta == nil {
						return nil, errors.New("email metadata required for reference")
					}
					source = meta.Subject
					if field.Source == "filename" {
						source = meta.Filename
					}
				}
				value, err := captureEmailField(c.tradeRefs[name], field.Group, source, true)
				if err != nil {
					return nil, fmt.Errorf("trade reference %s: %w", name, err)
				}
				replace = append(replace, "{"+name+"}", value)
			}
			ref = strings.NewReplacer(replace...).Replace(policy.RefTemplate)
		}
		trades = append(trades, dto.ParsedTrade{Ticker: fields["ticker"], Type: side, Lot: int(lots), Price: price, Date: date, BrokerName: c.template.BrokerName, RefNo: ref, AmountDone: gross})
		position = row[1]
	}
	if strings.TrimSpace(table[position:]) != "" {
		return nil, errors.New("unrecognized trailing trade")
	}
	fees := map[string]float64{"BUY": 0, "SELL": 0, "ALL": 0}
	for i, spec := range policy.Fees {
		for side, group := range map[string]int{"BUY": spec.BuyGroup, "SELL": spec.SellGroup, "ALL": spec.TotalGroup} {
			if group == 0 {
				continue
			}
			raw, err := captureEmailField(c.tradeFees[i], group, content, false)
			if err != nil {
				return nil, fmt.Errorf("fee: %w", err)
			}
			if raw == "" {
				continue
			}
			n, err := cfg.Numbers.parse(raw)
			if err != nil {
				return nil, errors.New("invalid fee amount")
			}
			f, _ := n.Float64()
			fees[side] += f
		}
	}
	totals := map[string]float64{"BUY": 0, "SELL": 0, "ALL": 0}
	for _, trade := range trades {
		totals[trade.Type] += trade.AmountDone
		totals["ALL"] += trade.AmountDone
	}
	for i := range trades {
		trade := &trades[i]
		gross := trade.AmountDone
		side := trade.Type
		if policy.FeeAllocation == "all" {
			side = "ALL"
		}
		rate := policy.BuyFallbackRate
		if trade.Type == "SELL" {
			rate = policy.SellFallbackRate
		}
		tax := gross * rate
		if fees[side] > 0 {
			tax = fees[side] * (gross / totals[side])
		}
		trade.Tax = math.Round(tax*100) / 100
		if math.IsNaN(trade.Tax) || math.IsInf(trade.Tax, 0) {
			return nil, errors.New("trade fees exceed numeric range")
		}
		if trade.Type == "SELL" && policy.SellAmount == "subtract" {
			trade.AmountDone = math.Round((gross-trade.Tax)*100) / 100
		} else {
			trade.AmountDone = math.Round((gross+trade.Tax)*100) / 100
		}
		if trade.AmountDone <= 0 || math.IsNaN(trade.AmountDone) || math.IsInf(trade.AmountDone, 0) {
			return nil, errors.New("invalid net trade amount")
		}
	}
	return trades, nil
}
