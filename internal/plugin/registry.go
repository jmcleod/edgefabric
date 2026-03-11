package plugin

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = make(map[PluginType]map[string]any) // PluginType → name → factory
)

// Register adds a named factory for the given plugin type.
// It panics on duplicate (name, type) pairs — this is intentional because
// Register is called during init(), so duplicates are caught at startup.
func Register(pt PluginType, name string, factory any) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[pt]; !exists {
		registry[pt] = make(map[string]any)
	}
	if _, dup := registry[pt][name]; dup {
		panic(fmt.Sprintf("plugin: duplicate registration for %s/%s", pt, name))
	}
	registry[pt][name] = factory
}

// Get retrieves the factory for the given plugin type and name.
// Returns (factory, true) if found, (nil, false) otherwise.
func Get(pt PluginType, name string) (any, bool) {
	mu.RLock()
	defer mu.RUnlock()

	byName, ok := registry[pt]
	if !ok {
		return nil, false
	}
	factory, ok := byName[name]
	return factory, ok
}

// RegisteredNames returns a sorted list of plugin names for the given type.
func RegisteredNames(pt PluginType) []string {
	mu.RLock()
	defer mu.RUnlock()

	byName, ok := registry[pt]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// resetForTesting clears the registry and returns a function to restore it.
// Only for use in tests.
func resetForTesting() func() {
	mu.Lock()
	saved := registry
	registry = make(map[PluginType]map[string]any)
	mu.Unlock()

	return func() {
		mu.Lock()
		registry = saved
		mu.Unlock()
	}
}
