package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pad-lab2/internal/assign"
	"pad-lab2/internal/students"
)

// card wraps content in a rounded panel.
func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(theme.Color(colorCard))
	bg.CornerRadius = 12
	bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 14, 16, 16), content))
}

// header is the tinted title band at the top of a window.
func header(icon fyne.Resource, title, subtitle string) fyne.CanvasObject {
	bg := canvas.NewRectangle(theme.Color(colorHeader))
	t := canvas.NewText(title, theme.Color(theme.ColorNameForeground))
	t.TextSize = 20
	t.TextStyle = fyne.TextStyle{Bold: true}
	sub := widget.NewLabel(subtitle)
	sub.Wrapping = fyne.TextWrapWord
	ic := widget.NewIcon(icon)
	iconBox := container.NewGridWrap(fyne.NewSize(36, 36), ic)
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 8, 16, 16),
		container.NewBorder(nil, nil, container.NewCenter(iconBox), nil,
			container.NewVBox(t, sub))))
}

func fieldLabel(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

// badge is a small coloured pill with text.
func badge(text string, fill color.Color) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.CornerRadius = 10
	t := canvas.NewText(text, color.White)
	t.TextSize = 13
	t.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewHBox(container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(3, 3, 10, 10), t)))
}

// numberDot is a filled circle with the question number.
func numberDot(n int) fyne.CanvasObject {
	c := canvas.NewCircle(theme.Color(theme.ColorNamePrimary))
	t := canvas.NewText(fmt.Sprint(n), color.White)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.Alignment = fyne.TextAlignCenter
	return container.NewCenter(container.NewGridWrap(fyne.NewSize(26, 26), container.NewStack(c, container.NewCenter(t))))
}

// resultView is the "Ваш билет" window.
type resultView struct {
	win       fyne.Window
	number    *canvas.Text
	student   *widget.Label
	badgeText string
	questions []*widget.Label
	closeBtn  *widget.Button
}

func newResultView(a fyne.App, group string, s students.Student, d assign.Decision) *resultView {
	rv := &resultView{win: a.NewWindow(fmt.Sprintf("Билет № %d", d.Ticket.Number))}

	caption := canvas.NewText("ВАШ БИЛЕТ", theme.Color(theme.ColorNamePlaceHolder))
	caption.TextSize = 12
	caption.TextStyle = fyne.TextStyle{Bold: true}
	caption.Alignment = fyne.TextAlignCenter

	rv.number = canvas.NewText(fmt.Sprintf("№ %d", d.Ticket.Number), theme.Color(theme.ColorNamePrimary))
	rv.number.TextSize = 54
	rv.number.TextStyle = fyne.TextStyle{Bold: true}
	rv.number.Alignment = fyne.TextAlignCenter

	rv.student = widget.NewLabelWithStyle(fmt.Sprintf("%s · группа %s", s.Display(), group),
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	if d.Repeat {
		rv.badgeText = "Повторный запрос — выдан тот же билет"
	} else {
		rv.badgeText = "Новый билет"
	}
	fill := theme.Color(theme.ColorNameSuccess)
	if d.Repeat {
		fill = theme.Color(theme.ColorNameWarning)
	}
	top := container.NewVBox(caption, rv.number, rv.student, container.NewCenter(badge(rv.badgeText, fill)))

	list := container.NewVBox(fieldLabel("Вопросы"))
	if len(d.Ticket.Questions) == 0 {
		list.Add(widget.NewLabel("Вопросы этого билета не найдены в tickets.docx."))
	}
	for i, q := range d.Ticket.Questions {
		l := widget.NewLabel(q)
		l.Wrapping = fyne.TextWrapWord
		rv.questions = append(rv.questions, l)
		if i > 0 {
			list.Add(widget.NewSeparator())
		}
		list.Add(container.NewBorder(nil, nil, numberDot(i+1), nil, l))
	}

	rv.closeBtn = widget.NewButtonWithIcon("Закрыть", theme.CancelIcon(), rv.win.Close)
	hint := widget.NewLabelWithStyle("ESC — закрыть", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	footer := container.NewBorder(nil, nil, nil, rv.closeBtn, hint)

	bg := canvas.NewRectangle(theme.Color(colorHeader))
	rv.win.SetContent(container.NewBorder(
		container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(18, 14, 16, 16), top)),
		container.NewPadded(footer), nil, nil,
		container.NewPadded(card(list)),
	))
	rv.win.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			rv.win.Close()
		}
	})
	rv.win.Resize(fyne.NewSize(540, 600))
	// Wrapped labels compute their height for the width they had when built;
	// refresh once the window has its real width so rows are not too tall.
	for _, q := range rv.questions {
		q.Refresh()
	}
	rv.win.Content().Refresh()
	return rv
}
