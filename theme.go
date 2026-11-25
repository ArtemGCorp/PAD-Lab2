package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// appTheme: indigo accent, softer background, rounder corners, a bit more air.
// Light and dark variants both follow the system setting.
type appTheme struct{}

var _ fyne.Theme = appTheme{}

var (
	accent      = color.NRGBA{R: 0x4F, G: 0x46, B: 0xE5, A: 0xFF} // indigo-600
	accentDark  = color.NRGBA{R: 0x81, G: 0x8C, B: 0xF8, A: 0xFF} // indigo-400
	successCol  = color.NRGBA{R: 0x05, G: 0x96, B: 0x69, A: 0xFF}
	warningCol  = color.NRGBA{R: 0xD9, G: 0x77, B: 0x06, A: 0xFF}
	bgLight     = color.NRGBA{R: 0xF4, G: 0xF5, B: 0xFB, A: 0xFF}
	cardLight   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	bgDark      = color.NRGBA{R: 0x14, G: 0x16, B: 0x24, A: 0xFF}
	cardDark    = color.NRGBA{R: 0x1E, G: 0x21, B: 0x35, A: 0xFF}
	headerLight = color.NRGBA{R: 0xE0, G: 0xE7, B: 0xFF, A: 0xFF}
	headerDark  = color.NRGBA{R: 0x2A, G: 0x2E, B: 0x4F, A: 0xFF}
)

func (appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	dark := v == theme.VariantDark
	switch n {
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		if dark {
			return accentDark
		}
		return accent
	case theme.ColorNameBackground:
		if dark {
			return bgDark
		}
		return bgLight
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		if dark {
			return cardDark
		}
		return cardLight
	case colorHeader:
		if dark {
			return headerDark
		}
		return headerLight
	case colorCard:
		if dark {
			return cardDark
		}
		return cardLight
	case theme.ColorNameSuccess:
		return successCol
	case theme.ColorNameWarning:
		return warningCol
	}
	return theme.DefaultTheme().Color(n, v)
}

// Custom colour names, resolved through the theme so they follow the variant.
const (
	colorHeader fyne.ThemeColorName = "lab2Header"
	colorCard   fyne.ThemeColorName = "lab2Card"
)

// fixedVariant forces a light or dark variant (used for screenshots in tests).
type fixedVariant struct {
	fyne.Theme
	v fyne.ThemeVariant
}

func (f fixedVariant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return f.Theme.Color(n, f.v)
}

func (appTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 15
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius, theme.SizeNameButtonRadius:
		return 8
	}
	return theme.DefaultTheme().Size(n)
}
