package strategy

import (
	"sort"
	"sync"
	"testing"
)

// ── Registry ──────────────────────────────────────────────────────────────────

func TestNewRegistry_IsEmpty(t *testing.T) {
	r := NewRegistry()
	if names := r.Names(); len(names) != 0 {
		t.Errorf("expected empty registry, got %v", names)
	}
}

func TestRegistry_RegisterAndCreate(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", func() Strategy { return NewDummyStrategy("foo") })
	got, err := r.Create("foo")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.Name() != "foo" {
		t.Errorf("expected Name()=foo, got %q", got.Name())
	}
}

func TestRegistry_CreateUnknownReturnsError(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Create("missing"); err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestRegistry_RegisterDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register("dup", func() Strategy { return NewDummyStrategy("dup") })
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate Register")
		}
	}()
	r.Register("dup", func() Strategy { return NewDummyStrategy("dup") })
}

func TestRegistry_NamesReturnsAll(t *testing.T) {
	r := NewRegistry()
	r.Register("a", func() Strategy { return NewDummyStrategy("a") })
	r.Register("b", func() Strategy { return NewDummyStrategy("b") })
	names := r.Names()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("expected [a b], got %v", names)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	r.Register("x", func() Strategy { return NewDummyStrategy("x") })
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = r.Create("x") }()
		go func() { defer wg.Done(); _ = r.Names() }()
	}
	wg.Wait()
}

// ── Global registry ──────────────────────────────────────────────────────────

func TestGlobalRegister_AndList(t *testing.T) {
	// Use a unique name to avoid colliding with other tests.
	const name = "__test_global_strategy__"
	Register(name, func() Strategy { return NewDummyStrategy(name) })

	found := false
	for _, n := range List() {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in global List(), got %v", name, List())
	}

	s, err := Create(name)
	if err != nil {
		t.Fatalf("Create(%q) error: %v", name, err)
	}
	if s.Name() != name {
		t.Errorf("Create returned strategy with Name()=%q", s.Name())
	}
}

func TestGlobal_Returns_SameRegistry(t *testing.T) {
	if Global() == nil {
		t.Error("Global() returned nil")
	}
	a := Global()
	b := Global()
	if a != b {
		t.Error("Global() must return the same instance on repeated calls")
	}
}
