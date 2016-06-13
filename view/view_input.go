package view

// TODO move this to a new package...

// This file handles multiplexing actions taken when specific keys are pressed
//  while using the RSS reader.

import (
	"log"
	"os"
	"sync"
	"unicode/utf8"

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

type InputManager struct {
	// What has the user typed so far?
	inputText     []byte
	inputTextLock sync.RWMutex
	// Is the user in typing, selection mode, or... ?
	inputMode InputType
	// The item the user is selecting (and the max item they CAN select).
	inputItemIndex       int
	inputNumItemsVisible int

	// Outgoing requests (made BY the InputManager)
	sharedChanNewFeedRequest chan string /* Whatever the user has inputted */

	// View which created us.
	view ViewInterface

	// Incoming requests (made TO the InputManager)
	chanSetLastSeenNumItems chan int
	chanGetSelectionMode    chan InputType
	chanGetItemIndex        chan int
	chanGetInputString      chan string

	// Global exit request. When this is closed, GTFO.
	exitRequest chan bool
}

func (im *InputManager) Start(sharedChanNewFeedRequest chan string, v ViewInterface) {
	log.Println("InputManager Start")
	im.sharedChanNewFeedRequest = sharedChanNewFeedRequest

	im.view = v
	im.chanSetLastSeenNumItems = make(chan int)
	im.chanGetSelectionMode = make(chan InputType)
	im.chanGetItemIndex = make(chan int)
	im.chanGetInputString = make(chan string)
	im.exitRequest = v.GetChanExitRequest()

	// These two threads comprise the entire input.
	go im.respondToView()
	go im.reactToKeys()
	log.Println("InputManager Start complete")
}

// Channel requests (no response expected)

func (im *InputManager) SetLastSeenNumItems(i int) {
	im.chanSetLastSeenNumItems <- i
}

// Channel requests (responses expected)

func (im *InputManager) GetSelectionMode() InputType {
	im.chanGetSelectionMode <- 0
	return <-im.chanGetSelectionMode
}
func (im *InputManager) GetItemIndex() int {
	im.chanGetItemIndex <- 0
	return <-im.chanGetItemIndex
}
func (im *InputManager) GetInputString() string {
	im.chanGetInputString <- ""
	return <-im.chanGetInputString
}

// Functions for switching Selection Modes

func (im *InputManager) enterRssSelectionMode() {
	im.inputMode = RssSelectionMode
	im.inputItemIndex = 0
	im.view.SetStatus(StatusMsgStruct{"[↑/↓/j/k]:Move [→/l]:Expand [←/h]:Collapse [SPACE]:Delete [ENTER]:Color [TAB]:URL entry", StatusInfo})
}

func (im *InputManager) enterRssEntryMode() {
	im.inputMode = RssEntryMode
	im.view.SetStatus(StatusMsgStruct{"Enter the URL of an RSS feed to follow [ENTER]:Submit [TAB]:Item Selection Mode", StatusInfo})

}

// Key actions

func (im *InputManager) keyActionUpSelectionMode() {
	im.inputItemIndex += 1
	if im.inputItemIndex >= im.inputNumItemsVisible {
		im.inputItemIndex = im.inputNumItemsVisible - 1
	}
}

func (im *InputManager) keyActionDownSelectionMode() {
	im.inputItemIndex -= 1
	if im.inputItemIndex < 0 {
		im.inputItemIndex = 0
	}
}

func (im *InputManager) keyActionDeleteItem() {
	im.view.DeleteItem(im.inputItemIndex)
}

func (im *InputManager) keyActionCollapseSelectionMode() {
	im.view.CollapseItem(im.inputItemIndex)
}

func (im *InputManager) keyActionExpandSelectionMode() {
	im.view.ExpandItem(im.inputItemIndex)
}

func (im *InputManager) keyActionUpdateColorSelectionMode() {
	im.view.ChangeColor(im.inputItemIndex)
}

// All functions which modify input text
// TODO maybe move to a new file / object? Seems separate...

func (im *InputManager) inputTextAddRune(r rune) {
	var buf [utf8.UTFMax]byte
	utf8.EncodeRune(buf[:], r)
	im.inputTextLock.Lock()
	defer im.inputTextLock.Unlock()
	im.inputText = append(im.inputText, buf[0])
}

func (im *InputManager) inputTextDeleteRune() {
	im.inputTextLock.Lock()
	defer im.inputTextLock.Unlock()
	if len(im.inputText) > 0 {
		im.inputText = im.inputText[:len(im.inputText)-1]
	}
}

func (im *InputManager) inputTextMakeEmpty() {
	im.inputTextLock.Lock()
	defer im.inputTextLock.Unlock()
	im.inputText = make([]byte, 0)
}

func (im *InputManager) inputTextAsString() string {
	im.inputTextLock.RLock()
	defer im.inputTextLock.RUnlock()
	return string(im.inputText)
}

// Functions responsible for reacting to certain actions

func (im *InputManager) respondToView() {
	for {
		select {
		case numItemsVisible := <-im.chanSetLastSeenNumItems:
			log.Println("IM sees Request for last seen num items: ", numItemsVisible)
			// TODO THIS IS UNLOCKED. IF SHOULD DEF BE LOCKED.
			im.inputNumItemsVisible = numItemsVisible
			if im.inputItemIndex >= numItemsVisible {
				im.inputItemIndex = numItemsVisible - 1
			}
		case <-im.chanGetSelectionMode:
			log.Println("IM sees Request for selection mode: ", im.inputMode)
			im.chanGetSelectionMode <- im.inputMode
		case <-im.chanGetItemIndex:
			log.Println("IM sees Request for get item index: ", im.inputItemIndex)
			im.chanGetItemIndex <- im.inputItemIndex
		case <-im.chanGetInputString:
			log.Println("IM sees Request for inputstring", im.inputTextAsString())
			im.chanGetInputString <- im.inputTextAsString()
		}
	}
}

func (im *InputManager) reactToKeys() {
	for {
		switch ev := tb.PollEvent(); ev.Type {
		case tb.EventKey:
			switch im.inputMode {
			case RssSelectionMode:
				im.reactToKeySelectionMode(ev.Key, ev.Ch)
			case RssEntryMode:
				im.reactToKeyEntryMode(ev.Key, ev.Ch)
			}
		case tb.EventError:
			log.Println("Received erroneous event while reacing to keys:")
			log.Println(ev.Err.Error())
		}

		// Pls redraw the screen after user input. User latency 'n' all.
		im.view.Redraw()
	}
}

func (im *InputManager) reactToKeySelectionMode(k tb.Key, r rune) {
	switch k {
	case tb.KeyEsc, tb.KeyCtrlC:
		close(im.exitRequest)
		return
	case tb.KeySpace, tb.KeyBackspace, tb.KeyBackspace2:
		im.keyActionDeleteItem()
	case tb.KeyEnter:
		im.keyActionUpdateColorSelectionMode()
	case tb.KeyTab:
		im.enterRssEntryMode()
	case tb.KeyArrowLeft:
		im.keyActionCollapseSelectionMode()
	case tb.KeyArrowRight:
		im.keyActionExpandSelectionMode()
	case tb.KeyArrowUp:
		im.keyActionUpSelectionMode()
	case tb.KeyArrowDown:
		im.keyActionDownSelectionMode()
	default:
		s := runeToAsciiChar(r)
		switch s {
		case "k":
			im.keyActionUpSelectionMode()
		case "j":
			im.keyActionDownSelectionMode()
		case "h":
			im.keyActionCollapseSelectionMode()
		case "l":
			im.keyActionExpandSelectionMode()
		}
	}
}

func (im *InputManager) reactToKeyEntryMode(k tb.Key, r rune) {
	switch k {
	case tb.KeyEsc, tb.KeyCtrlC:
		close(im.exitRequest)
		return
	case tb.KeyEnter:
		im.sharedChanNewFeedRequest <- im.inputTextAsString()
		im.inputTextMakeEmpty()
	case tb.KeyTab:
		im.enterRssSelectionMode()
	case tb.KeyArrowLeft:
	case tb.KeyArrowRight:
	case tb.KeyArrowDown:
		return
	case tb.KeyArrowUp:
		im.enterRssSelectionMode()
	case tb.KeyBackspace, tb.KeyBackspace2:
		im.inputTextDeleteRune()
	default:
		im.inputTextAddRune(r)
	}
}

// Utility functions

func runeToAsciiChar(r rune) string {
	if utf8.RuneLen(r) != 1 {
		log.Println("Attempting to convert rune to ascii char: ", r)
		os.Exit(-1)
	}

	var buf [utf8.UTFMax]byte
	utf8.EncodeRune(buf[:], r)
	return string(buf[0])
}
