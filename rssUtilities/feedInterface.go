package rssUtilities

// TODO(smklein): Decouple this from the interface.
import "github.com/SlyMarbo/rss"

// FeedInterface decouples the "RSS/Atom" interface from our implementation.
type FeedInterface interface {
	Start(URL string) chan rss.Item
	GetTitle() string
	End()
}
