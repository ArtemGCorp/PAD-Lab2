// Package assign decides which ticket a student gets.
//
// Rule: if the student (group + last + first) already has a record in the
// journal, the ticket from the FIRST record is reused and the new row is
// marked as a repeat. Otherwise a random valid ticket is drawn.
package assign

import (
	"errors"
	"math/rand/v2"

	"pad-lab2/internal/results"
	"pad-lab2/internal/tickets"
)

// ErrNoTickets — nothing to draw from.
var ErrNoTickets = errors.New("нет доступных билетов")

// ErrUnknownTicket — the journal points at a ticket number that is no longer
// in tickets.docx (the number is still reused; questions cannot be shown).
var ErrUnknownTicket = errors.New("билет из журнала отсутствует в tickets.docx")

// Decision is the outcome for one generation.
type Decision struct {
	Ticket tickets.Ticket
	Repeat bool
}

// Picker returns an index in [0, n).
type Picker func(n int) int

// RandomPicker uses math/rand/v2.
func RandomPicker(n int) int { return rand.IntN(n) }

// Decide applies the assignment rule. For a repeat whose ticket is missing
// from ts it returns the decision (Ticket.Number set, no questions) together
// with ErrUnknownTicket.
func Decide(history []results.Record, ts []tickets.Ticket, group, last, first string, pick Picker) (Decision, error) {
	if n, ok := results.FirstTicket(history, group, last, first); ok {
		if t, found := tickets.Find(ts, n); found {
			return Decision{Ticket: t, Repeat: true}, nil
		}
		return Decision{Ticket: tickets.Ticket{Number: n}, Repeat: true}, ErrUnknownTicket
	}
	if len(ts) == 0 {
		return Decision{}, ErrNoTickets
	}
	return Decision{Ticket: ts[pick(len(ts))], Repeat: false}, nil
}
