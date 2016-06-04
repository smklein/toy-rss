package main

import (
	"log"
	"os"
	"sync"

	util "github.com/smklein/toy-rss/rssUtilities"
)

func handleFeed(feed *util.Feed, itemPipe chan *util.RssEntry, newItemRequest chan *util.RssEntry) {
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

func addFeed(URL string, newItemRequest chan *util.RssEntry) (util.FeedInterface, error) {
	// TODO store the feed somewhere, so we can disable it...
	feed := &util.Feed{}
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

	feedMap := make(map[string]util.FeedInterface)
	// Serialize new items
	newItemRequest := make(chan *util.RssEntry, 100)

	// DeathCountWg can be used by main.
	// Add one if you need to run something before you die.
	var deathWg sync.WaitGroup
	// Waiting for
	// 1) tb to close in view.
	deathWg.Add(1)

	v := &util.View{}
	v.Start(newItemRequest, &deathWg)

	for {
		select {
		case newURL := <-v.NewFeedRequest:
			f, err := addFeed(newURL, newItemRequest)
			if err != nil {
				v.SetStatusMsg(err.Error(), util.StatusError)
			} else {
				v.SetStatusMsg("Added Feed ["+f.GetTitle()+"]", util.StatusSuccess)
				feedMap[newURL] = f
			}
			v.RedrawRequest <- true
		case <-v.ExitRequest:
			deathWg.Wait()
			return
		}
	}
}
