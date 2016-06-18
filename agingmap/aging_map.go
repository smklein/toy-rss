package agingmap

import (
	"container/list"
	"log"
	"sync"
)

const threadSafe = false

// AgingMap implements the AgingMap interface.
// This is my first attempt at an implementation.
type AgingMap struct {
	internalMap  map[string]*list.Element
	internalList list.List
	maxElements  int
	rwLock       sync.RWMutex
}

func (am *AgingMap) Serialize() []KeyValuePair {
	pairs := make([]KeyValuePair, 0, am.maxElements)

	am.rwLock.RLock()
	defer am.rwLock.RUnlock()
	e := am.internalList.Front()
	for e != nil {
		pairs = append(pairs, e.Value.(KeyValuePair))
		e = e.Next()
	}

	return pairs
}

// Init initializes the map with a maximum size.
func (am *AgingMap) Init(max int) {
	am.internalMap = make(map[string]*list.Element)
	am.internalList.Init()
	am.maxElements = max
	if am.maxElements <= 0 {
		log.Fatal("Empty map")
	}
}

// Add places the key/value pair inside the map.
// It refreshes the lifetime of key if it already exists
// in the map.
func (am *AgingMap) Add(key string, value string) {
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
		kvp := am.internalList.Remove(am.internalList.Back()).(KeyValuePair)
		delete(am.internalMap, kvp.Key)
	}

	am.internalMap[key] = am.internalList.PushFront(KeyValuePair{key, value})
}

// Get retrieves the value for the key in the map (or returns "" if it doesn't
// exist).
func (am *AgingMap) Get(key string) string {
	if threadSafe {
		am.rwLock.RLock()
		defer am.rwLock.RUnlock()
	}
	element := am.internalMap[key]
	if element == nil {
		return ""
	}

	kvp := element.Value.(KeyValuePair)
	return kvp.Value
}

// Remove deletes the key/value pair from the map, and returns the value.
func (am *AgingMap) Remove(key string) string {
	if threadSafe {
		am.rwLock.Lock()
		defer am.rwLock.Unlock()
	}
	element := am.internalMap[key]
	if element == nil {
		return ""
	}

	kvp := element.Value.(KeyValuePair)
	delete(am.internalMap, key)
	am.internalList.Remove(element)
	return kvp.Value
}
