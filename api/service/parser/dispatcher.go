package parser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/marchelrn/stock_api/dto"
)

// BrokerDispatcherParser routes parsing requests using the active templates in
// the database. A template either names the metadata engine or a built-in
// legacy extractor, so adding or moving a broker never requires new routing
// branches here. The old content-sniffing chain remains only as a logged
// fallback for emails that no template covers yet.
type BrokerDispatcherParser struct {
	stockbit *StockbitParser
	eipo     *StockbitEIPOParser
	mandiri  *MandiriParser
	trimegah *TrimegahParser
	metadata *MetadataParser
	fallback bool
	compare  bool
}

func NewBrokerDispatcherParser(metadata *MetadataParser, stockbit *StockbitParser, mandiri *MandiriParser, trimegah *TrimegahParser) *BrokerDispatcherParser {
	return &BrokerDispatcherParser{
		stockbit: stockbit,
		eipo:     NewStockbitEIPOParser(),
		mandiri:  mandiri,
		trimegah: trimegah,
		metadata: metadata,
		fallback: legacyFallbackEnabled(),
		compare:  strings.EqualFold(os.Getenv("PDF_PARSER_COMPARE"), "true"),
	}
}

// legacyFallbackEnabled reads PDF_PARSER_FALLBACK. Default is on so existing
// mailboxes keep working while templates are migrated; set it to false once
// every broker has a template to make routing strict.
func legacyFallbackEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("PDF_PARSER_FALLBACK")))
	return value != "false" && value != "0" && value != "off"
}

func (d *BrokerDispatcherParser) BrokerName() string {
	return "Broker Dispatcher"
}

// selectTemplate returns the single active trade template whose selectors match
// the email. No match is not an error: callers may still use the fallback.
func (d *BrokerDispatcherParser) selectTemplate(ctx context.Context, meta EmailMetadata) (*compiledMetadata, error) {
	if d.metadata == nil {
		return nil, errors.New("metadata parser is not configured")
	}
	templates, err := d.metadata.load(ctx)
	if err != nil {
		return nil, err
	}
	var matches []compiledMetadata
	excluded := false
	for _, template := range templates {
		if template.kind != KindTrade || template.document != DocumentPDF || template.discoveryOnly {
			continue
		}
		if !template.matchesEmailSelectors(meta) {
			continue
		}
		if template.excludesFilename(meta.Filename) {
			excluded = true
			continue
		}
		matches = append(matches, template)
	}
	if len(matches) > 1 {
		keys := make([]string, 0, len(matches))
		for _, match := range matches {
			keys = append(keys, fmt.Sprintf("%s v%d", match.template.Key, match.template.Version))
		}
		return nil, fmt.Errorf("ambiguous email: multiple active templates match (%s)", strings.Join(keys, ", "))
	}
	if len(matches) == 0 {
		if excluded {
			return nil, ErrEmailNotApplicable
		}
		return nil, nil
	}
	return &matches[0], nil
}

func (d *BrokerDispatcherParser) legacyEmailParser(name string) emailTradeParser {
	switch name {
	case "stockbit":
		return d.stockbit
	case "trimegah":
		return d.trimegah
	case "mandiri":
		return d.mandiri
	case "eipo":
		return d.eipo
	default:
		return nil
	}
}

type emailTradeParser interface {
	ParseEmail(context.Context, []byte, EmailMetadata) ([]dto.ParsedTrade, error)
}

func (d *BrokerDispatcherParser) Parse(pdfBytes []byte) ([]dto.ParsedTrade, error) {
	if d.metadata == nil {
		return nil, errors.New("metadata parser is not configured")
	}
	trades, err := d.metadata.Parse(pdfBytes)
	if err == nil || !d.fallback {
		return trades, err
	}
	// Check configuration separately: a broken store must not bypass routing.
	if _, configErr := d.metadata.load(context.Background()); configErr != nil {
		return nil, configErr
	}
	log.Printf("PDF metadata parsing unavailable; using legacy fallback")
	return d.parseLegacyPDF(pdfBytes)
}

