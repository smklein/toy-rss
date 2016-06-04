package rssUtilities

import (
	"log"
	"sync"
	"unicode/utf8"

	tb "github.com/nsf/termbox-go"
)

// StatusType modifies the statusMsg color.
type StatusType uint8

// InputType determines how the user is interacting with the system.
type InputType uint8

const (
	// StatusError indicates an error.
	StatusError StatusType = iota
	// StatusSuccess indicates a success.
	StatusSuccess
	// StatusInfo indicates info.
	StatusInfo
)

const (
	// RssEntryMode means the user is inputting the URL of an RSS feed.
	RssEntryMode InputType = iota
	// RssSelectionMode means the user is picking an RSS entry.
	RssSelectionMode
)

var defaultFeedURLs = [...]string{
	"http://feeds.arstechnica.com/arstechnica/index",
	"https://news.ycombinator.com/rss",
	//"https://www.reddit.com/.rss",
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// View gives access to the user.
type View struct {
	input         []byte
	inputMode     InputType
	statusMsg     string
	statusMsgType StatusType

	itemBufferedCap int
	itemList        []*RssEntry

	viewLock sync.RWMutex
	deathWg  *sync.WaitGroup

	// TODO(smklein): Make these public through methods, not directly.
	RedrawRequest  chan bool
	NewFeedRequest chan string
	ExitRequest    chan bool
}

// Start launches the view.
func (v *View) Start(newItemPipe chan *RssEntry, deathWg *sync.WaitGroup) {
	log.Println("View started")
	v.viewLock.Lock()
	// TODO(smklein): This should be configurable
	v.itemBufferedCap = 200
	v.itemList = make([]*RssEntry, 0)
	v.RedrawRequest = make(chan bool)
	v.NewFeedRequest = make(chan string)
	v.ExitRequest = make(chan bool)
	v.deathWg = deathWg
	v.viewLock.Unlock()

	go v.listUpdater(newItemPipe)
	go v.drawLoop()
	go v.eventLoop()
}

func (v *View) listUpdater(newItemPipe chan *RssEntry) {
	for {
		item := <-newItemPipe
		v.viewLock.Lock()
		v.itemList = append(v.itemList, item)
		if len(v.itemList) > v.itemBufferedCap {
			// TODO(smklein): Display info to user that they're missing items...
			v.itemList = v.itemList[1:]
		}
		v.viewLock.Unlock()
		v.RedrawRequest <- true
	}
}

// SetStatusMsg sets the status message.
func (v *View) SetStatusMsg(msg string, sType StatusType) {
	v.viewLock.Lock()
	v.statusMsg = msg
	v.statusMsgType = sType
	v.viewLock.Unlock()
}

func (v *View) inputRune(r rune) {
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], r)
	if n != 1 {
		panic("Unexpected rune size of 1")
	}
	v.viewLock.Lock()
	v.input = append(v.input, buf[0])
	v.viewLock.Unlock()
}

func (v *View) eventLoop() {
	// First, register the default feeds that we want to have enabled.
	// TODO(smklein): This will... probably change form once we have a
	// persistant storage story.
	for i := range defaultFeedURLs {
		v.NewFeedRequest <- defaultFeedURLs[i]
	}

	for {
		switch ev := tb.PollEvent(); ev.Type {
		case tb.EventKey:
			switch ev.Key {
			case tb.KeyEsc, tb.KeyCtrlC:
				close(v.ExitRequest)
				return
			case tb.KeyEnter:
				log.Println("ENTER")
				v.viewLock.Lock()
				v.NewFeedRequest <- string(v.input)
				v.input = make([]byte, 0)
				v.viewLock.Unlock()
			case tb.KeyTab:
				log.Println("TAB KEY. Switch mode (input / navigate)")
				v.viewLock.Lock()
				switch v.inputMode {
				case RssSelectionMode:
					v.inputMode = RssEntryMode
				case RssEntryMode:
					v.inputMode = RssSelectionMode
				}
				v.viewLock.Unlock()
			case tb.KeyArrowLeft:
				log.Println("LEFT")
			case tb.KeyArrowRight:
				log.Println("RIGHT")
			case tb.KeyArrowUp:
				log.Println("UP")
			case tb.KeyArrowDown:
				log.Println("DOWN")
			case tb.KeyBackspace, tb.KeyBackspace2:
				v.viewLock.Lock()
				if len(v.input) > 0 {
					v.input = v.input[:len(v.input)-1]
				}
				v.viewLock.Unlock()
			default:
				// TODO(smklein): Restrict keys a little more, yeah?
				log.Println("You pressed: ", ev.Ch)
				v.inputRune(ev.Ch)
			}
		case tb.EventError:
			panic(ev.Err)
		}

		// Pls redraw the screen after user input. User latency 'n' all.
		v.RedrawRequest <- true
	}
}

