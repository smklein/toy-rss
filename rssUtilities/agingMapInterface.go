package rssUtilities

// AgingMapInterface is a Map which also has a capped size.
// As more elements are added, old elements can be removed.
type AgingMapInterface interface {
	// Must be called before anything else. Set the max element size.
	init(int)

	// Adding the same key multiple times "refreshes" the age and updates the
	// value.
	add(key string, value string)
	get(key string) string
	remove(key string) string
}
