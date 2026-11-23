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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"pad-lab2/internal/assign"
	"pad-lab2/internal/results"
	"pad-lab2/internal/students"
	"pad-lab2/internal/tickets"
)

type ui struct {
	app     fyne.App
	win     fyne.Window
	group   *widget.Select
	student *widget.Select
	btn     *widget.Button
	status  *widget.Label

	studentsPath string
	ticketsPath  string
	journal      *results.Journal

	roster  *students.Roster
	tickets []tickets.Ticket
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
	u.status = widget.NewLabel("")
	u.status.Wrapping = fyne.TextWrapWord

	u.student = widget.NewSelect(nil, func(string) { u.refreshButton() })
	u.student.PlaceHolder = "Выберите студента"

	u.group = widget.NewSelect(nil, u.onGroup)
	u.group.PlaceHolder = "Выберите группу"

	u.btn = widget.NewButton("Сгенерировать билет", u.onGenerate)
	u.btn.Importance = widget.HighImportance
	u.btn.Disable()

	reload := widget.NewButton("Перечитать файлы", u.load)

	form := widget.NewForm(
		widget.NewFormItem("Группа", u.group),
		widget.NewFormItem("Студент", u.student),
	)
	u.win.SetContent(container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("Экзамен: выдача билетов", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
		u.btn,
		layout.NewSpacer(),
		container.NewHBox(reload),
		u.status,
	)))
	u.win.Resize(fyne.NewSize(520, 320))
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

	info := fmt.Sprintf("Групп: %d · Билетов: %d", len(u.group.Options), len(u.tickets))
	if len(problems) > 0 {
		for _, p := range problems {
			log.Print(p)
		}
		u.status.SetText(info + "\n" + strings.Join(problems, "\n"))
		if u.roster == nil || u.tickets == nil {
			dialog.ShowError(errors.New(strings.Join(problems, "\n")), u.win)
		}
	} else {
		u.status.SetText(info)
	}
	u.refreshButton()
}

func (u *ui) onGroup(g string) {
	u.student.ClearSelected()
	u.student.Options = nil
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
	u.showResult(s, d)
}

func (u *ui) showResult(s students.Student, d assign.Decision) {
	w := u.app.NewWindow(fmt.Sprintf("Билет № %d", d.Ticket.Number))

	title := widget.NewLabelWithStyle(fmt.Sprintf("Ваш билет № %d", d.Ticket.Number),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	box := container.NewVBox(
		widget.NewLabelWithStyle(s.Display(), fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
		title,
		widget.NewSeparator(),
	)
	if len(d.Ticket.Questions) == 0 {
		box.Add(widget.NewLabel("Вопросы этого билета не найдены в tickets.docx."))
	}
	for i, q := range d.Ticket.Questions {
		l := widget.NewLabel(fmt.Sprintf("%d. %s", i+1, q))
		l.Wrapping = fyne.TextWrapWord
		box.Add(l)
	}
	if d.Repeat {
		box.Add(widget.NewLabelWithStyle("Повторный запрос: выдан тот же билет, что и в первый раз.",
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}
	box.Add(layout.NewSpacer())
	box.Add(widget.NewLabelWithStyle("Нажмите ESC, чтобы закрыть", fyne.TextAlignCenter, fyne.TextStyle{}))

	w.SetContent(container.NewPadded(box))
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	w.SetOnClosed(func() {
		u.student.ClearSelected()
		u.refreshButton()
		u.win.RequestFocus()
	})
	w.Resize(fyne.NewSize(480, 300))
	w.CenterOnScreen()
	w.Show()
	w.RequestFocus()
}