func (d *BrokerDispatcherParser) parseLegacyPDF(pdfBytes []byte) ([]dto.ParsedTrade, error) {
	if bytes.Contains(pdfBytes, []byte("STOCKBIT")) {
		trades, err := d.stockbit.Parse(pdfBytes)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
	}

	if bytes.Contains(pdfBytes, []byte("MANDIRI")) {
		trades, err := d.mandiri.Parse(pdfBytes)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
	}

	if bytes.Contains(pdfBytes, []byte("TRIMEGAH")) {
		trades, err := d.trimegah.Parse(pdfBytes)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
	}

	// Try stockbit parser first
	trades, err := d.stockbit.Parse(pdfBytes)
	if err == nil && len(trades) > 0 {
		return trades, nil
	}

	// IPO allotment confirmations keep an "Allotment Date" header instead of
	// the regular trade table.
	if bytes.Contains(pdfBytes, []byte("Allotment Date")) {
		trades, err := d.eipo.Parse(pdfBytes)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
	}

	return nil, err
}

func (d *BrokerDispatcherParser) ParseEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(meta.Filename), "_tcbond_") {
		return nil, ErrEmailNotApplicable
	}
	template, routeErr := d.selectTemplate(ctx, meta)
	if routeErr != nil {
		return nil, routeErr
	}

	if template != nil {
		if template.parseMode == ParseModeLegacy {
			extractor := d.legacyEmailParser(template.legacyParser)
			if extractor == nil {
				if !d.fallback {
					return nil, fmt.Errorf("template %q references unknown legacy_parser %q", template.template.Key, template.legacyParser)
				}
				log.Printf("Template %q has no usable extractor, using legacy fallback", template.template.Key)
				return d.parseLegacyEmail(ctx, data, meta)
			}
			trades, err := extractor.ParseEmail(ctx, data, meta)
			if err == nil && len(trades) > 0 {
				return trades, nil
			}
			if err == nil {
				err = errors.New("legacy extractor returned no trades")
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if !d.fallback {
				return trades, err
			}
			log.Printf("Template %q legacy extractor did not apply (%v), using fallback", template.template.Key, err)
			return d.parseLegacyEmail(ctx, data, meta)
		}

		trades, err := parseTradePDF(ctx, data, []compiledMetadata{*template}, &meta)
		if err == nil {
			if d.compare {
				legacy, legacyErr := d.parseLegacyEmail(ctx, data, meta)
				log.Printf("PDF comparison template=%q version=%d equal=%t metadata_rows=%d legacy_rows=%d legacy_failed=%t",
					template.template.Key, template.template.Version, legacyErr == nil && reflect.DeepEqual(trades, legacy),
					len(trades), len(legacy), legacyErr != nil)
			}
			return trades, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !d.fallback || template.config.Trade != nil && template.config.Trade.PasswordSource != "" {
			return nil, err
		}
		log.Printf("Template %q metadata parsing failed (%v), using fallback", template.template.Key, err)
		trades, fallbackErr := d.parseLegacyEmail(ctx, data, meta)
		if errors.Is(fallbackErr, ErrNoStockbitTradeRows) {
			return nil, fmt.Errorf("%w (template %s v%d: %v)", fallbackErr, template.template.Key, template.template.Version, err)
		}
		return trades, fallbackErr
	}

	if !d.fallback {
		return nil, ErrEmailNotApplicable
	}
	log.Printf("No PDF template matched email; using legacy fallback")
	trades, err := d.parseLegacyEmail(ctx, data, meta)
	if errors.Is(err, ErrNoStockbitTradeRows) {
		return nil, fmt.Errorf("%w (pengirim, subjek, atau nama lampiran tidak cocok dengan template aktif)", err)
	}
	return trades, err
}

// parseLegacyEmail is the pre-template routing chain, kept for mailboxes and
// brokers that do not have a database template yet.
func (d *BrokerDispatcherParser) parseLegacyEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	from := strings.ToLower(meta.From)
	subj := strings.ToLower(meta.Subject)
	filename := strings.ToLower(meta.Filename)

	if strings.Contains(from, "trimegah") || strings.Contains(from, "trim.operation") || strings.Contains(subj, "trimegah") || strings.Contains(filename, "trimegah") || bytes.Contains(data, []byte("TRIMEGAH")) {
		return d.trimegah.ParseEmail(ctx, data, meta)
	}

	// IPO allotment confirmations (TC_EIPO_*.pdf) use a different table layout
	// (Buy/Refund columns) and must be routed before the regular Stockbit
	// parser, which rejects them as inapplicable.
	if strings.Contains(filename, "eipo") || strings.Contains(subj, "ipo") {
		trades, err := d.eipo.ParseEmail(ctx, data, meta)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
		if errors.Is(err, ErrEmailNotApplicable) {
			return nil, ErrEmailNotApplicable
		}
	}

	if strings.Contains(from, "stockbit") || strings.Contains(subj, "stockbit") || strings.Contains(filename, "tc_") || bytes.Contains(data, []byte("STOCKBIT")) {
		trades, err := d.stockbit.ParseEmail(ctx, data, meta)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
		if errors.Is(err, ErrEmailNotApplicable) {
			return nil, ErrEmailNotApplicable
		}
	}

	if strings.Contains(from, "mandiri") || strings.Contains(subj, "mandiri") || bytes.Contains(data, []byte("MANDIRI")) {
		trades, err := d.mandiri.Parse(data)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
	}

	if strings.Contains(subj, "tc") {
		trades, err := d.trimegah.ParseEmail(ctx, data, meta)
		if err == nil && len(trades) > 0 {
			return trades, nil
		}
		if errors.Is(err, ErrEmailNotApplicable) {
			return nil, ErrEmailNotApplicable
		}
	}

	// Try stockbit parser
	trades, err := d.stockbit.ParseEmail(ctx, data, meta)
	if err == nil && len(trades) > 0 {
		return trades, nil
	}
	if errors.Is(err, ErrEmailNotApplicable) {
		return nil, ErrEmailNotApplicable
	}

	return nil, err
}

