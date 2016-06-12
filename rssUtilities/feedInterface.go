package rssUtilities

import "time"

type RssEntryState uint8

const (
	standardEntryState RssEntryState = iota
	collapsedEntryState
	expandedEntryState
)

func ExpandEntryState(state RssEntryState) RssEntryState {
	switch state {
	case collapsedEntryState:
		return standardEntryState
	default:
		return expandedEntryState
	}
}

func CollapseEntryState(state RssEntryState) RssEntryState {
	switch state {
	case expandedEntryState:
		return standardEntryState
	default:
		return collapsedEntryState
	}
}

// RssEntry represents OUR version of an entry.
// It's in a form that can be easily dumped to the view.
type RssEntry struct {
	FeedTitle   string
	ItemTitle   string
	ItemSummary string
	ItemContent string
	URL         string
	ItemDate    time.Time
	State       RssEntryState
}

// Feed implements the FeedInterface
type Feed struct {
	itemPipe    chan *RssEntry
	Title       string
	URL         string
	initialized bool
	disabled    bool
}

// FeedInterface decouples the "RSS/Atom" interface from our implementation.
type FeedInterface interface {
	Start(URL string) (chan *RssEntry, error)
	GetTitle() string
	End()
}
