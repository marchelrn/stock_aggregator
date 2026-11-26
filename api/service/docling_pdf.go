package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
)

//go:embed docling_pdf.py
var doclingProgram string

const ajaibBroker = "Ajaib Sekuritas"

func doclingEnabled(broker string) bool {
	return os.Getenv("TC_FALLBACK_PROVIDER") == "docling" && strings.EqualFold(broker, ajaibBroker)
}

var errDoclingNoTable = errors.New("Docling belum menemukan tabel transaksi Ajaib.")

type doclingDocument struct {
	Texts  []string
	Tables []struct {
		Rows  int `json:"num_rows"`
		Cols  int `json:"num_cols"`
		Cells []struct {
			Row    int `json:"start_row_offset_idx"`
			Col    int `json:"start_col_offset_idx"`
			EndRow int `json:"end_row_offset_idx"`
			EndCol int `json:"end_col_offset_idx"`
			Text   string
		} `json:"table_cells"`
	}
}

func extractDoclingAjaib(ctx context.Context, pdf []byte) (*dto.TradeDraft, error) {
	if len(pdf) == 0 || len(pdf) > maxReviewBytes {
		return nil, errors.New("Ukuran PDF di luar batas ekstraksi.")
	}
	python := os.Getenv("DOCLING_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmd := exec.CommandContext(ctx, python, "-I", "-c", doclingProgram)
	cmd.Env = append(os.Environ(), "HF_HUB_OFFLINE=1", "OMP_NUM_THREADS=2")
	cmd.Stdin = bytes.NewReader(pdf)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("Waktu ekstraksi Docling habis.")
		}
		return nil, errors.New("Ekstraksi Docling gagal. Periksa Python, model lokal, dan PDF.")
	}
	if len(out) > 8<<20 {
		return nil, errors.New("Hasil konversi Docling terlalu besar.")
	}
	var doc doclingDocument
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, errors.New("Struktur JSON Docling tidak valid.")
	}
	return normalizeDoclingAjaib(doc)
}

var (
	doclingNumber     = regexp.MustCompile(`^(?:[0-9]+|[0-9]{1,3}(?:,[0-9]{3})+)(?:\.[0-9]{1,2})?$`)
	doclingDate       = regexp.MustCompile(`(?i)\b(?:tgl\.|tanggal) transaksi\s*:?\s*(\d{1,2} [A-Za-z]+ \d{4}|\d{4}-\d{2}-\d{2})\b`)
	doclingSettlement = regexp.MustCompile(`(?i)\b(Total Terima|Total Bayar|Total Tagihan)\s*:\s*((?:Rp\.?\s*)?(?:\(\s*[0-9][0-9,.]*\s*\)|[0-9][0-9,.]*))`)
)

func doclingAmount(raw string, allowAccountingNotation bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && strings.EqualFold(raw[:2], "Rp") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw[2:], "."))
	}
	if allowAccountingNotation && strings.HasPrefix(raw, "(") && strings.HasSuffix(raw, ")") {
		raw = strings.TrimSpace(raw[1 : len(raw)-1])
	}
	if !doclingNumber.MatchString(raw) {
		return "", errors.New("Format nominal pada sel PDF Ajaib tidak valid.")
	}
	return strings.ReplaceAll(raw, ",", ""), nil
}