func (d *BrokerDispatcherParser) EmailSelectors(ctx context.Context) ([]EmailMetadata, error) {
	if d.metadata == nil {
		return nil, nil
	}
	return d.metadata.EmailSelectors(ctx)
}

// EmailSelectorsForKinds narrows discovery to trade or cash/dividend templates
// so one query never pulls in another kind's senders.
func (d *BrokerDispatcherParser) EmailSelectorsForKinds(ctx context.Context, kinds ...string) ([]EmailMetadata, error) {
	if d.metadata == nil {
		return nil, nil
	}
	return d.metadata.EmailSelectorsForKinds(ctx, kinds...)
}

// ReviewBroker requires positive trade-document evidence in addition to the active
// broker selectors. Some senders also deliver account statements from the same mailbox.
func (d *BrokerDispatcherParser) ReviewBroker(ctx context.Context, meta EmailMetadata) (string, error) {
	name := strings.ToLower(meta.Filename)
	if strings.Contains(name, "_tcbond_") {
		return "", nil
	}
	t, err := d.selectTemplate(ctx, meta)
	if errors.Is(err, ErrEmailNotApplicable) {
		return "", nil
	}
	if err != nil || t == nil {
		return "", err
	}
	if t.template.EmailSender == "" && t.config.SenderDomain == "" {
		return "", nil
	}
	if t.template.SubjectContains == "" && t.subject == nil && t.filename == nil &&
		!strings.Contains(strings.ToLower(meta.Subject), "trade confirmation") && !strings.HasPrefix(name, "tc_") && !strings.HasPrefix(name, "tc-") {
		return "", nil
	}
	return t.template.BrokerName, nil
}
