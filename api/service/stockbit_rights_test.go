package service

import (
	"os"
	"os/exec"
	"testing"
)

func TestStockbitRightExercise(t *testing.T) {
	header := "Stockbit Sekuritas Digital\nStatement of Account\n"
	credit := "23/10/2026 23/10/2026 R 0001 Exercise Right AAAA-R 0 62,500 (62,500)\n"
	debit := "23/10/2026 23/10/2026 U 0002 Exercise Right AAAA-R 62,500 0 0\n"
	if _, err := parseRightExercises(header + credit); err == nil {
		t.Fatal("accepted unmatched credit")
	}
	rows, err := parseRightExercises(header + credit + debit + credit)
	if err != nil || len(rows) != 1 || rows[0].Amount != "62500" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	for env, n := range map[string]int{"STOCKBIT_CASH_SOA": 2, "STOCKBIT_POSITION_SOA": 0} {
		if path := os.Getenv(env); path != "" {
			layout, err := exec.Command("pdftotext", "-layout", path, "-").Output()
			if err != nil {
				t.Fatal(err)
			}
			rows, err := parseRightExercises(string(layout))
			if err != nil || len(rows) != n {
				t.Fatalf("%s rows=%d err=%v", env, len(rows), err)
			}
			if n == 2 && (rows[0].Amount != "62500" || rows[1].Amount != "75000" || rows[0].Ticker != "BUVA" || rows[1].Ticker != "BUVA") {
				t.Fatal("incorrect SOA amounts or ticker")
			}
		}
	}
}
