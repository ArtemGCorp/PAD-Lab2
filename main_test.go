package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"pad-lab2/internal/results"
)

// copyFile copies the committed sample inputs into a temp dir.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (*ui, string) {
	t.Helper()
	dir := t.TempDir()
	copyFile(t, "students.xlsx", filepath.Join(dir, "students.xlsx"))
	copyFile(t, "tickets.docx", filepath.Join(dir, "tickets.docx"))
	a := test.NewTempApp(t)
	u := newUI(a, filepath.Join(dir, "students.xlsx"), filepath.Join(dir, "tickets.docx"), filepath.Join(dir, "results.xlsx"))
	u.win.Show()
	u.load()
	return u, dir
}

func labels(w fyne.Window) string {
	var sb strings.Builder
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Label:
			sb.WriteString(v.Text + "\n")
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		}
	}
	walk(w.Content())
	return sb.String()
}

func TestUIFlow(t *testing.T) {
	u, dir := setup(t)

	if got := strings.Join(u.group.Options, ","); got != "TI-221,TI-222,TI-223" {
		t.Fatalf("groups = %s", got)
	}
	if !u.btn.Disabled() {
		t.Fatal("button must be disabled before any selection")
	}

	u.group.SetSelected("TI-222")
	if got := strings.Join(u.student.Options, ","); got != "Popescu Ion,Rusu Elena,Ceban Andrei" {
		t.Fatalf("students = %s", got)
	}
	if !u.btn.Disabled() {
		t.Fatal("button must be disabled with group only")
	}

	u.student.SetSelected("Rusu Elena")
	if u.btn.Disabled() {
		t.Fatal("button must be enabled after both selections")
	}

	before := len(u.app.Driver().AllWindows())
	test.Tap(u.btn)
	wins := u.app.Driver().AllWindows()
	if len(wins) != before+1 {
		t.Fatalf("result window not opened")
	}
	res := wins[len(wins)-1]
	text := labels(res)
	if !strings.Contains(text, "Ваш билет №") || strings.Count(text, "\n1. ")+strings.Count(text, "\n2. ")+strings.Count(text, "\n3. ") != 3 {
		t.Fatalf("result window text:\n%s", text)
	}

	// Saved immediately.
	recs, err := results.New(filepath.Join(dir, "results.xlsx")).ReadAll()
	if err != nil || len(recs) != 1 || recs[0].Repeat {
		t.Fatalf("recs = %+v err = %v", recs, err)
	}
	first := recs[0].Ticket

	// ESC closes the result window, not the app.
	res.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if len(u.app.Driver().AllWindows()) != before {
		t.Fatal("ESC must close only the result window")
	}

	// Same student again -> same ticket, new row marked "да".
	u.student.SetSelected("Rusu Elena")
	test.Tap(u.btn)
	recs, _ = results.New(filepath.Join(dir, "results.xlsx")).ReadAll()
	if len(recs) != 2 || recs[1].Ticket != first || !recs[1].Repeat || recs[0].Repeat {
		t.Fatalf("recs = %+v", recs)
	}
}

func TestEmptyGroupKeepsButtonDisabled(t *testing.T) {
	u, _ := setup(t)
	u.group.SetSelected("TI-223")
	if len(u.student.Options) != 0 || !u.btn.Disabled() {
		t.Fatal("empty group: no students, button disabled")
	}
}

func TestMissingFilesDoNotCrash(t *testing.T) {
	dir := t.TempDir()
	a := test.NewTempApp(t)
	u := newUI(a, filepath.Join(dir, "students.xlsx"), filepath.Join(dir, "tickets.docx"), filepath.Join(dir, "results.xlsx"))
	u.win.Show()
	u.load()
	if len(u.group.Options) != 0 || !u.btn.Disabled() {
		t.Fatal("no data expected")
	}
	if !strings.Contains(u.status.Text, "Не найден файл студентов") || !strings.Contains(u.status.Text, "Не найден файл билетов") {
		t.Fatalf("status = %q", u.status.Text)
	}
}

func TestLockedJournalDoesNotWrite(t *testing.T) {
	u, dir := setup(t)
	os.WriteFile(filepath.Join(dir, "~$results.xlsx"), []byte("x"), 0o644)
	u.group.SetSelected("TI-221")
	u.student.SetSelected("Иванов Пётр")
	before := len(u.app.Driver().AllWindows())
	test.Tap(u.btn) // shows the retry dialog inside the main window
	if len(u.app.Driver().AllWindows()) != before {
		t.Fatal("result window must not open while the journal is locked")
	}
	if _, err := os.Stat(filepath.Join(dir, "results.xlsx")); !os.IsNotExist(err) {
		t.Fatal("nothing must be written while locked")
	}
}
