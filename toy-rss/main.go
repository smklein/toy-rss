package main

import (
	"flag"
	"log" // TODO(smklein): Log vs fmt?
	"os"
	"sync"

	"github.com/SlyMarbo/rss"
	util "github.com/smklein/toy-rss/rssUtilities"
)

var feedURLs = [...]string{
	"http://feeds.arstechnica.com/arstechnica/index",
	"https://news.ycombinator.com/rss",
	//"https://www.reddit.com/.rss",
}

var flagURL string

// Init is a special function, which is called before main.
// TODO(smklein): Probably change the default args here. flagURL no longer
// seems super useful.
func init() {
	const (
		defaultURL = "https://www.reddit.com/.rss"
		usage      = "URL to be accessed"
	)
	flag.StringVar(&flagURL, "URL", defaultURL, usage)
	flag.StringVar(&flagURL, "u", defaultURL, usage+" (shorthand)")
}

func handleFeed(feed *util.Feed, itemPipe chan rss.Item) {
	log.Println("HANDLE FEED: ", feed.GetTitle())
	numReceived := 0
	for {
		item := <-itemPipe
		numReceived++
		log.Println("   ", numReceived, item.Title)
	}
}

func addFeed(URL string) util.FeedInterface {
	// TODO store the feed somewhere, so we can disable it...
	feed := &util.Feed{}
	itemPipe := feed.Start(URL)

	if itemPipe == nil {
		return nil
	}
	go handleFeed(feed, itemPipe)
	return feed
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

	flag.Parse()
	log.Println("URL: " + flagURL)

	//	am := &util.AgingMap{}
	//	am.Init(100)

	feedMap := make(map[string]util.FeedInterface)

	// DeathCountWg can be used by main.
	// Add one if you need to run something before you die.
	var deathWg sync.WaitGroup
	// Waiting for
	// 1) tb to close in view.
	deathWg.Add(1)

	v := &util.View{}
	v.Start(&deathWg)

	for {
		select {
		case newURL := <-v.NewFeedRequest:
			if f := addFeed(newURL); f != nil {
				feedMap[newURL] = f
				log.Println("SUCCESS, Feed started: ", f.GetTitle())
			} else {
				log.Println("!!! ERROR, could not add: ", newURL)
			}
		case <-v.ExitRequest:
			deathWg.Wait()
			return
		}
	}
}
