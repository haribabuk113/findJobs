package payments

import (
	"strings"
	"testing"
	"time"
)

func TestWriteCSV(t *testing.T) {
	at := time.Date(2026, 11, 2, 10, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))
	refType, refID := "job", "abc-123"
	entries := []Entry{
		{Kind: "job_post", AmountPaise: -5050, BalanceAfterPaise: 4950, ActorName: "=HYPERLINK(\"x\")", RefType: &refType, RefID: &refID, CreatedAt: at},
		{Kind: "topup", AmountPaise: 10_000, BalanceAfterPaise: 10_000, ActorName: "Ravi, Kumar", CreatedAt: at},
	}
	var b strings.Builder
	if err := WriteCSV(&b, entries); err != nil {
		t.Fatal(err)
	}
	want := "date_time,action,performed_by,amount_inr,balance_after_inr,reference\n" +
		"2026-11-02T10:30:00+05:30,job_post,\"'=HYPERLINK(\"\"x\"\")\",-50.50,49.50,job:abc-123\n" +
		"2026-11-02T10:30:00+05:30,topup,\"Ravi, Kumar\",100.00,100.00,\n"
	if b.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}
