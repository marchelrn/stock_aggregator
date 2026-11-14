package parser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/marchelrn/stock_api/dto"
)

// BrokerDispatcherParser routes parsing requests to broker-specific parsers
// (Stockbit, EIPO, Mandiri, Trimegah) based on email metadata or PDF content, falling back to MetadataParser.
type BrokerDispatcherParser struct {
	stockbit *StockbitParser
	eipo     *StockbitEIPOParser
	mandiri  *MandiriParser
	trimegah *TrimegahParser
	metadata *MetadataParser
}

func NewBrokerDispatcherParser(metadata *MetadataParser, stockbit *StockbitParser, mandiri *MandiriParser, trimegah *TrimegahParser) *BrokerDispatcherParser {
	return &BrokerDispatcherParser{
		stockbit: stockbit,
		eipo:     NewStockbitEIPOParser(),
		mandiri:  mandiri,
		trimegah: trimegah,
		metadata: metadata,
	}
}

func (d *BrokerDispatcherParser) BrokerName() string {
	return "Broker Dispatcher"
}

func (d *BrokerDispatcherParser) Parse(pdfBytes []byte) ([]dto.ParsedTrade, error) {
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

	if d.metadata != nil {
		return d.metadata.Parse(pdfBytes)
	}

	return nil, err
}

func (d *BrokerDispatcherParser) ParseEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	from := strings.ToLower(meta.From)
	subj := strings.ToLower(meta.Subject)
	filename := strings.ToLower(meta.Filename)

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

	if strings.Contains(from, "trimegah") || strings.Contains(from, "trim.operation") || strings.Contains(subj, "tc") || bytes.Contains(data, []byte("TRIMEGAH")) {
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

	if d.metadata != nil {
		trades, err := d.metadata.ParseEmail(ctx, data, meta)
		if err != nil {
			if strings.Contains(err.Error(), "no extractable text") || strings.Contains(err.Error(), "no active template matches") {
				return nil, fmt.Errorf("%w: %s", ErrEmailNotApplicable, err.Error())
			}
			return nil, err
		}
		return trades, nil
	}

	return nil, err
}

func (d *BrokerDispatcherParser) EmailSelectors(ctx context.Context) ([]EmailMetadata, error) {
	if d.metadata != nil {
		selectors, err := d.metadata.EmailSelectors(ctx)
		if err == nil && len(selectors) > 0 {
			return selectors, nil
		}
	}
	return []EmailMetadata{
		{From: "no-reply@stockbit.com", Subject: "Trade Confirmation"},
		{From: "trim.operation@trimegah.com", Subject: "TC"},
		{From: "trim.operation@trimegah.com", Subject: "Trade Confirmation"},
	}, nil
}
