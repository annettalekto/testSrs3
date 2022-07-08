package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type numericalEntry struct {
	widget.Entry
	Entered bool
}

func newNumericalEntry() *numericalEntry {
	entry := &numericalEntry{}
	entry.ExtendBaseWidget(entry)
	return entry
}

func (e *numericalEntry) TypedRune(r rune) {
	switch r {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', ',':
		fmt.Printf("Pressed %v\n", r)
		e.Entry.TypedRune(r)
	}
}

// KP_Enter
func (e *numericalEntry) TypedKey(key *fyne.KeyEvent) {
	e.Entered = false

	switch key.Name {
	case fyne.KeyEnter, fyne.KeyReturn:
		e.Entered = true
		fmt.Printf("Нажата клавиша %s\n", key.Name) // todo убрать после отладки
	case fyne.KeyBackspace, fyne.KeyDelete, fyne.KeyRight, fyne.KeyLeft, fyne.KeyHome, fyne.KeyEnd:
		e.Entry.TypedKey(key)

	}

	// if key.Name == "Return" {
	// 	// send m.Text somewhere...
	// 	fmt.Printf("Нажата клавиша %s", key.Name)
	// } else {
	// 	e.TypedKey(key)
	// }
}

func (e *numericalEntry) TypedShortcut(shortcut fyne.Shortcut) {
	paste, ok := shortcut.(*fyne.ShortcutPaste)
	if !ok {
		e.Entry.TypedShortcut(shortcut)
		return
	}

	content := paste.Clipboard.Content()
	if _, err := strconv.ParseFloat(content, 64); err == nil {
		e.Entry.TypedShortcut(shortcut)
	}
}
