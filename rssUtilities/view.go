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

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// View gives access to the user.
type View struct {
	maxLines      int
	input         []byte
	inputMode     InputType
	statusMsg     string
	statusMsgType StatusType

	// XXX ACCESS HERE IS RACY FIXME PLS
	itemList []*RssEntry

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
	v.maxLines = 50
	v.itemList = make([]*RssEntry, 0)
	v.RedrawRequest = make(chan bool)
	v.NewFeedRequest = make(chan string)
	v.ExitRequest = make(chan bool)
	v.deathWg = deathWg

	go v.listUpdater(newItemPipe)
	go v.drawLoop()
	go v.eventLoop()
}

func (v *View) listUpdater(newItemPipe chan *RssEntry) {
	for {
		item := <-newItemPipe
		// TODO come up with better synchronization for this slice...
		v.viewLock.Lock()
		v.itemList = append(v.itemList, item)
		v.viewLock.Unlock()
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
	v.viewLock.RUnlock()

	log.Println("Input string: ", inputString)
	log.Println("Status string: ", statusString)

	numItems := len(v.itemList)
	numItemsVisible := numItems
	// XXX totally arbitrary
	maxItemsVisible := h / 2
	if numItemsVisible > maxItemsVisible {
		numItemsVisible = maxItemsVisible
	}

	log.Println("# items visible: ", numItemsVisible)

	for x := 0; x <= w; x++ {
		for y := 0; y <= h; y++ {
			if numItems != len(v.itemList) {
				// XXX This panic shouldn't exist
				panic("Time to lock itemlist, bruh")
			}

			if h-2-numItems <= y && y < h-2 {
				// RSS Items. Lowest index --> oldest.
				index := y - (h - 2 - numItems)
				if index < numItemsVisible {
					item := v.itemList[index]
					if x < len(item.ItemTitle) {
						log.Println("Index: ", index)
						log.Println(item.ItemTitle[x:])
						r, _ := utf8.DecodeRuneInString(item.ItemTitle[x:])
						tb.SetCell(x, y, r, fgColor, tb.ColorDefault)
					}
				}
			} else if y == h-2 {
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
			} else if y == h-1 {
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
