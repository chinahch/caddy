package agent

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestDurableReportsSurviveResponseLossAndRestart(t *testing.T) {
	dir := t.TempDir()
	q, e := OpenSpool(dir)
	if e != nil {
		t.Fatal(e)
	}
	r := Report{NodeID: 3, ReportID: "run-1", PeriodStart: time.Now().Add(-time.Minute).UTC(), PeriodEnd: time.Now().UTC(), Entries: []Entry{{UserID: 5, Up: 101, Down: 202}}}
	if e = q.Enqueue(r); e != nil {
		t.Fatal(e)
	}
	first, e := q.Pending()
	if e != nil || len(first) != 1 {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(first[0])
	q, e = OpenSpool(dir)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := q.Pending()
	data, _ := os.ReadFile(after[0])
	if !bytes.Equal(before, data) {
		t.Fatal("retry changed batch identity or payload")
	}
	if e = q.Ack(after[0]); e != nil {
		t.Fatal(e)
	}
	left, _ := q.Pending()
	if len(left) != 0 {
		t.Fatal("acknowledged batch remained")
	}
}

func TestSpoolRejectsReportsOutsideReceiverBounds(t *testing.T) {
	q, _ := OpenSpool(t.TempDir())
	e := q.Enqueue(Report{NodeID: 1, ReportID: "too-large", PeriodStart: time.Now().Add(-time.Minute), PeriodEnd: time.Now(), Entries: []Entry{{UserID: 1, Up: (1 << 50) + 1}}})
	if e == nil {
		t.Fatal("report exceeds receiver limit and would remain stuck forever")
	}
}
