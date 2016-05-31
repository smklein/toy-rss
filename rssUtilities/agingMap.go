package rssUtilities

import (
	"container/list"
	"log"
	"sync"
)

type keyValuePair struct {
	key   string
	value string
}

const threadSafe = false

// AgingMap implements the AgingMap interface.
// This is my first go at an implementation.
type AgingMap struct {
	internalMap  map[string]*list.Element
	internalList list.List
	maxElements  int
	rwLock       sync.RWMutex
}

func (am *AgingMap) init(max int) {
	am.internalMap = make(map[string]*list.Element)
	am.internalList.Init()
	am.maxElements = max
	if am.maxElements <= 0 {
		log.Fatal("Empty map")
	}
}

func (am *AgingMap) add(key string, value string) {
	if threadSafe {
		am.rwLock.Lock()
		defer am.rwLock.Unlock()
	}
	if element, ok := am.internalMap[key]; ok {
		// Refresh the age.
		am.internalList.MoveToFront(element)
		return
	}

	if len(am.internalMap) >= am.maxElements {
		// Make room for the new element.
		kvp := am.internalList.Remove(am.internalList.Back()).(keyValuePair)
		delete(am.internalMap, kvp.key)
	}

	am.internalMap[key] = am.internalList.PushFront(keyValuePair{key, value})
}

func (am *AgingMap) get(key string) string {
	if threadSafe {
		am.rwLock.RLock()
		defer am.rwLock.RUnlock()
	}
	element := am.internalMap[key]
	if element == nil {
		return ""
	}

	kvp := element.Value.(keyValuePair)
	return kvp.value
}

func (am *AgingMap) remove(key string) string {
	if threadSafe {
		am.rwLock.Lock()
		defer am.rwLock.Unlock()
	}
	element := am.internalMap[key]
	if element == nil {
		return ""
	}

	kvp := element.Value.(keyValuePair)
	delete(am.internalMap, key)
	am.internalList.Remove(element)
	return kvp.value
}
