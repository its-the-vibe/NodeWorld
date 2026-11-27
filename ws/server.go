package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/its-the-vibe/NodeWorld/sim"
)

type control struct {
	Type       string  `json:"type"`
	Days       int     `json:"days"`
	DaysPerSec float64 `json:"days_per_sec"`
}

type Server struct {
	mu          sync.Mutex
	closed      bool
	connections map[*websocket.Conn]bool
	wg          sync.WaitGroup
	handler     http.Handler
}

func New(s *sim.Simulation) *Server {
	server := &Server{connections: make(map[*websocket.Conn]bool)}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		server.mu.Lock()
		if server.closed {
			server.mu.Unlock()
			conn.CloseNow()
			return
		}
		server.connections[conn] = true
		server.wg.Add(1)
		server.mu.Unlock()
		defer func() {
			server.mu.Lock()
			delete(server.connections, conn)
			server.mu.Unlock()
			server.wg.Done()
		}()
		defer conn.CloseNow()
		conn.SetReadLimit(4096)
		snapshot, events := s.Subscribe()
		defer s.Unsubscribe(events)
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		write := func(v any) error {
			writeCtx, done := context.WithTimeout(ctx, 10*time.Second)
			defer done()
			return wsjson.Write(writeCtx, conn, v)
		}
		if err := write(sim.Event{Type: "snapshot", Snapshot: &snapshot}); err != nil {
			return
		}
		errors := make(chan string, 1)
		go func() {
			defer cancel()
			for {
				select {
				case event, ok := <-events:
					if !ok || write(event) != nil {
						return
					}
				case message := <-errors:
					if write(map[string]string{"type": "error", "message": message}) != nil {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		for {
			var command control
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if err = json.Unmarshal(data, &command); err == nil {
				switch command.Type {
				case "pause":
					s.Pause()
				case "resume":
					s.Resume()
				case "set_speed":
					err = s.SetSpeed(command.DaysPerSec)
				case "step":
					if command.Days == 0 {
						command.Days = 1
					}
					if command.Days < 1 || command.Days > 10000 {
						err = fmt.Errorf("days must be between 1 and 10000")
					} else {
						// Simulation errors are broadcast to all subscribers.
						_, _, _ = s.Step(command.Days)
					}
				default:
					err = fmt.Errorf("unknown command %q", command.Type)
				}
			}
			if err != nil {
				select {
				case errors <- err.Error():
				case <-ctx.Done():
					return
				}
			}
		}
	})
	server.handler = mux
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	for conn := range s.connections {
		conn.CloseNow()
	}
	s.mu.Unlock()
	s.wg.Wait()
}
