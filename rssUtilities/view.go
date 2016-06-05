package rssUtilities

import (
	"log"
	"os"
	"sync"
	"unicode/utf8"

	tb "github.com/nsf/termbox-go"
)

// StatusType modifies the statusMsg color.
type StatusType uint8

const (
	// StatusError indicates an error.
	StatusError StatusType = iota
	// StatusSuccess indicates a success.
	StatusSuccess
	// StatusInfo indicates info.
	StatusInfo
)

var defaultFeedURLs = [...]string{
	"http://feeds.arstechnica.com/arstechnica/index",
	"https://news.ycombinator.com/rss",
	//"https://www.reddit.com/.rss",
}

// XXX This is the bad shut down. Use exitrequests.
func check(e error) {
	if e != nil {
		panic(e)
	}
}

type channelInfo struct {
	channelColor tb.Attribute
}

// View gives access to the user.
type View struct {
	// Rss Entry items
	lastSeenNumItems int
	itemBufferedCap  int
	itemList         []*RssEntry
	channelInfoMap   map[string]*channelInfo

	// User input & Status message
	input          []byte
	inputMode      InputType
	inputItemIndex int
	statusMsg      string
	statusMsgType  StatusType

	// Synchronization tools
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
	v.channelInfoMap = make(map[string]*channelInfo)
	v.deathWg = deathWg
	v.RedrawRequest = make(chan bool)
	v.NewFeedRequest = make(chan string)
	v.ExitRequest = make(chan bool)
	v.viewLock.Unlock()

	go v.listUpdater(newItemPipe)
	go v.drawLoop()
	go v.eventLoop()
}

func (v *View) AddChannelInfo(title string) {
	v.viewLock.Lock()
	v.channelInfoMap[title] = &channelInfo{}
	v.viewLock.Unlock()
}

