package mago

import (
	"testing"

	"github.com/darkliquid/mago/internal/testlib"
)

// newNullLibrary builds the current bridge from source, opens it, and registers
// cleanup. It is the shared entry point for null-backend lifecycle tests.
func newNullLibrary(t *testing.T) *Library {
	t.Helper()

	libPath := testlib.BuildRuntimeLibrary(t, ".")
	lib, err := Open(WithLibraryPath(libPath))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	t.Cleanup(func() {
		if err := lib.Close(); err != nil {
			t.Errorf("close library: %v", err)
		}
	})
	return lib
}

// newNullContext creates a null-backend context and registers cleanup.
func newNullContext(t *testing.T, lib *Library) *Context {
	t.Helper()

	ctx, err := lib.NewContext(BackendNull)
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	t.Cleanup(func() {
		if err := ctx.Close(); err != nil {
			t.Errorf("close context: %v", err)
		}
	})
	return ctx
}

func TestDoubleCloseIsSafe(t *testing.T) {
	lib := newNullLibrary(t)

	ctx := newNullContext(t, lib)
	if err := ctx.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := ctx.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestUseAfterCloseReturnsError(t *testing.T) {
	lib := newNullLibrary(t)
	if err := lib.Close(); err != nil {
		t.Fatalf("close library: %v", err)
	}

	if _, err := lib.NewContext(BackendNull); err == nil {
		t.Fatal("expected error creating a context on a closed library")
	}
}
