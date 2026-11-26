package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var panelHeader = regexp.MustCompile(`(?im)^([ \t]*Tipe[ \t]+Efek[ \t]+Harga[ \t]+Jumlah[ \t]+Total Beli[ \t]+)(Tipe[ \t]+Efek[ \t]+Harga[ \t]+Jumlah[ \t]+Total Jual)[ \t]*\r?$`)
var panelDate = regexp.MustCompile(`(?im)\b(?:Tgl\.|Tanggal) Transaksi[ \t]*:[ \t]*(\d{1,2} [A-Za-z]+ \d{4})[ \t]*\r?$`)
var panelNet = regexp.MustCompile(`(?im)\b(Total Terima|Total Bayar)[ \t]*:[ \t]*(?:Rp\.?[ \t]*)?([\d.]+)[ \t]*\r?$`)
var panelSummary = regexp.MustCompile(`^([^:]+?)\s*:\s*(\(?[\d.]+\)?)$`)

// Flatten one populated panel only when its printed subtotal, gross and net agree.
func flattenAjaibPanels(layout string) (string, error) {
	matches := panelHeader.FindAllStringSubmatchIndex(layout, -1)
	if len(matches) == 0 {
		return layout, nil
	}
	if len(matches) != 1 {
		return "", errors.New("Ada beberapa header tabel dua sisi Ajaib.")
	}
	h := matches[0]
	dates := panelDate.FindAllStringSubmatch(layout[:h[0]], -1)
	nets := panelNet.FindAllStringSubmatch(layout[:h[0]], -1)
	if len(dates) != 1 || len(nets) != 1 {
		return "", errors.New("Tanggal transaksi atau total bayar/terima di atas tabel Ajaib belum terbaca unik.")
	}
	sell := strings.EqualFold(nets[0][1], "Total Terima")
	side, subtotal := "BUY", "Total Beli"
	if sell {
		side, subtotal = "SELL", "Total Jual"
	}
	split := h[4] - h[0]
	var rows, fees []string
	var panelGross, totalGross, totalNet string
	finished := false
	for _, line := range strings.Split(layout[h[1]:], "\n") {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}
		if finished {
			if strings.EqualFold(raw, "Catatan") {
				break
			}
			return "", errors.New("Ada baris tambahan setelah total keseluruhan Ajaib.")
		}
		if m := panelSummary.FindStringSubmatch(raw); m != nil {
			label := strings.TrimSpace(m[1])
			value := m[2]
			switch {
			case strings.EqualFold(label, subtotal) && panelGross == "" && totalGross == "":
				panelGross = value
			case strings.EqualFold(label, "Total Transaksi") && panelGross != "" && totalGross == "":
				totalGross = value
			case feeLabel.MatchString(label) && totalGross != "":
				fees = append(fees, label+" | "+value)
			case strings.EqualFold(label, "Total Keseluruhan") && totalGross != "":
				totalNet = value
				finished = true
			default:
				return "", errors.New("Subtotal, biaya, atau urutan ringkasan Ajaib belum dikenali.")
			}
			continue
		}
		if panelGross != "" {
			return "", errors.New("Baris di bawah subtotal Ajaib belum dikenali.")
		}
		cut := min(split, len(line))
		left, right := strings.TrimSpace(line[:cut]), strings.TrimSpace(line[cut:])
		if (sell && left != "") || (!sell && right != "") {
			return "", errors.New("Kedua sisi tabel Ajaib terisi atau batas kolom bergeser.")
		}
		fields := strings.Fields(raw)
		if len(fields) != 5 {
			return "", fmt.Errorf("Baris transaksi Ajaib memiliki %d sel; format dua sisi memerlukan 5.", len(fields))
		}
		rows = append(rows, strings.Join(fields, " | "))
	}
	eq := func(a, b string) bool { x, e := cents(a); y, f := cents(b); return e == nil && f == nil && x == y }
	if !finished || len(rows) == 0 || !eq(panelGross, totalGross) || !eq(nets[0][2], totalNet) {
		return "", errors.New("Subtotal atau total neto pada kedua bagian TC Ajaib belum cocok.")
	}
	out := []string{dates[0][1], side, "Tipe | Efek | Harga | Jumlah | " + subtotal}
	out = append(out, rows...)
	out = append(out, subtotal+" | "+panelGross)
	out = append(out, fees...)
	out = append(out, nets[0][1]+" | "+totalNet)
	return strings.Join(out, "\n"), nil
}
