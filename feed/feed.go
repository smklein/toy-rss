package feed

import (
	"errors"
	"log"
	"time"

	"github.com/SlyMarbo/rss"
	"github.com/smklein/toy-rss/agingmap"
)

// GetTitle does what you would expect.
func (f *Feed) GetTitle() string {
	return f.Title
}

func (f *Feed) doFeed(initPipe chan error) {
	defer close(f.itemPipe)
	defer close(initPipe)

	// We are going to handle a database of items ourselves.
	rss.CacheParsedItemIDs(false)

	// Avoid duplicates, up to a limit, but let expirations occur.
	duplicatesMap := agingmap.AgingMap{}
	duplicatesMap.Init(100)

	for {
		if f.disabled {
			return
		}

		rssFeed, err := rss.Fetch(f.URL)
		if err != nil {
			log.Println(err)
			initPipe <- errors.New("Fetching RSS feed failed: " + err.Error())
			f.disabled = true
			return
		}
		// TODO(smklein): Set other non-Item features here
		f.Title = rssFeed.Title

		if !f.initialized {
			initPipe <- nil
			f.initialized = true
		}
		for _, item := range rssFeed.Items {
			if duplicatesMap.Get(item.ID) == "" {
				duplicatesMap.Add(item.ID, item.ID)
				// Only place items in the itemPipe if they are not visible in
				// the duplicatesMap.
				newItem := &RssEntry{}
				newItem.FeedTitle = f.Title
				newItem.ItemTitle = item.Title
				newItem.ItemSummary = item.Summary
				newItem.ItemContent = item.Content
				newItem.URL = item.Link
				newItem.ItemDate = item.Date
				f.itemPipe <- newItem
			}
		}

		// TODO(smklein): Make this more conservative (rssFeed.Refresh)
		<-time.After(time.Duration(20 * time.Second))
	}
}

// Start begins the feed. It will continue to retrieve items from the URL at an
// interval. Although no formal guarantees are made regarding duplicates,
// generally, it can be assumed that duplicates will not be sent on the pipe
// for a while.
func (f *Feed) Start(URL string) (chan *RssEntry, error) {
	log.Println("Start: ", URL)
	f.URL = URL
	f.itemPipe = make(chan *RssEntry, 20)

	initPipe := make(chan error)
	go f.doFeed(initPipe)
	err := <-initPipe
	if err != nil {
		f.disabled = true
		return nil, err
	}
	return f.itemPipe, nil
}

// End terminates the feed.
func (f *Feed) End() {
	f.disabled = true
}
