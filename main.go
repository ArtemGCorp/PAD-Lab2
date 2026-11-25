// Command pad-lab2 is a desktop (Fyne) exam ticket generator.
//
// Inputs:  students.xlsx (one sheet per group), tickets.docx (tickets).
// Output:  results.xlsx (append-only journal).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pad-lab2/internal/assign"
	"pad-lab2/internal/results"
	"pad-lab2/internal/students"
	"pad-lab2/internal/tickets"
)

type ui struct {
	app        fyne.App
	win        fyne.Window
	group      *widget.Select
	student    *widget.Select
	btn        *widget.Button
	reload     *widget.Button
	status     *widget.Label
	statusIcon *widget.Icon
	stats      *widget.Label

	studentsPath string
	ticketsPath  string
	journal      *results.Journal

	roster  *students.Roster
	tickets []tickets.Ticket

	// last is the most recently opened result window (used by tests).
	last *resultView
}

func main() {
	studentsPath := flag.String("students", "students.xlsx", "путь к списку студентов")
	ticketsPath := flag.String("tickets", "tickets.docx", "путь к списку билетов")
	resultsPath := flag.String("results", "results.xlsx", "путь к журналу результатов")
	logPath := flag.String("log", "lab2.log", "файл журнала ошибок")
	flag.Parse()

	if lf, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, lf))
		defer lf.Close()
	}

	a := app.NewWithID("md.pad.lab2")
	a.Settings().SetTheme(appTheme{})
	u := newUI(a, *studentsPath, *ticketsPath, *resultsPath)
	u.win.Show()
	u.load()
	a.Run()
}

func newUI(a fyne.App, studentsPath, ticketsPath, resultsPath string) *ui {
	u := &ui{
		app:          a,
		win:          a.NewWindow("Генератор экзаменационных билетов"),
		studentsPath: studentsPath,
		ticketsPath:  ticketsPath,
		journal:      results.New(resultsPath),
	}
	u.build()
	return u
}

func (u *ui) build() {
	u.student = widget.NewSelect(nil, func(string) { u.refreshButton() })
	u.student.PlaceHolder = "Сначала выберите группу"

	u.group = widget.NewSelect(nil, u.onGroup)
	u.group.PlaceHolder = "Выберите группу"

	u.btn = widget.NewButtonWithIcon("Сгенерировать билет", theme.DocumentCreateIcon(), u.onGenerate)
	u.btn.Importance = widget.HighImportance
	u.btn.Disable()

	u.reload = widget.NewButtonWithIcon("Перечитать файлы", theme.ViewRefreshIcon(), u.load)
	u.reload.Importance = widget.LowImportance

	u.stats = widget.NewLabel("")
	u.stats.TextStyle = fyne.TextStyle{Bold: true}
	u.statusIcon = widget.NewIcon(theme.InfoIcon())
	u.status = widget.NewLabel("")
	u.status.Wrapping = fyne.TextWrapWord

	selection := card(container.NewVBox(
		fieldLabel("Группа"), u.group,
		fieldLabel("Студент"), u.student,
		widget.NewSeparator(),
		u.btn,
	))

	footer := card(container.NewVBox(
		container.NewBorder(nil, nil, u.statusIcon, u.reload, u.stats),
		u.status,
	))

	u.win.SetContent(container.NewBorder(
		header(theme.AccountIcon(), "Выдача экзаменационных билетов",
			"Выберите группу и студента, затем нажмите «Сгенерировать билет»"),
		container.NewPadded(footer),
		nil, nil,
		container.NewPadded(selection),
	))
	u.win.Resize(fyne.NewSize(560, 560))
	u.win.SetMaster()
}

