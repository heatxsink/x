package storage

// resetForTest clears the memoized Store map. Only compiled during tests.
func resetForTest() {
	storesMu.Lock()
	defer storesMu.Unlock()
	stores = map[string]Store{}
}
