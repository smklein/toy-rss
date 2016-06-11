package rssUtilities

import (
	"errors"
	"log"
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

type StatusMsgStruct struct {
	Message string
	Type    StatusType
}

// TODO(smklein): Use default colors
var defaultFeedURLs = [...]string{
	"http://feeds.arstechnica.com/arstechnica/index",
	"https://news.ycombinator.com/rss",
	"https://lwn.net/headlines/rss",
	"http://feeds.feedburner.com/linuxjournalcom?format=xml",
	//"https://www.reddit.com/.rss",
}

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
	itemBufferedCap int
	itemList        []*RssEntry
	channelInfoMap  map[string]*channelInfo /* Title --> Info */

	// Status Message
	status StatusMsgStruct

	// Synchronization tools
	// TODO this needs to become more fine-grained before it spirals out of control
	viewLock sync.RWMutex
	deathWg  *sync.WaitGroup

	// Mechanism to interact with input (presumeably keyboard)
	inputManager *InputManager

	// When this is closed, GTFO.
	exitRequest chan bool

	// INCOMING
	setStatusRequest   chan StatusMsgStruct
	redrawRequest      chan bool
	deleteItemRequest  chan int /* Item Index */
	changeColorRequest chan int /* Item Index */
}

// Start launches the view. Undefined to call multiple times.
func (v *View) Start(newItemPipe chan *RssEntry, newFeedRequest chan string, deathWg *sync.WaitGroup) {
	log.Println("View Start called")
	v.viewLock.Lock()
	// TODO(smklein): This should be configurable
	v.itemBufferedCap = 200
	v.itemList = make([]*RssEntry, 0)
	v.channelInfoMap = make(map[string]*channelInfo)

	v.deathWg = deathWg

	v.inputManager = new(InputManager)
	v.exitRequest = make(chan bool)

	v.setStatusRequest = make(chan StatusMsgStruct)
	v.redrawRequest = make(chan bool)
	v.deleteItemRequest = make(chan int)
	v.changeColorRequest = make(chan int)
	v.viewLock.Unlock()

	v.inputManager.Start(newFeedRequest, v)

	go v.listUpdater(newItemPipe)
	go v.drawLoop()

	// First, register the default feeds that we want to have enabled.
	// TODO(smklein): This will... probably change form once we have a
	// persistant storage story.
	go func() {
		for i := range defaultFeedURLs {
			newFeedRequest <- defaultFeedURLs[i]
		}
	}()
	log.Println("View Start Complete")
}

func (v *View) GetChanExitRequest() chan bool {
	return v.exitRequest
}
func (v *View) DeleteItem(i int) {
	v.deleteItemRequest <- i
}
func (v *View) SetStatus(status StatusMsgStruct) {
	v.setStatusRequest <- status
}
func (v *View) ChangeColor(i int) {
	v.changeColorRequest <- i
}
func (v *View) Redraw() {
	v.redrawRequest <- true
}
func (v *View) AddChannelInfo(title string) {
	v.viewLock.Lock()
	v.channelInfoMap[title] = &channelInfo{}
	v.channelInfoMap[title].channelColor = fgColor
	v.viewLock.Unlock()
}
func (v *View) CollapseItem(index int) {
	v.viewLock.Lock()
	if 0 <= index && index < len(v.itemList) {
		v.itemList[index].State = CollapseEntryState(v.itemList[index].State)
	}
	v.viewLock.Unlock()
}
func (v *View) ExpandItem(index int) {
	v.viewLock.Lock()
	if 0 <= index && index < len(v.itemList) {
		v.itemList[index].State = ExpandEntryState(v.itemList[index].State)
	}
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
		v.redrawRequest <- true
	}
}

func (v *View) setStatusMsg(status StatusMsgStruct) {
	v.status = status
}

func (v *View) checkItemIndexValid(index int) error {
	if index < 0 || len(v.itemList) <= index {
		return errors.New("Attempting to access out of range item")
	}
	return nil
}

func (v *View) tryToDeleteItem(index int) {
	v.viewLock.Lock()
	defer v.viewLock.Unlock()
	if err := v.checkItemIndexValid(index); err != nil {
		v.setStatusMsg(StatusMsgStruct{err.Error(), StatusError})
		return
	}
	v.itemList = append(v.itemList[:index], v.itemList[index+1:]...)
}

