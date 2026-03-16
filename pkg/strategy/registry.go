package strategy

import (
	"fmt"
	"sync"
)

// Constructor is a factory function that returns a new Strategy instance.
// The engine calls this once per Run() invocation.
type Constructor func() Strategy

// Registry maps strategy names to their constructors.
// It is safe for concurrent use.
type Registry struct {
	mu           sync.RWMutex
	constructors map[string]Constructor
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{constructors: make(map[string]Constructor)}
}

// Register associates name with constructor.
// Panics if name is already registered (fail-fast at startup).
func (r *Registry) Register(name string, ctor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.constructors[name]; exists {
		panic(fmt.Sprintf("strategy %q is already registered", name))
	}
	r.constructors[name] = ctor
}

// Create returns a new Strategy for name, or an error if name is unknown.
func (r *Registry) Create(name string) (Strategy, error) {
	r.mu.RLock()
	ctor, ok := r.constructors[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown strategy %q; registered strategies: %v", name, r.Names())
	}
	return ctor(), nil
}

// Names returns all registered strategy names in an unspecified order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.constructors))
	for n := range r.constructors {
		names = append(names, n)
	}
	return names
}
