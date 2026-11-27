package sim

import (
	"math"
	"testing"
	"time"

	"github.com/its-the-vibe/NodeWorld/config"
)

func testConfig() config.Config {
	return config.Config{
		Nodes:      []config.Node{{ID: "a", Level: 3}, {ID: "b", Level: 1}},
		Links:      []config.Link{{From: "a", To: "b", Magnitude: 7, ScheduleDay: 2}},
		Simulation: config.Settings{CycleDays: 2, DaysPerSec: 1},
	}
}

func TestOverflowLeavesTransferUnapplied(t *testing.T) {
	cfg := testConfig()
	cfg.Nodes[1].Level = math.MaxInt64
	cfg.Links[0].ScheduleDay = 1
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snapshot, _, err := s.Step(1)
	if err == nil || snapshot.Nodes[0].Level != 3 || snapshot.Nodes[1].Level != math.MaxInt64 || snapshot.SimDay != 1 {
		t.Fatalf("overflow: %+v %v", snapshot, err)
	}
}

func TestSelfTransfer(t *testing.T) {
	cfg := testConfig()
	cfg.Links = []config.Link{{From: "a", To: "a", Magnitude: 7, ScheduleDay: 1}}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snapshot, events, err := s.Step(1)
	if err != nil || snapshot.Nodes[0].Level != 3 || *events[1].SourceLevel != -4 || *events[1].TargetLevel != 3 {
		t.Fatalf("self transfer: %+v %+v %v", snapshot, events, err)
	}
}

func TestStepScheduleAndNegativeLevel(t *testing.T) {
	s, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snapshot, events, err := s.Step(1)
	if err != nil || snapshot.SimDay != 1 || snapshot.Nodes[0].Level != 3 || len(events) != 1 {
		t.Fatalf("day 1: %+v %+v %v", snapshot, events, err)
	}
	snapshot, events, err = s.Step(1)
	if err != nil || snapshot.SimDay != 2 || snapshot.Nodes[0].Level != -4 || snapshot.Nodes[1].Level != 8 || len(events) != 2 {
		t.Fatalf("day 2: %+v %+v %v", snapshot, events, err)
	}
	if events[1].Type != "flow_event" || *events[1].SourceLevel != -4 || *events[1].TargetLevel != 8 {
		t.Fatalf("unexpected flow: %+v", events[1])
	}
	snapshot, events, err = s.Step(2)
	if err != nil || snapshot.SimDay != 2 || snapshot.ElapsedDays != 4 || snapshot.Nodes[0].Level != -11 || len(events) != 3 {
		t.Fatalf("cycle: %+v %+v %v", snapshot, events, err)
	}
}

func TestSubscribeAndPlayback(t *testing.T) {
	s, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snapshot, ch := s.Subscribe()
	defer s.Unsubscribe(ch)
	if snapshot.SimDay != 0 {
		t.Fatal(snapshot)
	}
	if err := s.Play(100); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-ch:
		if e.Type != "day_tick" || e.SimDay != 1 {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("no tick")
	}
	s.Pause()
	if s.Snapshot().Running {
		t.Fatal("still running")
	}
	if err := s.SetSpeed(0); err == nil {
		t.Fatal("invalid speed accepted")
	}
}
