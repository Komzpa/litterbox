package ops

// Lookup returns a registered operation handler.
func Lookup(name string) (Handler, bool) {
	registry.RLock()
	defer registry.RUnlock()
	h, ok := registry.handlers[name]
	return h, ok
}
