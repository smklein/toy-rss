package storage

import (
	"errors"
	"sync"

	tb "github.com/nsf/termbox-go"
)

type ChannelInfo struct {
	ChannelColor tb.Attribute
}

type ViewStorage struct {
	filename        string
	itemBufferedCap int

	itemLock sync.RWMutex
	itemList []*RssEntry

	channelInfoLock sync.RWMutex
	channelInfoMap  map[string]*ChannelInfo /* Title --> Info */

}

func MakeViewStorage(key string, cap int) *ViewStorage {
	// TODO(smklein): Read storage from file if it exists
	return &ViewStorage{
		filename:        key,
		itemBufferedCap: cap,
		itemList:        make([]*RssEntry, 0),
		channelInfoMap:  make(map[string]*ChannelInfo),
	}
}

func (s *ViewStorage) GetCopyOfSomeItems(n int) []RssEntry {
	s.itemLock.RLock()
	defer s.itemLock.RUnlock()

	if len(s.itemList) < n {
		n = len(s.itemList)
	}
	itemListCopy := make([]RssEntry, n)
	for i := range itemListCopy {
		itemListCopy[i] = *s.itemList[i]
	}
	return itemListCopy
}

func (s *ViewStorage) AddItem(item *RssEntry) {
	s.itemLock.Lock()
	defer s.itemLock.Unlock()
	// Place new items at the BACK of the itemList.
	s.itemList = append(s.itemList, item)
	if len(s.itemList) > s.itemBufferedCap {
		s.itemList = s.itemList[1:]
	}
}

func (s *ViewStorage) DeleteItem(index int) error {
	s.itemLock.Lock()
	defer s.itemLock.Unlock()
	if index < 0 || len(s.itemList) <= index {
		return errors.New("DeleteItem: Attempting to access out of range item")
	}
	s.itemList = append(s.itemList[:index], s.itemList[index+1:]...)
	return nil
}

func (s *ViewStorage) ChangeColor(index int) error {
	s.itemLock.RLock()
	defer s.itemLock.RUnlock()
	s.channelInfoLock.Lock()
	defer s.channelInfoLock.Unlock()

	if index < 0 || len(s.itemList) <= index {
		return errors.New("ChangeColor: Attempting to access out of range item")
	}
	if chInfo, ok := s.channelInfoMap[s.itemList[index].FeedTitle]; ok {
		// TODO(smklein): Better color selection than cycling!
		chInfo.ChannelColor = (chInfo.ChannelColor + 1) % tb.ColorWhite
		return nil
	} else {
		return errors.New("ChangeColor: Channel not registered in channelInfoMap")
	}
}

func (s *ViewStorage) ChangeItemState(index int, expand bool) (RssEntryState, string) {
	s.itemLock.Lock()
	defer s.itemLock.Unlock()
	if index >= len(s.itemList) {
		return 0, ""
	}

	if expand {
		s.itemList[index].State = ExpandEntryState(s.itemList[index].State)
	} else {
		s.itemList[index].State = CollapseEntryState(s.itemList[index].State)
	}

	if s.itemList[index].State == BrowserEntryState {
		// Edge case: When opening in browser, set to "non-browser"
		// state immediately following the state change.
		s.itemList[index].State = CollapseEntryState(s.itemList[index].State)
		return BrowserEntryState, s.itemList[index].URL
	}

	return s.itemList[index].State, ""
}

func (s *ViewStorage) GetChannelInfo(title string) *ChannelInfo {
	s.channelInfoLock.RLock()
	defer s.channelInfoLock.RUnlock()
	info := s.channelInfoMap[title]
	return info
}

func (s *ViewStorage) SetChannelInfo(title string, info *ChannelInfo) {
	s.channelInfoLock.Lock()
	defer s.channelInfoLock.Unlock()
	s.channelInfoMap[title] = info
}
