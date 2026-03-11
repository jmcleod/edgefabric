package plugin

import (
	"testing"

	"github.com/jmcleod/edgefabric/internal/bgp"
)

func TestRegisterAndGet(t *testing.T) {
	restore := resetForTesting()
	defer restore()

	called := false
	factory := BGPFactory(func() bgp.Service {
		called = true
		return bgp.NewNoopService()
	})

	Register(PluginTypeBGP, "test-bgp", factory)

	got, ok := Get(PluginTypeBGP, "test-bgp")
	if !ok {
		t.Fatal("expected factory to be registered")
	}

	f, ok := got.(BGPFactory)
	if !ok {
		t.Fatal("expected BGPFactory type assertion to succeed")
	}

	svc := f()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if !called {
		t.Fatal("expected factory to be called")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	restore := resetForTesting()
	defer restore()

	factory := BGPFactory(func() bgp.Service { return bgp.NewNoopService() })
	Register(PluginTypeBGP, "dup", factory)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()

	Register(PluginTypeBGP, "dup", factory) // should panic
}

func TestRegisteredNames(t *testing.T) {
	restore := resetForTesting()
	defer restore()

	Register(PluginTypeBGP, "beta", BGPFactory(func() bgp.Service { return bgp.NewNoopService() }))
	Register(PluginTypeBGP, "alpha", BGPFactory(func() bgp.Service { return bgp.NewNoopService() }))

	names := RegisteredNames(PluginTypeBGP)
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("expected [alpha, beta], got %v", names)
	}
}

func TestGetUnknown(t *testing.T) {
	restore := resetForTesting()
	defer restore()

	got, ok := Get(PluginTypeBGP, "nonexistent")
	if ok {
		t.Fatal("expected ok to be false for unregistered plugin")
	}
	if got != nil {
		t.Fatal("expected nil factory for unregistered plugin")
	}
}

func TestRegisteredNamesEmpty(t *testing.T) {
	restore := resetForTesting()
	defer restore()

	names := RegisteredNames(PluginTypeBGP)
	if names != nil {
		t.Fatalf("expected nil for unregistered type, got %v", names)
	}
}

func TestBuiltinRegistrations(t *testing.T) {
	// This test uses the real registry populated by builtin.go init().
	// It does NOT call resetForTesting().

	tests := []struct {
		pluginType PluginType
		expected   []string
	}{
		{PluginTypeBGP, []string{"gobgp", "noop"}},
		{PluginTypeDNS, []string{"miekg", "noop"}},
		{PluginTypeCDN, []string{"noop", "proxy"}},
		{PluginTypeRoute, []string{"forwarder", "noop"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.pluginType), func(t *testing.T) {
			names := RegisteredNames(tt.pluginType)
			if len(names) != len(tt.expected) {
				t.Fatalf("expected %d plugins, got %d: %v", len(tt.expected), len(names), names)
			}
			for i, name := range names {
				if name != tt.expected[i] {
					t.Errorf("names[%d] = %q, want %q", i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestDifferentTypesCanShareNames(t *testing.T) {
	restore := resetForTesting()
	defer restore()

	Register(PluginTypeBGP, "noop", BGPFactory(func() bgp.Service { return bgp.NewNoopService() }))
	// Register "noop" under a different plugin type — should not panic.
	Register(PluginTypeCDN, "noop", "placeholder-factory")

	bgpFactory, ok := Get(PluginTypeBGP, "noop")
	if !ok {
		t.Fatal("expected BGP noop to be registered")
	}
	_, ok = bgpFactory.(BGPFactory)
	if !ok {
		t.Fatal("expected BGPFactory type")
	}

	cdnFactory, ok := Get(PluginTypeCDN, "noop")
	if !ok {
		t.Fatal("expected CDN noop to be registered")
	}
	if cdnFactory != "placeholder-factory" {
		t.Fatal("expected placeholder CDN factory")
	}
}
