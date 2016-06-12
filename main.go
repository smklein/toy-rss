package main

import (
	"log"
	"os"
	"sync"

	"github.com/smklein/toy-rss/feed"
	"github.com/smklein/toy-rss/view"
)

func handleFeed(feed *feed.Feed, itemPipe chan *feed.RssEntry, newItemRequest chan *feed.RssEntry) {
	log.Println("HANDLE FEED: ", feed.GetTitle())
	numReceived := 0
	for {
		item, ok := <-itemPipe
		if !ok {
			// TODO(smklein): handle removing feed here
			return
		}
		numReceived++
		log.Println("   ", numReceived, item.ItemTitle)
		newItemRequest <- item
	}
}

func addFeed(URL string, newItemRequest chan *feed.RssEntry) (feed.FeedInterface, error) {
	// TODO store the feed somewhere, so we can disable it...
	feed := &feed.Feed{}
	itemPipe, err := feed.Start(URL)
	if err != nil {
		return nil, err
	}
	go handleFeed(feed, itemPipe, newItemRequest)
	return feed, nil
}

// Set up logging info.
// We're going to use the screen, so we should log to a separate file.
func initLog() *os.File {
	// Make the log show the calling filename, line number.
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	f, err := os.OpenFile("toy_rss_log_output.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	log.SetOutput(f)
	return f
}

func main() {
	logFile := initLog()
	defer logFile.Close()

	feedMap := make(map[string]feed.FeedInterface)
	// Serialize new items
	newItemRequest := make(chan *feed.RssEntry, 100)
	newFeedRequest := make(chan string)

	// DeathCountWg can be used by main.
	// Add one if you need to run something before you die.
	var deathWg sync.WaitGroup
	// Waiting for
	// 1) tb to close in view.
	deathWg.Add(1)

	v := &view.View{}
	v.Start(newItemRequest, newFeedRequest, &deathWg)
	for {
		select {
		case newURL := <-newFeedRequest:
			f, err := addFeed(newURL, newItemRequest)
			if err != nil {
				v.SetStatus(view.StatusMsgStruct{err.Error(), view.StatusError})
			} else {
				v.SetStatus(view.StatusMsgStruct{"Added Feed [" + f.GetTitle() + "]", view.StatusSuccess})
				v.AddChannelInfo(f.GetTitle())
				feedMap[newURL] = f
			}
			v.Redraw()
		case <-v.GetChanExitRequest():
			deathWg.Wait()
			return
		}
	}
}
