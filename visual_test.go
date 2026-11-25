package main

// Visual UI test: opens the REAL application window and drives it step by
// step with pauses, so the scenario can be watched. Every step is also checked.
//
//	VISUAL=1 go test -run Visual -v .
//	VISUAL=1 VISUAL_DELAY=2s go test -run Visual -v .   # slower
//
// Without VISUAL the test is skipped and the normal headless tests run.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/test"

	"pad-lab2/internal/results"
)

var visualApp fyne.App

func TestMain(m *testing.M) {
	if os.Getenv("VISUAL") == "" {
		os.Exit(m.Run())
	}
	// Real windows need the GUI event loop on the main thread, so the tests
	// run in a goroutine while a.Run() owns the main thread.
	flag.Parse()
	_ = flag.Set("test.run", "^TestVisual")
	visualApp = app.NewWithID("md.pad.lab2.visual")
	visualApp.Settings().SetTheme(appTheme{})
	code := 1
	go func() {
		defer fyne.Do(visualApp.Quit)
		code = m.Run()
	}()
	visualApp.Run()
	os.Exit(code)
}

// stepper shows the current step in the window title and pauses after it.
type stepper struct {
	t     *testing.T
	win   fyne.Window
	n     int
	delay time.Duration
}

func (s *stepper) step(name string, action func(), check func() error) {
	s.t.Helper()
	s.n++
	title := fmt.Sprintf("Шаг %d: %s", s.n, name)
	s.t.Log(title)
	var err error
	fyne.DoAndWait(func() {
		s.win.SetTitle(title)
		if action != nil {
			action()
		}
	})
	time.Sleep(s.delay / 2)
	if check != nil {
		fyne.DoAndWait(func() { err = check() })
	}
	if err != nil {
		fyne.DoAndWait(func() { s.win.SetTitle("ОШИБКА: " + title) })
		time.Sleep(2 * s.delay)
		s.t.Fatalf("%s: %v", title, err)
	}
	time.Sleep(s.delay / 2)
}

func visualDelay() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("VISUAL_DELAY")); err == nil {
		return d
	}
	return 1200 * time.Millisecond
}

func expect(ok bool, format string, args ...any) error {
	if ok {
		return nil
	}
	return fmt.Errorf(format, args...)
}

func TestVisualScenario(t *testing.T) {
	if visualApp == nil {
		t.Skip("set VISUAL=1 to watch the scenario in a real window")
	}
	dir := t.TempDir()
	copyFile(t, "students.xlsx", filepath.Join(dir, "students.xlsx"))
	copyFile(t, "tickets.docx", filepath.Join(dir, "tickets.docx"))
	resultsPath := filepath.Join(dir, "results.xlsx")

	var u *ui
	fyne.DoAndWait(func() {
		u = newUI(visualApp, filepath.Join(dir, "students.xlsx"), filepath.Join(dir, "tickets.docx"), resultsPath)
		u.win.SetMaster() // closing it ends the app
		u.win.Show()
	})
	s := &stepper{t: t, win: u.win, delay: visualDelay()}
	var first string

	s.step("загрузка файлов", u.load, func() error {
		return expect(strings.Join(u.group.Options, ",") == "TI-221,TI-222,TI-223", "groups = %v", u.group.Options)
	})
	s.step("кнопка неактивна без выбора", nil, func() error {
		return expect(u.btn.Disabled(), "button must be disabled")
	})
	s.step("выбор группы TI-221", func() { u.group.SetSelected("TI-221") }, func() error {
		return expect(len(u.student.Options) == 4 && u.btn.Disabled(), "students = %v", u.student.Options)
	})
	s.step("выбор студента", func() { u.student.SetSelected("Иванов Пётр") }, func() error {
		return expect(!u.btn.Disabled(), "button must be enabled")
	})
	s.step("генерация билета", func() { test.Tap(u.btn) }, func() error {
		if u.last == nil {
			return fmt.Errorf("result window not opened")
		}
		first = u.last.number.Text
		return expect(len(u.last.questions) == 3 && u.last.badgeText == "Новый билет",
			"questions=%d badge=%q", len(u.last.questions), u.last.badgeText)
	})
	s.step("ESC закрывает окно билета", func() {
		u.last.win.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	}, func() error {
		return expect(len(visualApp.Driver().AllWindows()) == 1 && u.student.Selected == "", "windows = %d", len(visualApp.Driver().AllWindows()))
	})
	s.step("повторный выбор того же студента", func() {
		u.student.SetSelected("Иванов Пётр")
		test.Tap(u.btn)
	}, func() error {
		return expect(u.last.number.Text == first && strings.HasPrefix(u.last.badgeText, "Повторный"),
			"number %q (first %q), badge %q", u.last.number.Text, first, u.last.badgeText)
	})
	s.step("кнопка «Закрыть»", func() { test.Tap(u.last.closeBtn) }, func() error {
		return expect(len(visualApp.Driver().AllWindows()) == 1, "result window still open")
	})
	s.step("журнал: 2 строки, вторая — повтор", nil, func() error {
		recs, err := results.New(resultsPath).ReadAll()
		if err != nil {
			return err
		}
		return expect(len(recs) == 2 && !recs[0].Repeat && recs[1].Repeat && recs[0].Ticket == recs[1].Ticket, "journal = %+v", recs)
	})
	s.step("пустая группа TI-223", func() { u.group.SetSelected("TI-223") }, func() error {
		return expect(len(u.student.Options) == 0 && u.btn.Disabled(), "empty group must keep the button disabled")
	})
	lock := filepath.Join(dir, "~$results.xlsx")
	s.step("results.xlsx открыт в Excel → диалог «Повторить»", func() {
		os.WriteFile(lock, []byte("x"), 0o644)
		u.group.SetSelected("TI-222")
		u.student.SetSelected("Rusu Elena")
		u.last = nil
		test.Tap(u.btn)
	}, func() error {
		return expect(u.last == nil && u.win.Canvas().Overlays().Top() != nil, "retry dialog expected")
	})
	s.step("Excel закрыт → билет выдан", func() {
		os.Remove(lock)
		u.win.Canvas().Overlays().Top().Hide()
		test.Tap(u.btn)
	}, func() error {
		return expect(u.last != nil, "ticket must be issued after the lock is released")
	})
	s.step("готово — все проверки пройдены", func() {
		u.last.win.Close()
	}, nil)
}
