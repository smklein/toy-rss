package storage

import (
	"path"

	"github.com/smklein/toy-rss/agingmap"
)

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

type FeedStorage struct {
	filename string
	Amap     agingmap.AgingMapInterface
}

func MakeFeedStorage(filename string, cap int) *FeedStorage {
	s := &FeedStorage{
		filename: "data/" + path.Clean(filename),
		Amap:     &agingmap.AgingMap{},
	}
	s.Amap.Init(cap)
	s.LoadFromStorage()
	return s
}

func (s *FeedStorage) Add(key, value string) {
	s.Amap.Add(key, value)
	s.DumpToStorage()
}

func (s *FeedStorage) Get(key string) string {
	return s.Amap.Get(key)
}
