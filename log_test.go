package mago

import (
	"sync"
	"testing"

	"github.com/darkliquid/mago/internal/testlib"
)

func TestLogCallbackReceivesPostedMessages(t *testing.T) {
	libPath := testlib.BuildRuntimeLibrary(t, ".")
	lib, err := Open(WithLibraryPath(libPath))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	defer func() {
		if err := lib.Close(); err != nil {
			t.Fatalf("close library: %v", err)
		}
	}()

	log, err := lib.NewLog()
	if err != nil {
		t.Fatalf("NewLog: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	var mu sync.Mutex
	var got []string
	token, err := log.Register(func(_ LogLevel, message string) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, message)
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	defer func() {
		if err := log.Unregister(token); err != nil {
			t.Errorf("Unregister: %v", err)
		}
	}()

	if err := log.Post(LogLevelInfo, "hello from mago"); err != nil {
		t.Fatalf("Post: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != "hello from mago" {
		t.Fatalf("got %v, want one message", got)
	}
}
