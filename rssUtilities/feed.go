package rssUtilities

import (
	"log"
	"time"

	"github.com/SlyMarbo/rss"
)

// Feed implements the FeedInterface
type Feed struct {
	itemPipe    chan rss.Item
	Title       string
	URL         string
	initialized bool
	disabled    bool
}

func (f *Feed) getURL() string {
	return f.URL
}

// GetTitle does what you would expect.
func (f *Feed) GetTitle() string {
	return f.Title
}

func (f *Feed) doFeed(initPipe chan bool) {
	defer close(f.itemPipe)
	defer close(initPipe)

	// We are going to handle a database of items ourselves.
	rss.CacheParsedItemIDs(false)

	// Avoid duplicates, up to a limit, but let expirations occur.
	duplicatesMap := AgingMap{}
	duplicatesMap.Init(100)

	for {
		if f.disabled {
			return
		}

		rssFeed, err := rss.Fetch(f.URL)
		if err != nil {
			log.Fatal(err)
			f.disabled = true
			return
		}
		// TODO(smklein): Set other non-Item features here
		f.Title = rssFeed.Title

		if !f.initialized {
			initPipe <- true
			f.initialized = true
		}
		for _, item := range rssFeed.Items {
			if duplicatesMap.Get(item.ID) == "" {
				duplicatesMap.Add(item.ID, item.ID)
				// Only place items in the itemPipe if they are not visible in
				// the duplicatesMap.
				f.itemPipe <- *item
			}
		}

		// TODO(smklein): Make this more conservative (rssFeed.Refresh)
		<-time.After(time.Duration(6 * time.Second))
	}
}

// Start begins the feed. It will continue to retrieve items from the URL at an
// interval. Although no formal guarantees are made regarding duplicates,
// generally, it can be assumed that duplicates will not be sent on the pipe
// for a while.
func (f *Feed) Start(URL string) chan rss.Item {
	log.Println("Start: ", URL)
	f.URL = URL
	f.itemPipe = make(chan rss.Item, 20)

	initPipe := make(chan bool)
	go f.doFeed(initPipe)
	<-initPipe
	// TODO better error handling?
	if f.disabled {
		return nil
	}
	return f.itemPipe
}

// End terminates the feed.
func (f *Feed) End() {
	f.disabled = true
}
