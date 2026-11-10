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
// (Stockbit, Mandiri) based on email metadata or PDF content, falling back to MetadataParser.
type BrokerDispatcherParser struct {
	stockbit *StockbitParser
	mandiri  *MandiriParser
	metadata *MetadataParser
}

func NewBrokerDispatcherParser(metadata *MetadataParser, stockbit *StockbitParser, mandiri *MandiriParser) *BrokerDispatcherParser {
	return &BrokerDispatcherParser{
		stockbit: stockbit,
		mandiri:  mandiri,
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

	// Try stockbit parser first
	trades, err := d.stockbit.Parse(pdfBytes)
	if err == nil && len(trades) > 0 {
		return trades, nil
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
	}, nil
}
