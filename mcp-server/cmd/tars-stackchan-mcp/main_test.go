package main

import (
	"fmt"
	"testing"
)

func TestNewBridgeFromEnvDefaultsToMock(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")

	bridge, err := newBridgeFromEnv()
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	if got := fmt.Sprintf("%T", bridge); got != "*mock.Bridge" {
		t.Fatalf("bridge type = %s, want *mock.Bridge", got)
	}
}

func TestNewBridgeFromEnvCreatesHTTPBridge(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "http")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "http://stackchan.local")
	t.Setenv("TARS_STACKCHAN_TOKEN", "secret")

	bridge, err := newBridgeFromEnv()
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	if got := fmt.Sprintf("%T", bridge); got != "*httpbridge.Bridge" {
		t.Fatalf("bridge type = %s, want *httpbridge.Bridge", got)
	}
}

func TestNewBridgeFromEnvRejectsUnknownBridge(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "serial")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")

	_, err := newBridgeFromEnv()
	if err == nil {
		t.Fatal("expected unknown bridge error")
	}
}
