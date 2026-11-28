package ws

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/its-the-vibe/NodeWorld/config"
	"github.com/its-the-vibe/NodeWorld/sim"
)

func TestWebSocketSnapshotAndStep(t *testing.T) {
	s, err := sim.New(config.Config{
		Nodes:      []config.Node{{ID: "a", Level: 1}, {ID: "b"}},
		Links:      []config.Link{{From: "a", To: "b", Magnitude: 2, ScheduleDay: 1}},
		Simulation: config.Settings{CycleDays: 30, DaysPerSec: 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	defer s.Close()
	handler := New(s)
	server := httptest.NewServer(handler)
	defer handler.Close()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	var message sim.Event
	if err := wsjson.Read(ctx, conn, &message); err != nil || message.Type != "snapshot" || message.Snapshot.SimDay != 0 {
		t.Fatalf("snapshot: %+v %v", message, err)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "step"}); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Read(ctx, conn, &message); err != nil || message.Type != "day_tick" || message.SimDay != 1 {
		t.Fatalf("tick: %+v %v", message, err)
	}
	message = sim.Event{}
	if err := wsjson.Read(ctx, conn, &message); err != nil || message.Type != "flow_event" || *message.SourceLevel != -1 || *message.TargetLevel != 2 {
		t.Fatalf("flow: %+v %v", message, err)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "set_speed", "days_per_sec": 3}); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "resume"}); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Read(ctx, conn, &message); err != nil || message.Type != "day_tick" || message.SimDay != 2 {
		t.Fatalf("playback: %+v %v", message, err)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "pause"}); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketResetCommand(t *testing.T) {
	s, err := sim.New(config.Config{
		Nodes:      []config.Node{{ID: "a", Level: 5}, {ID: "b", Level: 1}},
		Links:      []config.Link{{From: "a", To: "b", Magnitude: 2, ScheduleDay: 1}},
		Simulation: config.Settings{CycleDays: 2, DaysPerSec: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := New(s)
	server := httptest.NewServer(handler)
	defer handler.Close()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	var event sim.Event
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Step(1); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "reset"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"day_tick", "flow_event", "snapshot"} {
		event = sim.Event{}
		if err := wsjson.Read(ctx, conn, &event); err != nil || event.Type != want {
			t.Fatalf("event: %+v, want %s: %v", event, want, err)
		}
		if want == "snapshot" && (event.Snapshot == nil || event.Snapshot.SimDay != 0 ||
			event.Snapshot.Nodes[0].Level != 5 || event.Snapshot.Nodes[1].Level != 1) {
			t.Fatalf("reset snapshot: %+v", event)
		}
	}
}

func TestLargeStepStreamsAllTicks(t *testing.T) {
	s, err := sim.New(config.Config{Simulation: config.Settings{CycleDays: 30, DaysPerSec: 1}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := New(s)
	server := httptest.NewServer(handler)
	defer handler.Close()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	var event sim.Event
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "step", "days": 10000}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		if err := wsjson.Read(ctx, conn, &event); err != nil || event.Type != "day_tick" || event.SimDay != i%30+1 {
			t.Fatalf("tick %d: %+v %v", i, event, err)
		}
	}
}
