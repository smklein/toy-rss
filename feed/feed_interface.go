package feed

import "time"

// RssEntryState represents the viewing status of the entry.
type RssEntryState uint8

const (
	CollapsedEntryState RssEntryState = iota
	StandardEntryState
	ExpandedEntryState
	BrowserEntryState /* Temporary state; should get lowered immediately */
)

// ExpandEntryState returns the next largest entry state.
func ExpandEntryState(state RssEntryState) RssEntryState {
	switch state {
	case CollapsedEntryState:
		return StandardEntryState
	case StandardEntryState:
		return ExpandedEntryState
	default:
		return BrowserEntryState
	}
}

// CollapseEntryState returns the next smallest entry state.
func CollapseEntryState(state RssEntryState) RssEntryState {
	switch state {
	case BrowserEntryState:
		return ExpandedEntryState
	case ExpandedEntryState:
		return StandardEntryState
	default:
		return CollapsedEntryState
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

// Feed implements the FeedInterface.
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
