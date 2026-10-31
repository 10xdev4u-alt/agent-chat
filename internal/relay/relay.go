package relay

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"

	roomspkg "github.com/10xdev4u-alt/agent-chat/internal/rooms"
)

// Message is a relayed chat message.
type Message struct {
	RoomID   string   `json:"room_id"`
	SenderID string   `json:"sender_id"`
	Body     string   `json:"body"`
	Mentions []string `json:"mentions,omitempty"`
	ReplyTo  string   `json:"reply_to,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Origin   string   `json:"origin,omitempty"`
}

type peerConn struct {
	url       string
	conn      *websocket.Conn
	cancel    context.CancelFunc
	reconnect bool
	mu        sync.Mutex
}

// Relay manages connections to peer agent-chat instances.
type Relay struct {
	mu      sync.Mutex
	peers   map[string]*peerConn // outbound
	inbound map[string]*peerConn // inbound
	db      *sql.DB
}

// New creates a Relay.
func New(db *sql.DB) *Relay {
	return &Relay{db: db, peers: make(map[string]*peerConn), inbound: make(map[string]*peerConn)}
}

// NewRelay is an alias for New.
func NewRelay(db *sql.DB) *Relay { return New(db) }

func (r *Relay) dial(pc *peerConn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	conn, _, err := websocket.Dial(ctx, pc.url, nil)
	cancel()
	if err != nil {
		return err
	}
	connCtx, connCancel := context.WithCancel(context.Background())
	pc.mu.Lock()
	pc.conn = conn
	pc.cancel = connCancel
	pc.mu.Unlock()
	go r.readLoop(pc, connCtx)
	return nil
}

func (r *Relay) readLoop(pc *peerConn, ctx context.Context) {
	pc.mu.Lock()
	conn := pc.conn
	pc.mu.Unlock()
	if conn == nil {
		return
	}
	defer func() {
		conn.Close(websocket.StatusNormalClosure, "")
		pc.mu.Lock()
		if pc.conn == conn {
			pc.conn = nil
		}
		cancel := pc.cancel
		pc.cancel = nil
		pc.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		r.mu.Lock()
		if !pc.reconnect {
			delete(r.inbound, pc.url)
		}
		reconnect := pc.reconnect
		_, stillPeer := r.peers[pc.url]
		_, stillInbound := r.inbound[pc.url]
		still := stillPeer || stillInbound
		r.mu.Unlock()
		if reconnect && still {
			go r.reconnect(pc)
		}
	}()
	for {
		readCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		_, data, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			return
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		_ = r.Forward(msg)
	}
}

func (r *Relay) reconnect(pc *peerConn) {
	backoff := time.Second
	for {
		r.mu.Lock()
		_, ok := r.peers[pc.url]
		reconnect := pc.reconnect
		r.mu.Unlock()
		if !ok || !reconnect {
			return
		}
		time.Sleep(backoff)
		r.mu.Lock()
		_, ok = r.peers[pc.url]
		r.mu.Unlock()
		if !ok {
			return
		}
		if err := r.dial(pc); err == nil {
			return
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

// Connect connects to a peer instance.
func (r *Relay) Connect(peerURL string) error {
	r.mu.Lock()
	if _, ok := r.peers[peerURL]; ok {
		r.mu.Unlock()
		return fmt.Errorf("already connected: %s", peerURL)
	}
	pc := &peerConn{url: peerURL, reconnect: true}
	r.peers[peerURL] = pc
	r.mu.Unlock()

	if err := r.dial(pc); err != nil {
		go r.reconnect(pc)
		return err
	}
	return nil
}

// Disconnect disconnects from a peer.
func (r *Relay) Disconnect(peerURL string) error {
	r.mu.Lock()
	pc, ok := r.peers[peerURL]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("not connected: %s", peerURL)
	}
	pc.reconnect = false
	pc.mu.Lock()
	c := pc.conn
	cancel := pc.cancel
	pc.conn = nil
	pc.cancel = nil
	pc.mu.Unlock()
	delete(r.peers, peerURL)
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if c != nil {
		c.Close(websocket.StatusNormalClosure, "disconnect")
	}
	return nil
}

// ListPeers returns connected peer URLs.
func (r *Relay) ListPeers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.peers))
	for u := range r.peers {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// Broadcast sends a message to all peers.
func (r *Relay) Broadcast(roomID string, message any) error {
	msg := Message{RoomID: roomID}
	switch v := message.(type) {
	case string:
		msg.Body = v
	case Message:
		msg = v
		if msg.RoomID == "" {
			msg.RoomID = roomID
		}
	default:
		b, _ := json.Marshal(v)
		msg.Body = string(b)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	r.mu.Lock()
	peers := make([]*peerConn, 0, len(r.peers)+len(r.inbound))
	for _, pc := range r.peers {
		peers = append(peers, pc)
	}
	for _, pc := range r.inbound {
		peers = append(peers, pc)
	}
	r.mu.Unlock()

	var firstErr error
	for _, pc := range peers {
		pc.mu.Lock()
		c := pc.conn
		pc.mu.Unlock()
		if c == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := c.Write(ctx, websocket.MessageText, data); err != nil && firstErr == nil {
			firstErr = err
		}
		cancel()
	}
	return firstErr
}

// Forward forwards an incoming message to local rooms.
func (r *Relay) Forward(msg Message) error {
	if r.db == nil {
		return nil
	}
	if msg.RoomID == "" {
		return errors.New("room_id is required")
	}
	sender := msg.SenderID
	if sender == "" {
		sender = "relay"
	}
	// Best-effort join.
	_ = roomspkg.JoinRoom(r.db, msg.RoomID, sender, "member")
	_, _, err := roomspkg.SendMessage(r.db, msg.RoomID, sender, msg.Body, msg.Mentions, msg.ReplyTo, msg.Priority)
	return err
}

// ServeHTTP handles incoming peer WebSocket connections.
func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	key := req.RemoteAddr
	r.mu.Lock()
	if _, ok := r.inbound[key]; ok {
		key = key + "-" + time.Now().Format("150405.000")
	}
	if _, ok := r.peers[key]; ok {
		key = key + "-" + time.Now().Format("150405.000")
	}
	pc := &peerConn{url: key, conn: conn, reconnect: false}
	r.inbound[key] = pc
	r.mu.Unlock()

	pc.mu.Lock()
	pc.conn = conn
	pc.mu.Unlock()

	go func() {
		defer func() {
			conn.Close(websocket.StatusNormalClosure, "")
			r.mu.Lock()
			delete(r.inbound, key)
			delete(r.peers, key)
			r.mu.Unlock()
		}()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			_, data, err := conn.Read(ctx)
			cancel()
			if err != nil {
				return
			}
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			_ = r.Forward(msg)
		}
	}()
}

// Handler returns an http.Handler for incoming peer connections.
func (r *Relay) Handler() http.Handler { return r }

// normalizePeerURL ensures a scheme is present (best-effort).
func normalizePeerURL(u string) string {
	u = strings.TrimSpace(u)
	if !strings.Contains(u, "://") {
		return "ws://" + u
	}
	return u
}