func normalizeDoclingAjaib(doc doclingDocument) (*dto.TradeDraft, error) {
	draft := &dto.TradeDraft{}
	var printedGross, printedNet, topNet, topSide, side string
	var date string
	var fees []dto.ReviewFee
	var subtotal string
	recognized := false
	for _, line := range doc.Texts {
		line = strings.TrimSpace(line)
		for _, m := range doclingDate.FindAllStringSubmatch(line, -1) {
			if date != "" && date != m[1] {
				return nil, errors.New("Tanggal transaksi berbeda dalam PDF Ajaib.")
			}
			date = m[1]
		}
		for _, m := range doclingSettlement.FindAllStringSubmatch(line, -1) {
			if topNet != "" {
				return nil, errors.New("Docling membaca beberapa ringkasan neto Ajaib.")
			}
			topSide = "BUY"
			if strings.EqualFold(m[1], "Total Terima") {
				topSide = "SELL"
			}
			net, err := doclingAmount(m[2], topSide == "BUY")
			if err != nil {
				return nil, fmt.Errorf("Total bayar atau terima: %w", err)
			}
			topNet = net
		}
	}
	for _, table := range doc.Tables {
		if table.Rows < 1 || table.Rows > 204 || table.Cols < 1 || table.Cols > 24 {
			return nil, errors.New("Ukuran tabel Docling di luar batas impor.")
		}
		grid := make([][]string, table.Rows)
		for i := range grid {
			grid[i] = make([]string, table.Cols)
		}
		merged := false
		for _, c := range table.Cells {
			if c.Row < 0 || c.Row >= table.Rows || c.Col < 0 || c.Col >= table.Cols || grid[c.Row][c.Col] != "" {
				return nil, errors.New("Koordinat sel Docling tidak valid.")
			}
			grid[c.Row][c.Col] = strings.TrimSpace(c.Text)
			merged = merged || c.EndRow != c.Row+1 || c.EndCol != c.Col+1
		}
		header := grid[0]
		columns := map[string]int{}
		for i, label := range header {
			for role, meaning := range columnMeanings {
				if meaning.MatchString(label) {
					if _, duplicate := columns[role]; duplicate {
						return nil, errors.New("Peran kolom Docling muncul lebih dari sekali dalam satu tabel.")
					}
					columns[role] = i
				}
			}
		}
		if _, tradeTable := columns["ticker"]; tradeTable {
			if recognized || merged || len(columns) != 4 {
				return nil, errors.New("Struktur atau beberapa tabel transaksi Ajaib belum dapat direkonsiliasi.")
			}
			recognized = true
			grossColumn := columns["gross"]
			switch strings.ToLower(header[grossColumn]) {
			case "total beli", "bruto beli", "gross buy":
				side = "BUY"
			case "total jual", "bruto jual", "gross sell":
				side = "SELL"
			default:
				return nil, errors.New("Sisi beli atau jual belum terbaca dari header tabel Docling.")
			}
			for i, label := range header {
				if i != columns["ticker"] && i != columns["price"] && i != columns["shares"] && i != grossColumn && !strings.EqualFold(label, "Tipe") && !strings.EqualFold(label, "Board") {
					return nil, errors.New("Kolom tambahan pada tabel Docling belum dikenali.")
				}
			}
			for _, row := range grid[1:] {
				isSubtotal := false
				for i, cell := range row {
					if i == grossColumn || cell == "" {
						continue
					}
					if strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(cell, ":")), header[grossColumn]) && !isSubtotal {
						isSubtotal = true
						continue
					}
					if isSubtotal {
						isSubtotal = false
						break
					}
					break
				}
				if isSubtotal && subtotal == "" {
					v, err := doclingAmount(row[grossColumn], false)
					if err != nil {
						return nil, fmt.Errorf("Subtotal transaksi: %w", err)
					}
					subtotal = v
					continue
				}
				if subtotal != "" {
					return nil, errors.New("Docling menemukan baris tambahan di bawah subtotal transaksi.")
				}
				price, err := doclingAmount(row[columns["price"]], false)
				if err != nil {
					return nil, fmt.Errorf("Harga per lembar: %w", err)
				}
				shares, err := doclingAmount(row[columns["shares"]], false)
				if err != nil {
					return nil, fmt.Errorf("Jumlah lembar: %w", err)
				}
				gross, err := doclingAmount(row[grossColumn], false)
				if err != nil {
					return nil, fmt.Errorf("Bruto per transaksi: %w", err)
				}
				draft.Rows = append(draft.Rows, dto.ReviewFill{Ticker: row[columns["ticker"]], Side: side, Price: price, Shares: strings.TrimSuffix(strings.TrimSuffix(shares, ".00"), ".0"), Gross: gross})
			}
			continue
		}

		for _, row := range grid {
			if len(row) < 2 {
				continue
			}
			label := strings.TrimSpace(strings.TrimSuffix(row[0], ":"))
			for i, cell := range row[:len(row)-1] {
				cell = strings.TrimSpace(strings.TrimSuffix(cell, ":"))
				if strings.EqualFold(cell, "Tgl. Transaksi") || strings.EqualFold(cell, "Tanggal Transaksi") {
					v := strings.TrimSpace(strings.TrimPrefix(row[i+1], ":"))
					if date != "" && date != v {
						return nil, errors.New("Tanggal transaksi berbeda dalam PDF Ajaib.")
					}
					date = v
				}
			}
			switch {
			case strings.EqualFold(label, "Total Transaksi"), strings.EqualFold(label, "Total Keseluruhan"), feeLabel.MatchString(label):
				for _, middle := range row[1 : len(row)-1] {
					if middle != "" && middle != ":" {
						return nil, errors.New("Beberapa nominal dalam satu baris ringkasan Ajaib.")
					}
				}
				isFee := feeLabel.MatchString(label)
				value, err := doclingAmount(row[len(row)-1], isFee || topSide == "BUY")
				if err != nil {
					field := "Total bruto atau neto"
					if isFee {
						field = "Rincian biaya"
					}
					return nil, fmt.Errorf("%s: %w", field, err)
				}
				switch {
				case strings.EqualFold(label, "Total Transaksi"):
					if printedGross != "" {
						return nil, errors.New("Total bruto Ajaib muncul lebih dari sekali.")
					}
					printedGross = value
				case strings.EqualFold(label, "Total Keseluruhan"):
					if printedNet != "" {
						return nil, errors.New("Total neto Ajaib muncul lebih dari sekali.")
					}
					printedNet = value
				default:
					fees = append(fees, dto.ReviewFee{Label: label, Buy: value})
				}
			}
		}
	}
	if !recognized {
		return nil, errDoclingNoTable
	}
	for _, format := range []string{"2 January 2006", "2006-01-02"} {
		if parsed, err := time.Parse(format, date); err == nil {
			draft.Date = parsed.Format("2006-01-02")
			break
		}
	}
	equalAmounts := func(a, b string) bool { x, e := cents(a); y, f := cents(b); return e == nil && f == nil && x == y }
	if draft.Date == "" || len(draft.Rows) == 0 || side != topSide || !equalAmounts(topNet, printedNet) || !equalAmounts(subtotal, printedGross) {
		return nil, errors.New("Tanggal, sisi, subtotal atau total neto pada tabel Docling belum dapat dicocokkan.")
	}
	draft.Fees = fees
	if side == "SELL" {
		draft.SellGross, draft.SellNet = printedGross, printedNet
		for i := range draft.Fees {
			draft.Fees[i].Sell = draft.Fees[i].Buy
			draft.Fees[i].Buy = ""
		}
	} else {
		draft.BuyGross, draft.BuyNet = printedGross, printedNet
	}
	return draft, nil
}