func (v *View) tryToChangeItemColor(index int) {
	v.viewLock.Lock()
	defer v.viewLock.Unlock()
	if err := v.checkItemIndexValid(index); err != nil {
		v.setStatusMsg(StatusMsgStruct{err.Error(), StatusError})
		return
	}
	feedTitle := v.itemList[index].FeedTitle
	log.Println("Updating color selection mode")
	if chInfo, ok := v.channelInfoMap[feedTitle]; ok {
		color := chInfo.channelColor
		chInfo.channelColor = (color + 1) % tb.ColorWhite
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
		case <-v.redrawRequest:
		case status := <-v.setStatusRequest:
			v.setStatusMsg(status)
		case index := <-v.deleteItemRequest:
			v.tryToDeleteItem(index)
		case index := <-v.changeColorRequest:
			v.tryToChangeItemColor(index)
		case <-v.exitRequest:
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

func redrawMetadataAndItemDense(width, startLine int, metadataFgColor, itemFgColor tb.Attribute, item *RssEntry) int {
	// Abbreviate titles if necessary.
	itemTitleLen := utf8.RuneCountInString(item.ItemTitle)
	channelTitleLen := utf8.RuneCountInString(item.FeedTitle)
	itemTitleStart := 17
	maxChannelTitleLen := 15
	dotDotDotLen := 0
	if channelTitleLen > maxChannelTitleLen {
		dotDotDotLen = maxChannelTitleLen
		channelTitleLen = maxChannelTitleLen - 3
	}

	for x := 0; x <= width; x++ {
		// Metadata about item
		if x < channelTitleLen {
			// TODO(smklein): Try doing this casting when converting from
			// SlyMarbo's RSS --> my format.
			r := []rune(item.FeedTitle)[x]
			tb.SetCell(x, startLine, r, metadataFgColor, bgColor)
		} else if x < dotDotDotLen {
			// Does not happen if dotDotDotLen is zero.
			tb.SetCell(x, startLine, '.', metadataFgColor, bgColor)
		} else if x >= itemTitleStart && x-itemTitleStart < itemTitleLen {
			r := []rune(item.ItemTitle)[x-itemTitleStart]
			tb.SetCell(x, startLine, r, itemFgColor, bgColor)
		} else {
			tb.SetCell(x, startLine, ' ', blankFgColor, bgColor)

		}
	}
	return 1
}

func redrawMetadata(width, startLine int, fgColor tb.Attribute, item *RssEntry) int {
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

func redrawItemTitle(width, startLine int, fgColor tb.Attribute, item *RssEntry) int {
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

func (v *View) redrawRssItem(width, startLine, itemIndex, inputItemIndex int, inputMode InputType, itemListCopy []RssEntry) int {
	item := itemListCopy[itemIndex]

	itemFgColor := fgColor
	metadataFgColor := fgColor

	if inputMode == RssSelectionMode && inputItemIndex == itemIndex {
		itemFgColor = tb.ColorBlue | tb.AttrUnderline
	}
	if chInfo, ok := v.channelInfoMap[item.FeedTitle]; ok {
		metadataFgColor = chInfo.channelColor
	}

	linesUsed := 0

	switch item.State {
	case standardEntryState:
		// Requires two lines, the minimum number of lines. Will always fit.
		titleLines := redrawItemTitle(width, startLine, itemFgColor, &item)
		metadataLines := redrawMetadata(width, startLine-titleLines, metadataFgColor, &item)

		linesUsed = titleLines + metadataLines
	case expandedEntryState:
		// TODO probably return err if it doesn't fit? So we can stop drawing? Or something?
		panic("Unimplemented expanded state")
	case collapsedEntryState:
		linesUsed = redrawMetadataAndItemDense(width, startLine, metadataFgColor, itemFgColor, &item)
	default:
		panic("Unhandled entry state")
	}

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
	tb.Clear(tb.ColorDefault, tb.ColorDefault)
	w, h := tb.Size()

	fgColor = tb.ColorGreen

	// Lock and copy anything we want to display.
	v.viewLock.RLock()
	statusString := v.status.Message
	statusColor := getStatusColor(v.status.Type)

	minLinesPerEntry := 1
	linesUsableByEntries := (h - 4)
	// The largest possible number of items in view.
	numItemsInView := linesUsableByEntries / minLinesPerEntry
	if len(v.itemList) < numItemsInView {
		// Which may be even smaller if there are less items available.
		numItemsInView = len(v.itemList)
	}

	// XXX Copying the entire item list is "easy", but potentially pretty slow.
	itemListCopy := make([]RssEntry, numItemsInView)
	for i := range itemListCopy {
		itemListCopy[i] = *v.itemList[i]
	}
	v.viewLock.RUnlock()

	rssEntryLine := h - 3 // Initialized to the starting line.
	userInputLine := h - 2
	statusLine := h - 1

	// Let's cache these things so we don't pester the inputManager multiple
	// times while redrawing.
	inputString := v.inputManager.GetInputString()
	inputMode := v.inputManager.GetSelectionMode()
	inputItemIndex := v.inputManager.GetItemIndex()

	// While another entry can fit...
	itemIndex := 0
	for rssEntryLine-minLinesPerEntry > 0 {
		if itemIndex >= numItemsInView {
			break
		}
		rssEntryLine -= v.redrawRssItem(w, rssEntryLine, itemIndex, inputItemIndex, inputMode, itemListCopy)
		itemIndex += 1
	}

	// TODO(smklein): Might be fun to have an "all items gone" image?
	v.inputManager.SetLastSeenNumItems(itemIndex)

	v.redrawUserInput(w, userInputLine, inputString)
	v.redrawStatus(w, statusLine, statusString, statusColor)

	tb.Flush()
}
