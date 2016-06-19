package storage

import (
	"path"

	"github.com/smklein/toy-rss/agingmap"
)

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
