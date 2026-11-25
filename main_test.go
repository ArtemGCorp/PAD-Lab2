package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
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

// uiTheme is the theme used by newTestUI.
var uiTheme fyne.Theme = appTheme{}

func newTestUI(t *testing.T, dir string) *ui {
	t.Helper()
	a := test.NewTempApp(t)
	a.Settings().SetTheme(uiTheme)
	u := newUI(a, filepath.Join(dir, "students.xlsx"), filepath.Join(dir, "tickets.docx"), filepath.Join(dir, "results.xlsx"))
	u.win.Show()
	u.load()
	return u
}

func setup(t *testing.T) (*ui, string) {
	t.Helper()
	dir := t.TempDir()
	copyFile(t, "students.xlsx", filepath.Join(dir, "students.xlsx"))
	copyFile(t, "tickets.docx", filepath.Join(dir, "tickets.docx"))
	return newTestUI(t, dir), dir
}

func journalRows(t *testing.T, dir string) []results.Record {
	t.Helper()
	recs, err := results.New(filepath.Join(dir, "results.xlsx")).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func generate(t *testing.T, u *ui, group, student string) *resultView {
	t.Helper()
	u.group.SetSelected(group)
	u.student.SetSelected(student)
	u.last = nil
	test.Tap(u.btn)
	if u.last == nil {
		t.Fatalf("result window not opened for %s / %s", group, student)
	}
	return u.last
}

func pressEsc(w fyne.Window) {
	w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
}

func TestGroupsComeFromSheets(t *testing.T) {
	u, _ := setup(t)
	if got := strings.Join(u.group.Options, ","); got != "TI-221,TI-222,TI-223" {
		t.Fatalf("groups = %s", got)
	}
	if u.stats.Text != "Групп: 3 · Билетов: 10" {
		t.Fatalf("stats = %q", u.stats.Text)
	}
}

func TestStudentsFollowSelectedGroup(t *testing.T) {
	u, _ := setup(t)
	u.group.SetSelected("TI-222")
	if got := strings.Join(u.student.Options, ","); got != "Popescu Ion,Rusu Elena,Ceban Andrei" {
		t.Fatalf("students = %s", got)
	}
	u.student.SetSelected("Rusu Elena")
	u.group.SetSelected("TI-221") // switching group resets the student
	if u.student.Selected != "" || u.student.Options[0] != "Иванов Пётр" {
		t.Fatalf("after switch: selected=%q options=%v", u.student.Selected, u.student.Options)
	}
}

func TestButtonEnabledOnlyWithBothSelections(t *testing.T) {
	u, _ := setup(t)
	if !u.btn.Disabled() {
		t.Fatal("disabled before any selection")
	}
	u.group.SetSelected("TI-221")
	if !u.btn.Disabled() {
		t.Fatal("disabled with group only")
	}
	u.student.SetSelected("Иванов Пётр")
	if u.btn.Disabled() {
		t.Fatal("enabled after both selections")
	}
}

func TestResultWindowShowsTicketAndQuestions(t *testing.T) {
	u, dir := setup(t)
	rv := generate(t, u, "TI-222", "Rusu Elena")

	rec := journalRows(t, dir)
	if len(rec) != 1 || rec[0].Repeat {
		t.Fatalf("journal = %+v", rec)
	}
	n := rec[0].Ticket
	if rv.number.Text != "№ "+strconv.Itoa(n) || rv.win.Title() != "Билет № "+strconv.Itoa(n) {
		t.Fatalf("number=%q title=%q, journal ticket %d", rv.number.Text, rv.win.Title(), n)
	}
	if len(rv.questions) != 3 {
		t.Fatalf("questions = %d", len(rv.questions))
	}
	for _, q := range rv.questions {
		if strings.TrimSpace(q.Text) == "" {
			t.Fatal("empty question text")
		}
	}
	if !strings.Contains(rv.student.Text, "Rusu Elena") || !strings.Contains(rv.student.Text, "TI-222") {
		t.Fatalf("student line = %q", rv.student.Text)
	}
	if rv.badgeText != "Новый билет" {
		t.Fatalf("badge = %q", rv.badgeText)
	}
}

func TestEscClosesOnlyResultWindow(t *testing.T) {
	u, _ := setup(t)
	before := len(u.app.Driver().AllWindows())
	rv := generate(t, u, "TI-221", "Петрова Анна")
	if len(u.app.Driver().AllWindows()) != before+1 {
		t.Fatal("result window must be open")
	}
	pressEsc(rv.win)
	if len(u.app.Driver().AllWindows()) != before {
		t.Fatal("ESC must close only the result window")
	}
	if u.student.Selected != "" || !u.btn.Disabled() {
		t.Fatal("after closing, ready for the next student")
	}
	if u.group.Selected != "TI-221" {
		t.Fatal("group selection is kept")
	}
}

func TestCloseButtonClosesResultWindow(t *testing.T) {
	u, _ := setup(t)
	before := len(u.app.Driver().AllWindows())
	rv := generate(t, u, "TI-221", "Иванов Пётр")
	test.Tap(rv.closeBtn)
	if len(u.app.Driver().AllWindows()) != before {
		t.Fatal("Закрыть must close the result window")
	}
}

func TestRepeatGivesSameTicketAndAppendsRow(t *testing.T) {
	u, dir := setup(t)
	first := generate(t, u, "TI-222", "Rusu Elena")
	pressEsc(first.win)
	second := generate(t, u, "TI-222", "Rusu Elena")

	if second.number.Text != first.number.Text {
		t.Fatalf("repeat ticket %q != first %q", second.number.Text, first.number.Text)
	}
	if !strings.HasPrefix(second.badgeText, "Повторный запрос") {
		t.Fatalf("badge = %q", second.badgeText)
	}
	rec := journalRows(t, dir)
	if len(rec) != 2 || rec[0].Repeat || !rec[1].Repeat || rec[0].Ticket != rec[1].Ticket {
		t.Fatalf("journal = %+v", rec)
	}
}

func TestHistoryIsChronological(t *testing.T) {
	u, dir := setup(t)
	order := [][2]string{{"TI-221", "Иванов Пётр"}, {"TI-222", "Ceban Andrei"}, {"TI-221", "Иванов Пётр"}}
	for _, o := range order {
		pressEsc(generate(t, u, o[0], o[1]).win)
	}
	rec := journalRows(t, dir)
	if len(rec) != 3 || rec[0].Last != "Иванов" || rec[1].Last != "Ceban" || rec[2].Last != "Иванов" || !rec[2].Repeat {
		t.Fatalf("journal = %+v", rec)
	}
	for i := 1; i < len(rec); i++ {
		if rec[i].When.Before(rec[i-1].When) {
			t.Fatal("rows must be in chronological order")
		}
	}
}

func TestEmptyGroupKeepsButtonDisabled(t *testing.T) {
	u, _ := setup(t)
	u.group.SetSelected("TI-223")
	if len(u.student.Options) != 0 || !u.btn.Disabled() {
		t.Fatal("empty group: no students, button disabled")
	}
	if u.student.PlaceHolder != "В группе нет студентов" {
		t.Fatalf("placeholder = %q", u.student.PlaceHolder)
	}
}

func TestStatusShowsSkippedTickets(t *testing.T) {
	u, _ := setup(t)
	if !strings.Contains(u.status.Text, "пропущено фрагментов: 5") || u.status.Importance != widget.WarningImportance {
		t.Fatalf("status = %q (%v)", u.status.Text, u.status.Importance)
	}
}

func TestMissingFilesThenReload(t *testing.T) {
	dir := t.TempDir()
	u := newTestUI(t, dir)
	if len(u.group.Options) != 0 || !u.btn.Disabled() {
		t.Fatal("no data expected")
	}
	if !strings.Contains(u.status.Text, "Не найден файл студентов") || !strings.Contains(u.status.Text, "Не найден файл билетов") {
		t.Fatalf("status = %q", u.status.Text)
	}

	// The user copies the files in and presses «Перечитать файлы».
	copyFile(t, "students.xlsx", filepath.Join(dir, "students.xlsx"))
	copyFile(t, "tickets.docx", filepath.Join(dir, "tickets.docx"))
	test.Tap(u.reload)
	if len(u.group.Options) != 3 || u.stats.Text != "Групп: 3 · Билетов: 10" {
		t.Fatalf("after reload: groups=%v stats=%q", u.group.Options, u.stats.Text)
	}
}

func TestLockedJournalDoesNotWrite(t *testing.T) {
	u, dir := setup(t)
	lock := filepath.Join(dir, "~$results.xlsx")
	os.WriteFile(lock, []byte("x"), 0o644)
	u.group.SetSelected("TI-221")
	u.student.SetSelected("Иванов Пётр")
	before := len(u.app.Driver().AllWindows())
	test.Tap(u.btn) // shows the retry dialog inside the main window
	if len(u.app.Driver().AllWindows()) != before || u.last != nil {
		t.Fatal("result window must not open while the journal is locked")
	}
	if _, err := os.Stat(filepath.Join(dir, "results.xlsx")); !os.IsNotExist(err) {
		t.Fatal("nothing must be written while locked")
	}

	// After Excel is closed, generation works again.
	os.Remove(lock)
	test.Tap(u.btn)
	if u.last == nil || len(journalRows(t, dir)) != 1 {
		t.Fatal("generation must work after the lock is released")
	}
}

// TestScreenshots renders both windows to PNG for a visual check:
//
//	LAB2_SCREENSHOTS=/some/dir go test -run Screenshots .
func TestScreenshots(t *testing.T) {
	out := os.Getenv("LAB2_SCREENSHOTS")
	if out == "" {
		t.Skip("set LAB2_SCREENSHOTS=<dir>")
	}
	for _, v := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
		t.Run(v.name, func(t *testing.T) { screenshots(t, out, v.name, v.v) })
	}
}

func screenshots(t *testing.T, out, name string, v fyne.ThemeVariant) {
	uiTheme = fixedVariant{appTheme{}, v}
	defer func() { uiTheme = appTheme{} }()
	u, _ := setup(t)
	u.win.Resize(fyne.NewSize(560, 560))
	save(t, u.win, filepath.Join(out, "lab2-main-"+name+".png"))
	rv := generate(t, u, "TI-221", "Иванов Пётр")
	save(t, rv.win, filepath.Join(out, "lab2-ticket-"+name+".png"))
	pressEsc(rv.win)
	rv = generate(t, u, "TI-221", "Иванов Пётр")
	save(t, rv.win, filepath.Join(out, "lab2-ticket-repeat-"+name+".png"))
}

func save(t *testing.T, w fyne.Window, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
