package storage

import (
	"time"

	"github.com/smklein/toy-rss/agingmap"
)

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

type FeedStorage struct {
	filename string
	amap     agingmap.AgingMapInterface
}

func MakeFeedStorage(key string, cap int) *FeedStorage {
	// TODO(smklein): Read storage from file if it exists
	s := &FeedStorage{
		filename: key,
		amap:     &agingmap.AgingMap{},
	}
	s.amap.Init(100)
	return s
}

func (s *FeedStorage) Add(key, value string) {
	s.amap.Add(key, value)
}
func (s *FeedStorage) Get(key string) string {
	return s.amap.Get(key)
}
func (s *FeedStorage) Remove(key string) string {
	return s.amap.Remove(key)
}