func (v *View) listUpdater(newItemPipe chan *RssEntry) {
	for {
		item := <-newItemPipe
		v.viewLock.Lock()
		// New items are placed at the BACK of the itemList.
		v.itemList = append(v.itemList, item)
		if len(v.itemList) > v.itemBufferedCap {
			// TODO(smklein): Display info to user that they're missing items...
			// Old items are removed from the FRONT of the itemList.
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

func runeToAsciiChar(r rune) string {
	if utf8.RuneLen(r) != 1 {
		log.Println("Attempting to convert rune to ascii char: ", r)
		os.Exit(-1)
	}

	var buf [utf8.UTFMax]byte
	utf8.EncodeRune(buf[:], r)
	return string(buf[0])
}

func (v *View) inputRune(r rune) {
	var buf [utf8.UTFMax]byte
	utf8.EncodeRune(buf[:], r)
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
			switch v.inputMode {
			case RssSelectionMode:
				v.reactToKeySelectionMode(ev.Key, ev.Ch)
			case RssEntryMode:
				v.reactToKeyEntryMode(ev.Key, ev.Ch)
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

func redrawMetadata(width, startLine int, item *RssEntry) int {
	for x := 0; x <= width; x++ {
		// Metadata about item
		if x < utf8.RuneCountInString(item.FeedTitle) {
			// TODO(smklein): Try doing this casting when converting from
			// SlyMarbo's RSS --> my format.
			r := []rune(item.FeedTitle)[x]
			tb.SetCell(x, startLine, r, fgColor, bgColor)
		} else {
			tb.SetCell(x, startLine, ' ', blankFgColor, bgColor)

		}
	}
	return 1
}

func redrawItemTitle(width, startLine int, item *RssEntry) int {
	for x := 0; x <= width; x++ {
		// Item itself
		switch x < 2 {
		case true:
			tb.SetCell(x, startLine, ' ', blankFgColor, bgColor)
		case false:
			if x-2 < utf8.RuneCountInString(item.ItemTitle) {
				r := []rune(item.ItemTitle)[x-2]
				tb.SetCell(x, startLine, r, fgColor, bgColor)
			} else {
				tb.SetCell(x, startLine, ' ', blankFgColor, bgColor)
			}
		}
	}
	return 1
}

func (v *View) redrawRssItem(width, startLine, itemIndex int, itemListCopy []RssEntry) int {
	oldFgColor := fgColor
	if v.inputMode == RssSelectionMode && v.inputItemIndex == itemIndex {
		fgColor = tb.ColorBlue | tb.AttrUnderline
	}
	item := itemListCopy[itemIndex]
	linesUsed := 0
	switch item.State {
	case defaultEntryState:
		// Requires two lines, the minimum number of lines. Will always fit.
		titleLines := redrawItemTitle(width, startLine, &item)
		metadataLines := redrawMetadata(width, startLine-titleLines, &item)

		linesUsed = titleLines + metadataLines
	case expandedEntryState:
		panic("Unimplemented expanded state")
	default:
		panic("Unhandled entry state")
	}

	fgColor = oldFgColor
	return linesUsed
}

var fgColor tb.Attribute = tb.ColorGreen
var blankFgColor tb.Attribute = tb.ColorGreen
var bgColor tb.Attribute = tb.ColorDefault

func (v *View) redrawStatus(width, line int, statusString string, statusColor tb.Attribute) {
	statusStartCol := 4
	for x := 0; x <= width; x++ {
		if x <= 2 {
			// '-' before input
			tb.SetCell(x, line, '-', fgColor, bgColor)
		} else if statusStartCol <= x && 0 < len(statusString) {
			// Actual status
			r, size := utf8.DecodeRuneInString(statusString)
			statusString = statusString[size:]
			tb.SetCell(x, line, r, statusColor, bgColor)
		} else {
			// Clear out the rest
			tb.SetCell(x, line, ' ', blankFgColor, bgColor)
		}
	}
}

func (v *View) redrawUserInput(width, line int, inputString string) {
	userInputStartCol := 4
	inputStringStartLen := len(inputString)
	for x := 0; x <= width; x++ {
		if x <= 2 {
			// '>' before input
			tb.SetCell(x, line, '>', fgColor, bgColor)
		} else if userInputStartCol <= x && 0 < len(inputString) {
			// Actual user input
			r, size := utf8.DecodeRuneInString(inputString)
			inputString = inputString[size:]
			tb.SetCell(x, line, r, fgColor, bgColor)
		} else {
			if x == userInputStartCol+inputStringStartLen {
				// Cursor goes after user input
				tb.SetCursor(x, line)
			}
			// Clear out the rest
			tb.SetCell(x, line, ' ', blankFgColor, bgColor)
		}
	}
}

func (v *View) redrawAll() {
	// TODO(smklein): Only clear parts of screen that need re-drawing...
	check(tb.Clear(tb.ColorDefault, tb.ColorDefault))
	w, h := tb.Size()

	fgColor = tb.ColorGreen

	// Lock and copy anything we want to display.
	v.viewLock.RLock()
	inputString := string(v.input)
	statusString := v.statusMsg
	statusColor := getStatusColor(v.statusMsgType)

	// XXX Copying the entire item list is "easy", but potentially pretty slow.
	minLinesPerEntry := 2
	linesUsableByEntries := (h - 4)
	// The largest possible number of items in view.
	numItemsInView := linesUsableByEntries / minLinesPerEntry
	if len(v.itemList) < numItemsInView {
		// Which may be even smaller if there are less items available.
		numItemsInView = len(v.itemList)
	}

	itemListCopy := make([]RssEntry, numItemsInView)
	for i := range itemListCopy {
		itemListCopy[i] = *v.itemList[i]
	}
	v.viewLock.RUnlock()

	rssEntryLine := h - 3 // Initialized to the starting line.
	userInputLine := h - 2
	statusLine := h - 1

	// While another entry can fit...
	itemIndex := 0
	for rssEntryLine-minLinesPerEntry > 0 {
		if itemIndex >= numItemsInView {
			break
		}
		rssEntryLine -= v.redrawRssItem(w, rssEntryLine, itemIndex, itemListCopy)
		itemIndex += 1
	}

	// XXX take caution, this is not locked
	v.lastSeenNumItems = itemIndex

	v.redrawUserInput(w, userInputLine, inputString)
	v.redrawStatus(w, statusLine, statusString, statusColor)

	tb.Flush()
}
