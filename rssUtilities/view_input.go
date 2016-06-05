package rssUtilities

// This file handles multiplexing actions taken when specific keys are pressed
//  while using the RSS reader.

import (
	"log"

	tb "github.com/nsf/termbox-go"
)

// InputType determines how the user is interacting with the system.
type InputType uint8

const (
	// RssEntryMode means the user is inputting the URL of an RSS feed.
	RssEntryMode InputType = iota
	// RssSelectionMode means the user is picking an RSS entry.
	RssSelectionMode
)

func (v *View) enterRssSelectionMode() {
	v.inputMode = RssSelectionMode
	v.inputItemIndex = 0
	v.SetStatusMsg("[↑/↓/j/k] Move. [SPACE] Delete item. [ENTER] Open item further. [TAB] Swith to URL entry mode.", StatusInfo)
}

func (v *View) enterRssEntryMode() {
	v.inputMode = RssEntryMode
	v.SetStatusMsg("Enter the URL of an RSS feed to follow. [ENTER] to submit. [TAB] to select item.", StatusInfo)

}

func (v *View) keyActionUpSelectionMode() {
	v.inputItemIndex += 1
	if v.inputItemIndex >= v.lastSeenNumItems {
		v.inputItemIndex = v.lastSeenNumItems - 1
	}
}

func (v *View) keyActionDownSelectionMode() {
	v.inputItemIndex -= 1
	if v.inputItemIndex < 0 {
		v.inputItemIndex = 0
	}
}

func (v *View) keyActionDeleteItem() {
	v.viewLock.Lock()
	index := v.inputItemIndex
	if index < 0 || len(v.itemList) <= index {
		// SetStatusMsg also locks.
		v.viewLock.Unlock()
		v.SetStatusMsg("Attempting to delete out of range item", StatusError)
		return
	}
	if index == len(v.itemList)-1 {
		v.inputItemIndex -= 1
	}
	v.itemList = append(v.itemList[:index], v.itemList[index+1:]...)
	v.viewLock.Unlock()
}

func (v *View) reactToKeySelectionMode(k tb.Key, r rune) {
	switch k {
	case tb.KeyEsc, tb.KeyCtrlC:
		close(v.ExitRequest)
		return
	case tb.KeySpace, tb.KeyBackspace, tb.KeyBackspace2:
		v.keyActionDeleteItem()
	case tb.KeyEnter:
		log.Println("ENTER")
		// TODO ???
	case tb.KeyTab:
		v.enterRssEntryMode()
	case tb.KeyArrowLeft:
	case tb.KeyArrowRight:
		return
	case tb.KeyArrowUp:
		v.keyActionUpSelectionMode()
	case tb.KeyArrowDown:
		v.keyActionDownSelectionMode()
	default:
		s := runeToAsciiChar(r)
		switch s {
		case "k":
			v.keyActionUpSelectionMode()
		case "j":
			v.keyActionDownSelectionMode()
		}
	}
}

func (v *View) reactToKeyEntryMode(k tb.Key, r rune) {
	switch k {
	case tb.KeyEsc, tb.KeyCtrlC:
		close(v.ExitRequest)
		return
	case tb.KeyEnter:
		v.viewLock.Lock()
		v.NewFeedRequest <- string(v.input)
		v.input = make([]byte, 0)
		v.viewLock.Unlock()
	case tb.KeyTab:
		v.enterRssSelectionMode()
	case tb.KeyArrowLeft:
	case tb.KeyArrowRight:
	case tb.KeyArrowDown:
		return
	case tb.KeyArrowUp:
		v.enterRssSelectionMode()
	case tb.KeyBackspace, tb.KeyBackspace2:
		v.viewLock.Lock()
		if len(v.input) > 0 {
			v.input = v.input[:len(v.input)-1]
		}
		v.viewLock.Unlock()
	default:
		v.inputRune(r)
	}
}
