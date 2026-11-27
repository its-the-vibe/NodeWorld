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
