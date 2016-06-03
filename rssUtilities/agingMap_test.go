package rssUtilities

import "testing"

func verifyKVPair(t *testing.T, am AgingMapInterface, key, eVal string) {
	val := am.Get(key)
	if val != eVal {
		t.Error("Unexpected KV pair: ", key, val, ", expected ", eVal)
	}
}

func TestAgingMapBasic(t *testing.T) {
	// Verify that the AgingMap implements the interface
	var _ AgingMapInterface = (*AgingMap)(nil)

	am := &AgingMap{}
	am.Init(2)

	var key, eVal string
	key = "foo"
	eVal = "bar"
	am.Add(key, eVal)
	verifyKVPair(t, am, key, eVal)

	key = "baz"
	eVal = "blat"
	am.Add(key, eVal)
	verifyKVPair(t, am, key, eVal)

	am.Remove("foo")
	verifyKVPair(t, am, "foo", "")
	verifyKVPair(t, am, "baz", "blat")

	am.Remove("baz")
	verifyKVPair(t, am, "foo", "")
	verifyKVPair(t, am, "baz", "")
}

func TestAgingMapSize(t *testing.T) {
	am := &AgingMap{}
	am.Init(2)
	am.Add("foo", "bar")
	am.Add("foos", "ball")
	verifyKVPair(t, am, "foo", "bar")
	verifyKVPair(t, am, "foos", "ball")

	am.Add("baz", "blat")
	verifyKVPair(t, am, "foo", "")
	verifyKVPair(t, am, "foos", "ball")
	verifyKVPair(t, am, "baz", "blat")
}
