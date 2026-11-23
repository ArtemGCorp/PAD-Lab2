// Package tickets parses tickets.docx into exam tickets.
//
// Expected document layout (one paragraph per line):
//
//	Билет 1
//	1. Вопрос первый
//	2. Вопрос второй
//	3. Вопрос третий
//	Билет 2
//	...
//
// Questions may be typed as "N. text" or be Word auto-numbered list items
// (the number is then not part of the paragraph text). A ticket with fewer
// than 3 questions, or text that does not belong to a recognised header, is
// skipped and reported as a parse warning — the remaining tickets are kept.
package tickets

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MinQuestions is the number of questions a valid ticket must have.
const MinQuestions = 3

// Ticket is one exam ticket.
type Ticket struct {
	Number    int
	Questions []string
}

// Paragraph is one Word paragraph: its plain text and whether Word numbers it
// as a list item.
type Paragraph struct {
	Text     string
	Numbered bool
}

var (
	headerRe   = regexp.MustCompile(`^(?i)билет\s*№?\s*(\d+)\s*[.:]?\s*$`)
	questionRe = regexp.MustCompile(`^(\d+)\s*[.)]\s*(.+)$`)
	// Not `\b`: RE2 word boundaries are ASCII-only and never match after Cyrillic.
	looseHdrRe = regexp.MustCompile(`^(?i)билет(\s|№|$)`)
)

// ErrNoTickets is returned when the document holds no valid ticket at all.
var ErrNoTickets = errors.New("в tickets.docx не найдено ни одного корректного билета")

// LoadDocx opens a .docx file and parses its tickets.
// warnings lists every skipped ticket / unrecognised fragment.
func LoadDocx(path string) (tickets []Ticket, warnings []string, err error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось открыть %s: %w", path, err)
	}
	defer zr.Close()

	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return nil, nil, fmt.Errorf("%s не является документом Word (нет word/document.xml)", path)
	}
	rc, err := doc.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось прочитать %s: %w", path, err)
	}
	defer rc.Close()

	paras, err := ReadParagraphs(rc)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось разобрать %s: %w", path, err)
	}
	tickets, warnings = Parse(paras)
	if len(tickets) == 0 {
		return nil, warnings, ErrNoTickets
	}
	return tickets, warnings, nil
}

// ReadParagraphs extracts paragraphs from a word/document.xml stream.
func ReadParagraphs(r io.Reader) ([]Paragraph, error) {
	dec := xml.NewDecoder(r)
	var (
		paras []Paragraph
		cur   *Paragraph
		sb    strings.Builder
		inT   bool
	)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				cur = &Paragraph{}
				sb.Reset()
			case "numPr":
				if cur != nil {
					cur.Numbered = true
				}
			case "t":
				inT = true
			case "tab":
				if cur != nil {
					sb.WriteByte(' ')
				}
			case "br":
				if cur != nil {
					sb.WriteByte('\n')
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if cur != nil {
					cur.Text = sb.String()
					paras = append(paras, *cur)
					cur = nil
				}
			case "t":
				inT = false
			}
		case xml.CharData:
			if inT && cur != nil {
				sb.Write(t)
			}
		}
	}
	// Soft line breaks inside one paragraph are treated as separate lines.
	var out []Paragraph
	for _, p := range paras {
		lines := strings.Split(p.Text, "\n")
		for i, l := range lines {
			out = append(out, Paragraph{Text: l, Numbered: p.Numbered && i == 0})
		}
	}
	return out, nil
}

// Parse groups paragraphs into tickets. Invalid tickets are skipped and
// described in warnings. Duplicate ticket numbers keep the first occurrence.
func Parse(paras []Paragraph) (tickets []Ticket, warnings []string) {
	var cur *Ticket
	seen := map[int]bool{}

	flush := func() {
		if cur == nil {
			return
		}
		switch {
		case len(cur.Questions) < MinQuestions:
			warnings = append(warnings, fmt.Sprintf(
				"Билет %d пропущен: найдено вопросов %d, нужно минимум %d",
				cur.Number, len(cur.Questions), MinQuestions))
		case seen[cur.Number]:
			warnings = append(warnings, fmt.Sprintf(
				"Билет %d пропущен: номер уже встречался выше", cur.Number))
		default:
			seen[cur.Number] = true
			tickets = append(tickets, *cur)
		}
		cur = nil
	}

	for _, p := range paras {
		text := strings.TrimSpace(strings.ReplaceAll(p.Text, " ", " "))
		if text == "" {
			continue
		}
		if m := headerRe.FindStringSubmatch(text); m != nil {
			flush()
			n, _ := strconv.Atoi(m[1])
			cur = &Ticket{Number: n}
			continue
		}
		if looseHdrRe.MatchString(text) {
			// Looks like a header but is malformed ("Билет", "Билет X"): drop
			// the previous ticket's context so its questions are not merged.
			flush()
			warnings = append(warnings, fmt.Sprintf("Не распознан заголовок билета: %q — блок пропущен", text))
			continue
		}
		if cur == nil {
			warnings = append(warnings, fmt.Sprintf("Текст вне билета пропущен: %q", text))
			continue
		}
		if m := questionRe.FindStringSubmatch(text); m != nil {
			cur.Questions = append(cur.Questions, strings.TrimSpace(m[2]))
			continue
		}
		if p.Numbered {
			cur.Questions = append(cur.Questions, text)
			continue
		}
		warnings = append(warnings, fmt.Sprintf("Билет %d: строка не похожа на вопрос и пропущена: %q", cur.Number, text))
	}
	flush()

	sort.SliceStable(tickets, func(i, j int) bool { return tickets[i].Number < tickets[j].Number })
	return tickets, warnings
}

// Find returns the ticket with the given number.
func Find(ts []Ticket, number int) (Ticket, bool) {
	for _, t := range ts {
		if t.Number == number {
			return t, true
		}
	}
	return Ticket{}, false
}
