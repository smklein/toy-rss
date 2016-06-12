package view

import (
	"errors"
	"log"
	"os/exec"
	"sync"
	"time"
	"unicode/utf8"

	tb "github.com/nsf/termbox-go"
	"github.com/smklein/toy-rss/feed"
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
	itemList        []*feed.RssEntry
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
func (v *View) Start(newItemPipe chan *feed.RssEntry, newFeedRequest chan string, deathWg *sync.WaitGroup) {
	log.Println("View Start called")
	v.viewLock.Lock()
	// TODO(smklein): This should be configurable
	v.itemBufferedCap = 200
	v.itemList = make([]*feed.RssEntry, 0)
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
		v.itemList[index].State = feed.CollapseEntryState(v.itemList[index].State)
	}
	v.viewLock.Unlock()
}
func (v *View) ExpandItem(index int) {
	var openInBrowser = false
	v.viewLock.Lock()
	if 0 <= index && index < len(v.itemList) {
		v.itemList[index].State = feed.ExpandEntryState(v.itemList[index].State)
	}
	if v.itemList[index].State == feed.BrowserEntryState {
		v.itemList[index].State = feed.CollapseEntryState(v.itemList[index].State)
		openInBrowser = true
	}
	v.viewLock.Unlock()

	if openInBrowser {
		cmd := exec.Command("google-chrome", v.itemList[index].URL)
		err := cmd.Start()
		if err != nil {
			log.Fatal("Start error: ", err)
		}
		err = cmd.Wait()
		if err != nil {
			log.Fatal("Wait error: ", err)
		}
	}
}

func (v *View) listUpdater(newItemPipe chan *feed.RssEntry) {
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

func formatToLen(r []rune, l int) []rune {
	if len(r) > l {
		return append(r[:l-3], []rune("...")...)
	} else {
		return r
	}
}

type lineElement struct {
	contents []rune
	maxLen   int
	color    tb.Attribute
}

func redrawLine(width, startLine int, elements []lineElement) {
	ei := 0

	start := 0
	element := formatToLen(elements[ei].contents, elements[ei].maxLen)
	length := len(element)
	for x := 0; x <= width; x++ {
		if start <= x && x < start+length {
			// Print out current element
			r := element[x-start]
			tb.SetCell(x, startLine, r, elements[ei].color, bgColor)
		} else if /* There is another element */ ei+1 < len(elements) &&
			/* We are padded enough */ start+length < x {
			ei = ei + 1
			start = x + 1
			element = formatToLen(elements[ei].contents, elements[ei].maxLen)
			length = len(element)
			tb.SetCell(x, startLine, ' ', blankFgColor, bgColor)
		} else {
			tb.SetCell(x, startLine, ' ', blankFgColor, bgColor)
		}
	}
}

func (v *View) redrawCollapsedState(width, startLine int, item feed.RssEntry, itemFgColor, metadataFgColor tb.Attribute) int {
	// [Time] [Feed Title] [ItemTitle]
	redrawLine(width, startLine, []lineElement{
		{
			contents: []rune(item.ItemDate.Format(time.UnixDate))[:20],
			maxLen:   20,
			color:    fgColor,
		},
		{
			contents: []rune(item.FeedTitle),
			maxLen:   20,
			color:    metadataFgColor,
		},
		{
			contents: []rune(item.ItemTitle),
			maxLen:   60,
			color:    itemFgColor,
		},
	})
	return 1
}

func (v *View) redrawStandardState(width, startLine int, item feed.RssEntry, itemFgColor, metadataFgColor tb.Attribute) int {
	// [Time] [Feed Title]
	//   [ItemTitle]
	redrawLine(width, startLine-1, []lineElement{
		{
			contents: []rune(item.ItemDate.Format(time.UnixDate))[:20],
			maxLen:   20,
			color:    fgColor,
		},
		{
			contents: []rune(item.FeedTitle),
			maxLen:   80,
			color:    metadataFgColor,
		},
	})
	redrawLine(width, startLine, []lineElement{
		{
			contents: append([]rune("  "), []rune(item.ItemTitle)...),
			maxLen:   120,
			color:    itemFgColor,
		},
	})
	return 2
}

func (v *View) redrawExpandedState(width, startLine int, item feed.RssEntry, itemFgColor, metadataFgColor tb.Attribute) int {
	// [Time] [Feed Title]
	//   [ItemTitle]
	//   [ItemContent]
	redrawLine(width, startLine-2, []lineElement{
		{
			contents: []rune(item.ItemDate.Format(time.UnixDate))[:20],
			maxLen:   20,
			color:    fgColor,
		},
		{
			contents: []rune(item.FeedTitle),
			maxLen:   80,
			color:    metadataFgColor,
		},
	})
	redrawLine(width, startLine-1, []lineElement{
		{
			contents: append([]rune("  "), []rune(item.ItemTitle)...),
			maxLen:   120,
			color:    itemFgColor,
		},
	})

	redrawLine(width, startLine, []lineElement{
		{
			contents: append([]rune("  "), []rune(item.ItemContent)...),
			maxLen:   120,
			color:    itemFgColor,
		},
	})

	return 3
}

func (v *View) redrawRssItem(width, startLine, itemIndex, inputItemIndex int, inputMode InputType, itemListCopy []feed.RssEntry) int {
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

	// TODO add mechanism to "downgrade" size if it wouldn't fit on the top of the screen.
	state := item.State
	for linesUsed == 0 {
		switch state {
		case feed.CollapsedEntryState:
			linesUsed = v.redrawCollapsedState(
				width, startLine, item, itemFgColor, metadataFgColor)
		case feed.StandardEntryState:
			if startLine <= 2 {
				state = feed.CollapseEntryState(state)
				continue
			}

			linesUsed = v.redrawStandardState(
				width, startLine, item, itemFgColor, metadataFgColor)
		case feed.ExpandedEntryState:
			if startLine <= 3 {
				state = feed.CollapseEntryState(state)
				continue
			}

			linesUsed = v.redrawExpandedState(
				width, startLine, item, itemFgColor, metadataFgColor)
		default:
			panic("Unhandled entry state")
		}
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

	linesUsableByEntries := (h - 4)
	// The largest possible number of items in view.
	numItemsInView := linesUsableByEntries
	if len(v.itemList) < numItemsInView {
		// Which may be even smaller if there are less items available.
		numItemsInView = len(v.itemList)
	}

	// XXX Copying the entire item list is "easy", but potentially pretty slow.
	itemListCopy := make([]feed.RssEntry, numItemsInView)
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
	for rssEntryLine > 1 {
		if itemIndex >= numItemsInView {
			break
		}
		rssEntryLine -= v.redrawRssItem(w, rssEntryLine, itemIndex, inputItemIndex,
			inputMode, itemListCopy)
		itemIndex += 1
	}

	// TODO(smklein): Might be fun to have an "all items gone" image?
	v.inputManager.SetLastSeenNumItems(itemIndex)

	v.redrawUserInput(w, userInputLine, inputString)
	v.redrawStatus(w, statusLine, statusString, statusColor)

	tb.Flush()
}
