package view

import (
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
	"unicode/utf8"

	tb "github.com/nsf/termbox-go"
	"github.com/smklein/toy-rss/storage"
)

// TODO(smklein): Use default colors
var defaultFeedURLs = [...]string{
	"http://feeds.arstechnica.com/arstechnica/index",
	"https://news.ycombinator.com/rss",
	"https://lwn.net/headlines/rss",
	"http://feeds.feedburner.com/linuxjournalcom?format=xml",
	"http://preshing.com/feed",
	//"https://www.reddit.com/.rss",
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// View gives access to the user.
type view struct {
	// Rss Entry items
	storage *storage.ViewStorage

	// Status Message
	status StatusMsgStruct

	// Synchronization tools
	viewLock sync.RWMutex
	deathWg  *sync.WaitGroup

	// Mechanism to interact with input (presumeably keyboard)
	inputManager *InputManager

	// Blocks redraw loop, closes termbox, opens browser.
	openArticleRequest chan string

	// When this is closed, GTFO.
	exitRequest chan bool

	// INCOMING
	setStatusRequest   chan StatusMsgStruct
	redrawRequest      chan bool
	deleteItemRequest  chan int /* Item Index */
	changeColorRequest chan int /* Item Index */
}

// Start launches the view. Undefined to call multiple times.
func (v *view) Start(newItemPipe chan *storage.RssEntry, newFeedRequest chan string, deathWg *sync.WaitGroup) {
	log.Println("View Start called")
	v.viewLock.Lock()
	// TODO(smklein): This size should be configurable.
	v.storage = storage.MakeViewStorage("VIEW_STORAGE", 200)

	v.deathWg = deathWg

	v.inputManager = new(InputManager)
	v.openArticleRequest = make(chan string)
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

func (v *view) GetChanExitRequest() chan bool {
	return v.exitRequest
}
func (v *view) DeleteItem(i int) {
	v.deleteItemRequest <- i
}
func (v *view) SetStatus(status StatusMsgStruct) {
	v.setStatusRequest <- status
}
func (v *view) ChangeColor(i int) {
	v.changeColorRequest <- i
}
func (v *view) Redraw() {
	v.redrawRequest <- true
}
func (v *view) AddChannelInfo(title string) {
	info := &storage.ChannelInfo{
		ChannelColor: fgColor,
	}
	if v.storage.GetChannelInfo(title) == nil {
		v.storage.SetChannelInfo(title, info)
	}
}
func (v *view) CollapseItem(index int) {
	v.storage.ChangeItemState(index, false /* Expanding? */)
}
func (v *view) ExpandItem(index int) {
	newState, url := v.storage.ChangeItemState(index, true /* Expanding? */)

	// TODO(smklein): Should probably customize this command...
	// tmux split-window -h "w3m -dump test_server/test_files/birds.html | less"
	if newState == storage.BrowserEntryState {
		//cmd := exec.Command("w3m -dump", url)
		log.Println("Would have shown: ", url)
		v.openArticleRequest <- url

		/*
			cmd := exec.Command("tmux", "split-window", "-h", "\"w3m -dump test_server/test_files/birds.html | less\"")
			err := cmd.Start()
			if err != nil {
				log.Fatal("Start error: ", err)
			}
			err = cmd.Wait()
			if err != nil {
				log.Fatal("Wait error: ", err)
			}
		*/
	}
}

func (v *view) listUpdater(newItemPipe chan *storage.RssEntry) {
	for {
		item := <-newItemPipe
		v.storage.AddItem(item)
		v.redrawRequest <- true
	}
}

func (v *view) setStatusMsg(status StatusMsgStruct) {
	v.viewLock.Lock()
	v.status = status
	v.viewLock.Unlock()
}

func (v *view) tryToDeleteItem(index int) {
	if err := v.storage.DeleteItem(index); err != nil {
		v.setStatusMsg(StatusMsgStruct{err.Error(), StatusError})
		return
	}
}

func (v *view) tryToChangeItemColor(index int) {
	if err := v.storage.ChangeColor(index); err != nil {
		v.setStatusMsg(StatusMsgStruct{err.Error(), StatusError})
		return
	}
}

func (v *view) drawLoop() {
	// Initialize termbox-go
	check(tb.Init())

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
		case url := <-v.openArticleRequest:
			tb.Close()

			cmd := exec.Command("w3m", url)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				log.Fatal("W3M error: ", err)
			}

			tb.Init()
		case <-v.exitRequest:
			tb.Close()
			v.deathWg.Done()
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

func (v *view) redrawCollapsedState(width, startLine int, item storage.RssEntry, itemFgColor, metadataFgColor tb.Attribute) int {
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

func (v *view) redrawStandardState(width, startLine int, item storage.RssEntry, itemFgColor, metadataFgColor tb.Attribute) int {
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

func (v *view) redrawExpandedState(width, startLine int, item storage.RssEntry, itemFgColor, metadataFgColor tb.Attribute) int {
	// [Time] [Feed Title]
	//   [ItemTitle]
	//   [URL]
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
			contents: append([]rune("  "), []rune(item.URL)...),
			maxLen:   120,
			color:    itemFgColor,
		},
	})

	return 3
}

func (v *view) redrawRssItem(width, startLine, itemIndex, inputItemIndex int, inputMode InputType, itemListCopy []storage.RssEntry) int {
	item := itemListCopy[itemIndex]

	itemFgColor := fgColor
	metadataFgColor := fgColor

	if inputMode == RssSelectionMode && inputItemIndex == itemIndex {
		itemFgColor = tb.ColorBlue | tb.AttrUnderline
	}
	if chInfo := v.storage.GetChannelInfo(item.FeedTitle); chInfo != nil {
		metadataFgColor = chInfo.ChannelColor
	}

	linesUsed := 0

	// TODO add mechanism to "downgrade" size if it wouldn't fit on the top of the screen.
	state := item.State
	for linesUsed == 0 {
		switch state {
		case storage.CollapsedEntryState:
			linesUsed = v.redrawCollapsedState(
				width, startLine, item, itemFgColor, metadataFgColor)
		case storage.StandardEntryState:
			if startLine <= 2 {
				state = storage.CollapseEntryState(state)
				continue
			}

			linesUsed = v.redrawStandardState(
				width, startLine, item, itemFgColor, metadataFgColor)
		case storage.ExpandedEntryState:
			if startLine <= 3 {
				state = storage.CollapseEntryState(state)
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

func (v *view) redrawStatus(width, line int, statusString string, statusColor tb.Attribute) {
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

func (v *view) redrawUserInput(width, line int, inputString string) {
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

func (v *view) redrawAll() {
	// TODO(smklein): Only clear parts of screen that need re-drawing...
	tb.Clear(tb.ColorDefault, tb.ColorDefault)
	w, h := tb.Size()

	fgColor = tb.ColorGreen

	// Lock and copy anything we want to display.
	v.viewLock.RLock()
	statusString := v.status.Message
	statusColor := getStatusColor(v.status.Type)
	v.viewLock.RUnlock()

	// The largest possible number of items in view.
	linesUsableByEntries := (h - 4)
	if linesUsableByEntries < 0 {
		linesUsableByEntries = 0
	}
	itemListCopy := v.storage.GetCopyOfSomeItems(linesUsableByEntries)
	numItemsInView := len(itemListCopy)

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
