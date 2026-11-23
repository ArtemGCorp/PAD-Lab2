package results

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestAppendCreatesFileAndKeepsOldRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.xlsx")
	j := New(path)
	t0 := time.Date(2026, 1, 10, 9, 0, 0, 0, time.Local)

	if err := j.Append(Record{"G1", "Иванов", "Пётр", 4, t0, false}); err != nil {
		t.Fatal(err)
	}
	if err := j.Append(Record{"G1", "Иванов", "Пётр", 4, t0.Add(time.Minute), true}); err != nil {
		t.Fatal(err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, _ := f.GetRows(f.GetSheetName(0))
	if len(rows) != 3 {
		t.Fatalf("rows = %v", rows)
	}
	for i, h := range Headers {
		if rows[0][i] != h {
			t.Fatalf("header = %v", rows[0])
		}
	}
	if rows[1][5] != "нет" || rows[2][5] != "да" || rows[1][4] != "2026-01-10 09:00:00" {
		t.Fatalf("rows = %v", rows)
	}

	recs, err := j.ReadAll()
	if err != nil || len(recs) != 2 || !recs[1].Repeat || recs[0].Ticket != 4 {
		t.Fatalf("recs = %+v err = %v", recs, err)
	}
}

func TestReadAllMissingFile(t *testing.T) {
	recs, err := New(filepath.Join(t.TempDir(), "x.xlsx")).ReadAll()
	if err != nil || recs != nil {
		t.Fatalf("%v %v", recs, err)
	}
}

func TestAppendLockedByOffice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.xlsx")
	os.WriteFile(filepath.Join(dir, "~$results.xlsx"), []byte("x"), 0o644)
	err := New(path).Append(Record{Group: "G", Last: "L", First: "F", Ticket: 1, When: time.Now()})
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file must not be created while locked")
	}
}

func TestFirstTicketUsesEarliestRecord(t *testing.T) {
	recs := []Record{
		{Group: "G1", Last: "A", First: "B", Ticket: 3},
		{Group: "G2", Last: "A", First: "B", Ticket: 9},
		{Group: "G1", Last: "a", First: "b", Ticket: 5}, // never happens via app, but first wins
	}
	if n, ok := FirstTicket(recs, "G1", "A", "B"); !ok || n != 3 {
		t.Fatalf("got %d %v", n, ok)
	}
	if n, ok := FirstTicket(recs, "G2", "A", "B"); !ok || n != 9 {
		t.Fatalf("got %d %v", n, ok)
	}
	if _, ok := FirstTicket(recs, "G3", "A", "B"); ok {
		t.Fatal("unexpected match")
	}
}
