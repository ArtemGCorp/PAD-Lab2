package assign

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pad-lab2/internal/results"
	"pad-lab2/internal/tickets"
)

var ts = []tickets.Ticket{
	{Number: 1, Questions: []string{"a", "b", "c"}},
	{Number: 2, Questions: []string{"d", "e", "f"}},
	{Number: 5, Questions: []string{"g", "h", "i"}},
}

func TestNewStudentGetsRandomTicket(t *testing.T) {
	d, err := Decide(nil, ts, "G", "L", "F", func(n int) int { return n - 1 })
	if err != nil || d.Repeat || d.Ticket.Number != 5 {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestRandomPickerInRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		if v := RandomPicker(3); v < 0 || v >= 3 {
			t.Fatalf("out of range: %d", v)
		}
	}
}

// Idempotency: repeated generation for the same student through the real
// journal always yields the first ticket and appends a "да" row.
func TestRepeatReturnsSameTicketAndAppendsRow(t *testing.T) {
	j := results.New(filepath.Join(t.TempDir(), "results.xlsx"))
	draws := 0
	pick := func(n int) int { draws++; return draws % n } // different every call

	var first int
	for i := 0; i < 4; i++ {
		hist, err := j.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		d, err := Decide(hist, ts, "G1", "Иванов", "Пётр", pick)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = d.Ticket.Number
			if d.Repeat {
				t.Fatal("first generation must not be a repeat")
			}
		} else if d.Ticket.Number != first || !d.Repeat {
			t.Fatalf("call %d: %+v, want ticket %d repeat", i, d, first)
		}
		if err := j.Append(results.Record{Group: "G1", Last: "Иванов", First: "Пётр",
			Ticket: d.Ticket.Number, When: time.Now(), Repeat: d.Repeat}); err != nil {
			t.Fatal(err)
		}
	}
	if draws != 1 {
		t.Fatalf("random picker called %d times, want 1", draws)
	}
	recs, _ := j.ReadAll()
	if len(recs) != 4 {
		t.Fatalf("journal rows = %d, want 4 (history is append-only)", len(recs))
	}
	for i, r := range recs {
		if r.Ticket != first || r.Repeat != (i > 0) {
			t.Fatalf("row %d = %+v", i, r)
		}
	}
}

func TestSameNameInOtherGroupIsNotRepeat(t *testing.T) {
	hist := []results.Record{{Group: "G1", Last: "L", First: "F", Ticket: 2}}
	d, err := Decide(hist, ts, "G2", "L", "F", func(int) int { return 0 })
	if err != nil || d.Repeat || d.Ticket.Number != 1 {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestRepeatOfRemovedTicket(t *testing.T) {
	hist := []results.Record{{Group: "G", Last: "L", First: "F", Ticket: 42}}
	d, err := Decide(hist, ts, "G", "L", "F", RandomPicker)
	if !errors.Is(err, ErrUnknownTicket) || d.Ticket.Number != 42 || !d.Repeat {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestNoTickets(t *testing.T) {
	if _, err := Decide(nil, nil, "G", "L", "F", RandomPicker); !errors.Is(err, ErrNoTickets) {
		t.Fatalf("err = %v", err)
	}
}
