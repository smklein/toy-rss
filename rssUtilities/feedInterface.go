package rssUtilities

import "time"

type RssEntryState uint8

const (
	collapsedEntryState RssEntryState = iota
	standardEntryState
	expandedEntryState
	browserEntryState /* Temporary state; gets lowered immediately */
)

func ExpandEntryState(state RssEntryState) RssEntryState {
	switch state {
	case collapsedEntryState:
		return standardEntryState
	case standardEntryState:
		return expandedEntryState
	default:
		return browserEntryState
	}
}

func CollapseEntryState(state RssEntryState) RssEntryState {
	switch state {
	case browserEntryState:
		return expandedEntryState
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
