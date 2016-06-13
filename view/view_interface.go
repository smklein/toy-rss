package view

import (
	"sync"

	"github.com/smklein/toy-rss/storage"
)

// StatusType modifies the statusMsg color.
type StatusType uint8

const (
	// StatusError indicates an error.
	StatusError StatusType = iota
	// StatusSuccess indicates a success.
	StatusSuccess
	// StatusInfo indicates info.
	StatusInfo
)

type StatusMsgStruct struct {
	Message string
	Type    StatusType
}

type ViewInterface interface {
	// Initialization.
	Start(newItemPipe chan *storage.RssEntry, newFeedRequest chan string, deathWg *sync.WaitGroup)

	// Methods relating to drawing.
	SetStatus(status StatusMsgStruct)
	Redraw()

	// Method which pairs additional information with a single channel.
	AddChannelInfo(title string)

	// Methods operating on items.
	DeleteItem(index int)
	ChangeColor(index int)
	CollapseItem(index int)
	ExpandItem(index int)

	// Plumbs the exit channel.
	GetChanExitRequest() chan bool
}

// Returns a particular implementation of this interface.
func GetView() ViewInterface {
	return &view{}
}
