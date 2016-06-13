package feed

import "github.com/smklein/toy-rss/storage"

// Feed implements the FeedInterface.
type Feed struct {
	itemPipe    chan *storage.RssEntry
	Title       string
	URL         string
	initialized bool
	disabled    bool
}

// FeedInterface decouples the "RSS/Atom" interface from our implementation.
type FeedInterface interface {
	// The channel returned from "Start" will never return duplicate entries.
	Start(URL string) (chan *storage.RssEntry, error)
	GetTitle() string
	End()
}
