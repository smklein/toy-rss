package agingmap

// AgingMapInterface is a Map which also has a capped size.
// As more elements are added, old elements can be removed.
type AgingMapInterface interface {
	// Must be called before anything else. Set the max element size.
	Init(int)

	// Adding the same key multiple times "refreshes" the age and updates the
	// value.
	Add(key, value string)
	Get(key string) string
	Remove(key string) string
}
