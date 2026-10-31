package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConnectDisconnectListPeers(t *testing.T) {
	r := New(nil)
	relaySrv := httptest.NewServer(r)
	defer relaySrv.Close()

	peerURL := "ws" + relaySrv.URL[len("http"):]
	if err := r.Connect(peerURL); err != nil {
		t.Fatalf("connect: %v", err)
	}
	peers := r.ListPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if err := r.Disconnect(peerURL); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if len(r.ListPeers()) != 0 {
		t.Fatalf("expected 0 peers")
	}
}

func TestBroadcast(t *testing.T) {
	r := New(nil)
	relaySrv := httptest.NewServer(r)
	defer relaySrv.Close()

	r2 := New(nil)
	peerURL := "ws" + relaySrv.URL[len("http"):]
	if err := r2.Connect(peerURL); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer r2.Disconnect(peerURL)

	time.Sleep(100 * time.Millisecond)

	if err := r2.Broadcast("room1", "hello"); err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
}

func TestForwardNilDB(t *testing.T) {
	r := New(nil)
	if err := r.Forward(Message{RoomID: "r", Body: "hi"}); err != nil {
		t.Fatalf("forward: %v", err)
	}
}

func TestServeHTTPImplementsHandler(t *testing.T) {
	r := New(nil)
	_ = http.Handler(r)
}
