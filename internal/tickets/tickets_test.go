package tickets

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p(texts ...string) []Paragraph {
	out := make([]Paragraph, len(texts))
	for i, t := range texts {
		out[i] = Paragraph{Text: t}
	}
	return out
}

func TestParseValidTickets(t *testing.T) {
	ts, warns := Parse(p(
		"Билет 1", "1. Вопрос А", "2. Вопрос Б", "3. Вопрос В",
		"Билет 2", "1) Q1", "2) Q2", "3) Q3",
	))
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if len(ts) != 2 || ts[0].Number != 1 || ts[1].Number != 2 {
		t.Fatalf("got %+v", ts)
	}
	if got := strings.Join(ts[0].Questions, "|"); got != "Вопрос А|Вопрос Б|Вопрос В" {
		t.Fatalf("questions = %q", got)
	}
}

func TestParseSkipsTicketWithTooFewQuestions(t *testing.T) {
	ts, warns := Parse(p(
		"Билет 1", "1. a", "2. b",
		"Билет 2", "1. a", "2. b", "3. c",
	))
	if len(ts) != 1 || ts[0].Number != 2 {
		t.Fatalf("got %+v", ts)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "Билет 1") {
		t.Fatalf("warnings = %v", warns)
	}
}

func TestParseSkipsUnrecognisedHeader(t *testing.T) {
	ts, warns := Parse(p(
		"Билет один", "1. a", "2. b", "3. c",
		"Билет 3", "1. a", "2. b", "3. c",
	))
	if len(ts) != 1 || ts[0].Number != 3 {
		t.Fatalf("got %+v", ts)
	}
	if len(warns) == 0 {
		t.Fatal("expected warnings for the broken header")
	}
}

// A broken header right after a short ticket must not donate its questions to it.
func TestParseBrokenHeaderAfterShortTicket(t *testing.T) {
	ts, warns := Parse(p(
		"Билет 11", "1. one",
		"Билет двенадцать", "1. a", "2. b", "3. c",
	))
	if len(ts) != 0 {
		t.Fatalf("got %+v, want no valid tickets", ts)
	}
	if len(warns) < 2 {
		t.Fatalf("warnings = %v", warns)
	}
}

func TestParseNumberedListParagraphs(t *testing.T) {
	ts, _ := Parse([]Paragraph{
		{Text: "Билет № 5"},
		{Text: "Первый", Numbered: true},
		{Text: "Второй", Numbered: true},
		{Text: "Третий", Numbered: true},
	})
	if len(ts) != 1 || ts[0].Number != 5 || len(ts[0].Questions) != 3 {
		t.Fatalf("got %+v", ts)
	}
}

func TestParseDuplicateNumberKeepsFirst(t *testing.T) {
	ts, warns := Parse(p(
		"Билет 1", "1. a", "2. b", "3. c",
		"Билет 1", "1. x", "2. y", "3. z",
	))
	if len(ts) != 1 || ts[0].Questions[0] != "a" || len(warns) != 1 {
		t.Fatalf("got %+v / %v", ts, warns)
	}
}

func writeDocx(t *testing.T, documentXML string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tickets.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("word/document.xml")
	w.Write([]byte(documentXML))
	zw.Close()
	f.Close()
	return path
}

func TestLoadDocx(t *testing.T) {
	xml := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Билет</w:t></w:r><w:r><w:t xml:space="preserve"> 7</w:t></w:r></w:p>
<w:p><w:r><w:t>1. Что такое RPC?</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Теорема CAP</w:t></w:r></w:p>
<w:p><w:r><w:t>3. Raft</w:t></w:r></w:p>
</w:body></w:document>`
	ts, warns, err := LoadDocx(writeDocx(t, xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if len(ts) != 1 || ts[0].Number != 7 || ts[0].Questions[1] != "Теорема CAP" {
		t.Fatalf("got %+v", ts)
	}
}

func TestLoadDocxNoValidTickets(t *testing.T) {
	xml := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Билет 1</w:t></w:r></w:p></w:body></w:document>`
	_, _, err := LoadDocx(writeDocx(t, xml))
	if !errors.Is(err, ErrNoTickets) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadDocxMissingFile(t *testing.T) {
	if _, _, err := LoadDocx(filepath.Join(t.TempDir(), "nope.docx")); err == nil {
		t.Fatal("expected error")
	}
}