// load (re)reads students.xlsx and tickets.docx. Failures are reported, never fatal.
func (u *ui) load() {
	u.roster, u.tickets = nil, nil
	u.group.ClearSelected()
	u.group.Options = nil
	u.group.Refresh()
	u.onGroup("")

	var problems []string

	if _, err := os.Stat(u.studentsPath); err != nil {
		problems = append(problems, fmt.Sprintf("Не найден файл студентов: %s", u.studentsPath))
	} else if r, err := students.Load(u.studentsPath); err != nil {
		problems = append(problems, err.Error())
	} else {
		u.roster = r
	}

	if _, err := os.Stat(u.ticketsPath); err != nil {
		problems = append(problems, fmt.Sprintf("Не найден файл билетов: %s", u.ticketsPath))
	} else {
		ts, warns, err := tickets.LoadDocx(u.ticketsPath)
		for _, w := range warns {
			log.Printf("tickets.docx: %s", w)
		}
		if err != nil {
			problems = append(problems, err.Error())
		} else {
			u.tickets = ts
		}
		if len(warns) > 0 {
			problems = append(problems, fmt.Sprintf("В tickets.docx пропущено фрагментов: %d (подробности в lab2.log)", len(warns)))
		}
	}

	if u.roster != nil {
		u.group.Options = u.roster.Groups()
		u.group.Refresh()
	}

	u.stats.SetText(fmt.Sprintf("Групп: %d · Билетов: %d", len(u.group.Options), len(u.tickets)))
	if len(problems) > 0 {
		for _, p := range problems {
			log.Print(p)
		}
		u.status.SetText(strings.Join(problems, "\n"))
		u.status.Importance = widget.WarningImportance
		u.statusIcon.SetResource(theme.WarningIcon())
		if u.roster == nil || u.tickets == nil {
			dialog.ShowError(errors.New(strings.Join(problems, "\n")), u.win)
		}
	} else {
		u.status.SetText("Файлы загружены, можно выдавать билеты.")
		u.status.Importance = widget.SuccessImportance
		u.statusIcon.SetResource(theme.ConfirmIcon())
	}
	u.status.Refresh()
	u.refreshButton()
}

func (u *ui) onGroup(g string) {
	u.student.ClearSelected()
	u.student.Options = nil
	switch {
	case g == "":
		u.student.PlaceHolder = "Сначала выберите группу"
	case u.roster != nil && len(u.roster.Students(g)) == 0:
		u.student.PlaceHolder = "В группе нет студентов"
	default:
		u.student.PlaceHolder = "Выберите студента"
	}
	if u.roster != nil && g != "" {
		for _, s := range u.roster.Students(g) {
			u.student.Options = append(u.student.Options, s.Display())
		}
	}
	u.student.Refresh()
	u.refreshButton()
}

func (u *ui) refreshButton() {
	if u.group.Selected != "" && u.student.Selected != "" && len(u.tickets) > 0 {
		u.btn.Enable()
	} else {
		u.btn.Disable()
	}
}

func (u *ui) onGenerate() {
	group := u.group.Selected
	s, ok := u.roster.FindByDisplay(group, u.student.Selected)
	if !ok {
		dialog.ShowError(errors.New("студент не найден в выбранной группе"), u.win)
		return
	}
	u.tryGenerate(group, s)
}

// tryGenerate reads the journal, decides the ticket, appends a row, then shows
// the result. A locked journal offers a retry instead of failing.
func (u *ui) tryGenerate(group string, s students.Student) {
	retry := func(err error) {
		log.Printf("results.xlsx: %v", err)
		dialog.ShowCustomConfirm("Файл занят", "Повторить", "Отмена",
			widget.NewLabel(err.Error()+"\nЗакройте файл и нажмите «Повторить»."),
			func(ok bool) {
				if ok {
					u.tryGenerate(group, s)
				}
			}, u.win)
	}

	history, err := u.journal.ReadAll()
	if errors.Is(err, results.ErrLocked) {
		retry(err)
		return
	}
	if err != nil {
		log.Printf("results.xlsx: %v", err)
		dialog.ShowError(err, u.win)
		return
	}

	d, err := assign.Decide(history, u.tickets, group, s.Last, s.First, assign.RandomPicker)
	if err != nil && !errors.Is(err, assign.ErrUnknownTicket) {
		dialog.ShowError(err, u.win)
		return
	}
	if errors.Is(err, assign.ErrUnknownTicket) {
		log.Printf("студент %s: билет %d из журнала отсутствует в tickets.docx", s.Display(), d.Ticket.Number)
	}

	rec := results.Record{Group: group, Last: s.Last, First: s.First, Ticket: d.Ticket.Number, When: time.Now(), Repeat: d.Repeat}
	if err := u.journal.Append(rec); err != nil {
		if errors.Is(err, results.ErrLocked) {
			retry(err)
			return
		}
		log.Printf("results.xlsx: %v", err)
		dialog.ShowError(err, u.win)
		return
	}
	u.last = u.showResult(group, s, d)
}

func (u *ui) showResult(group string, s students.Student, d assign.Decision) *resultView {
	rv := newResultView(u.app, group, s, d)
	rv.win.SetOnClosed(func() {
		u.student.ClearSelected()
		u.refreshButton()
		u.win.RequestFocus()
	})
	rv.win.CenterOnScreen()
	rv.win.Show()
	rv.win.RequestFocus()
	return rv
}
