package storage

import (
	"errors"
	"path"
	"sync"

	tb "github.com/nsf/termbox-go"
)

type ViewStorage struct {
	filename        string
	itemBufferedCap int

	itemLock        sync.RWMutex
	channelInfoLock sync.RWMutex

	saved *SavedViewStorage
}

func MakeViewStorage(key string, cap int) *ViewStorage {
	s := &ViewStorage{
		filename:        "data/" + path.Clean(key),
		itemBufferedCap: cap,
	}
	if !s.LoadFromStorage() {
		s.saved = &SavedViewStorage{
			ItemList:       make([]*RssEntry, 0),
			ChannelInfoMap: make(map[string]*ChannelInfo),
		}
	}

	return s
}

func (s *ViewStorage) GetCopyOfSomeItems(n int) []RssEntry {
	s.itemLock.RLock()
	defer s.itemLock.RUnlock()

	if len(s.saved.ItemList) < n {
		n = len(s.saved.ItemList)
	}
	itemListCopy := make([]RssEntry, n)
	for i := range itemListCopy {
		itemListCopy[i] = *s.saved.ItemList[i]
	}
	return itemListCopy
}

func (s *ViewStorage) AddItem(item *RssEntry) {
	s.itemLock.Lock()
	defer s.itemLock.Unlock()
	// Place new items at the BACK of the itemList.
	s.saved.ItemList = append(s.saved.ItemList, item)
	if len(s.saved.ItemList) > s.itemBufferedCap {
		s.saved.ItemList = s.saved.ItemList[1:]
	}
	s.DumpToStorage()
}

func (s *ViewStorage) DeleteItem(index int) error {
	s.itemLock.Lock()
	defer s.itemLock.Unlock()
	if index < 0 || len(s.saved.ItemList) <= index {
		return errors.New("DeleteItem: Attempting to access out of range item")
	}
	s.saved.ItemList = append(s.saved.ItemList[:index], s.saved.ItemList[index+1:]...)
	s.DumpToStorage()
	return nil
}

func (s *ViewStorage) ChangeColor(index int) error {
	s.itemLock.RLock()
	defer s.itemLock.RUnlock()
	s.channelInfoLock.Lock()
	defer s.channelInfoLock.Unlock()

	if index < 0 || len(s.saved.ItemList) <= index {
		return errors.New("ChangeColor: Attempting to access out of range item")
	}
	if chInfo, ok := s.saved.ChannelInfoMap[s.saved.ItemList[index].FeedTitle]; ok {
		// TODO(smklein): Better color selection than cycling!
		chInfo.ChannelColor = (chInfo.ChannelColor + 1) % tb.ColorWhite
		s.DumpToStorage()
		return nil
	} else {
		return errors.New("ChangeColor: Channel not registered in channelInfoMap")
	}
}

func (s *ViewStorage) ChangeItemState(index int, expand bool) (RssEntryState, string) {
	s.itemLock.Lock()
	defer s.itemLock.Unlock()
	if index >= len(s.saved.ItemList) {
		return 0, ""
	}

	if expand {
		s.saved.ItemList[index].State = ExpandEntryState(s.saved.ItemList[index].State)
	} else {
		s.saved.ItemList[index].State = CollapseEntryState(s.saved.ItemList[index].State)
	}

	if s.saved.ItemList[index].State == BrowserEntryState {
		// Edge case: When opening in browser, set to "non-browser"
		// state immediately following the state change.
		s.saved.ItemList[index].State = CollapseEntryState(s.saved.ItemList[index].State)
		return BrowserEntryState, s.saved.ItemList[index].URL
	}

	return s.saved.ItemList[index].State, ""
}

func (s *ViewStorage) GetChannelInfo(title string) *ChannelInfo {
	s.channelInfoLock.RLock()
	defer s.channelInfoLock.RUnlock()
	info := s.saved.ChannelInfoMap[title]
	return info
}

func (s *ViewStorage) SetChannelInfo(title string, info *ChannelInfo) {
	s.channelInfoLock.Lock()
	defer s.channelInfoLock.Unlock()
	s.saved.ChannelInfoMap[title] = info
	s.DumpToStorage()
}
