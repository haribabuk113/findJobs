package payments

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
)

// WriteCSV writes ledger entries as CSV, one row per entry in the given order.
// Amounts are INR with two decimals; times are ISO 8601 with their zone.
func WriteCSV(w io.Writer, entries []Entry) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"date_time", "action", "performed_by", "amount_inr", "balance_after_inr", "reference"})
	for _, e := range entries {
		ref := ""
		if e.RefType != nil && e.RefID != nil {
			ref = *e.RefType + ":" + *e.RefID
		}
		cw.Write([]string{
			e.CreatedAt.Format(time.RFC3339),
			safeCell(e.Kind),
			safeCell(e.ActorName),
			inr(e.AmountPaise),
			inr(e.BalanceAfterPaise),
			safeCell(ref),
		})
	}
	cw.Flush()
	return cw.Error()
}

// inr formats paise as rupees: -5050 → "-50.50".
func inr(paise int64) string {
	sign := ""
	if paise < 0 {
		sign, paise = "-", -paise
	}
	return fmt.Sprintf("%s%d.%02d", sign, paise/100, paise%100)
}

// safeCell stops a spreadsheet from running text as a formula (CSV injection):
// a user's name like "=HYPERLINK(...)" is written as "'=HYPERLINK(...)".
func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