func (v *View) drawLoop() {
	// Initialize termbox-go
	check(tb.Init())
	defer func() {
		tb.Close()
		v.deathWg.Done()
	}()
	// TODO(smklein): Better cursor management! Not hidden tho...
	tb.SetCursor(0, 0)
	//	tb.SetInputMode(tb.InputAlt)

	for {
		v.redrawAll()
		select {
		case <-v.RedrawRequest:
		case <-v.ExitRequest:
			return
		}
	}
}

func getStatusColor(sType StatusType) tb.Attribute {
	statusColor := tb.ColorDefault
	switch sType {
	case StatusError:
		statusColor = tb.ColorRed
	case StatusSuccess:
		statusColor = tb.ColorGreen
	case StatusInfo:
		statusColor = tb.ColorCyan
	}
	return statusColor
}

func getModeColor(iType InputType) tb.Attribute {
	statusColor := tb.ColorDefault
	switch iType {
	case RssEntryMode:
		statusColor = tb.ColorGreen
	case RssSelectionMode:
		statusColor = tb.ColorBlue
	}
	return statusColor
}

func (v *View) redrawAll() {
	// TODO(smklein): Only clear parts of screen that need re-drawing...
	check(tb.Clear(tb.ColorDefault, tb.ColorDefault))
	w, h := tb.Size()

	log.Println("Width: ", w, ", Height: ", h)
	fgColor := tb.ColorGreen

	// Lock and copy anything we want to display.
	v.viewLock.RLock()
	inputString := string(v.input)
	statusString := v.statusMsg
	statusColor := getStatusColor(v.statusMsgType)
	modeColor := getModeColor(v.inputMode)

	// XXX Copying the entire item list is "easy", but potentially pretty slow.
	// XXX totally arbitrary visibility #
	LINES_PER_INDEX := 2
	maxItemsVisible := (h - 4) / LINES_PER_INDEX
	numItemsVisible := maxItemsVisible
	if len(v.itemList) < numItemsVisible {
		numItemsVisible = len(v.itemList)
	}

	itemListCopy := make([]RssEntry, numItemsVisible)
	for i := range itemListCopy {
		itemListCopy[i] = *v.itemList[i]
	}
	v.viewLock.RUnlock()

	USER_INPUT_LINE := h - 2
	STATUS_LINE := h - 1

	for y := 0; y <= h; y++ {
		for x := 0; x <= w; x++ {
			if USER_INPUT_LINE-len(itemListCopy) <= y && y < USER_INPUT_LINE {
				// RSS Items. Lowest index --> oldest.
				index := y - (USER_INPUT_LINE - len(itemListCopy))
				if index < numItemsVisible {
					item := itemListCopy[index]
					if utf8.RuneCountInString(item.ItemTitle) > 0 {
						r, size := utf8.DecodeRuneInString(item.ItemTitle)
						itemListCopy[index].ItemTitle = item.ItemTitle[size:]
						tb.SetCell(x, y, r, fgColor, tb.ColorDefault)
					}
				}
			} else if y == USER_INPUT_LINE {
				// User input
				if x <= 2 {
					tb.SetCell(x, y, '>', fgColor, tb.ColorDefault)
				} else if x == 3 {
					tb.SetCell(x, y, ' ', fgColor, tb.ColorDefault)
				} else {
					if len(inputString) > 0 {
						r, size := utf8.DecodeRuneInString(inputString)
						tb.SetCell(x, y, r, fgColor, tb.ColorDefault)
						inputString = inputString[size:]
					} else {
						tb.SetCell(x, y, ' ', fgColor, tb.ColorDefault)
					}
				}
			} else if y == STATUS_LINE {
				// Status
				if x <= 2 {
					tb.SetCell(x, y, '-', fgColor, tb.ColorDefault)
				} else if x == 3 {
					tb.SetCell(x, y, ' ', fgColor, tb.ColorDefault)
				} else {
					if len(statusString) > 0 {
						r, size := utf8.DecodeRuneInString(statusString)
						tb.SetCell(x, y, r, statusColor, tb.ColorDefault)
						statusString = statusString[size:]
					} else {
						tb.SetCell(x, y, ' ', fgColor, tb.ColorDefault)
					}
				}
			} else {
				tb.SetCell(x, y, '@', modeColor, tb.ColorDefault)
			}
		}
	}
	tb.Flush()
}
