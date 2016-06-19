package storage

/*
 * This file contains all the on-disk saved structures, and mechanisms
 * to dump to the disk itself.
 */

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"time"

	tb "github.com/nsf/termbox-go"
	"github.com/smklein/toy-rss/agingmap"
)

type ChannelInfo struct {
	ChannelColor tb.Attribute
}

type SavedViewStorage struct {
	ItemList       []*RssEntry
	ChannelInfoMap map[string]*ChannelInfo /* Title --> Info */
}

// RssEntryState represents the viewing status of the entry.
type RssEntryState uint8

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

// VIEW STORAGE

func (s *ViewStorage) LoadFromStorage() (loaded bool) {
	if f, err := ioutil.ReadFile(s.filename); err == nil {
		log.Println("Read filename: ", s.filename)
		err = json.Unmarshal(f, &s.saved)
		if err != nil {
			panic(err.Error())
		}
		for i := range s.saved.ItemList {
			// Reset all items to their collapsed state.
			s.saved.ItemList[i].State = CollapsedEntryState
		}
		return true
	} else {
		return false
	}
}

func (s *ViewStorage) DumpToStorage() {
	b, err := json.Marshal(s.saved)
	if err != nil {
		panic(err)
	}

	// XXX Is this racy? Should I actually make a temporary file?
	tempFileName := s.filename + "_TEMP"
	err = ioutil.WriteFile(tempFileName, b, 0644)
	if err != nil {
		panic(err)
	}
	err = os.Rename(tempFileName, s.filename)
}

// FEED STORAGE

func (s *FeedStorage) LoadFromStorage() (loaded bool) {
	if f, err := ioutil.ReadFile(s.filename); err == nil {
		// Feed exists in storage; re-open.
		log.Println("FeedStorage Loading: ", s.filename)
		var pairs []agingmap.KeyValuePair
		err = json.Unmarshal(f, &pairs)
		if err != nil {
			panic(err.Error())
		}
		for i := range pairs {
			s.Amap.Add(pairs[i].Key, pairs[i].Value)
		}
		return true
	} else {
		return false
	}
}

func (s *FeedStorage) DumpToStorage() {
	// It sucks to try to marshal the amap -- it contains a linked list, with
	// internal data structures that are inaccessible. Instead, we marshal a
	// list of kv pairs.
	b, err := json.Marshal(s.Amap.Serialize())
	if err != nil {
		panic(err)
	}

	// XXX Is this racy? Should I actually make a temporary file?
	tempfilename := s.filename + "_TEMP"
	err = ioutil.WriteFile(tempfilename, b, 0644)
	if err != nil {
		panic(err)
	}
	err = os.Rename(tempfilename, s.filename)
}
