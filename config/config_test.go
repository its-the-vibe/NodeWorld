package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("simulation:\n  cycle_days: 28\n  days_per_sec: 2\nnodes:\n  - id: a\n    level: 3\n    meta:\n      x: 10\n  - id: b\nlinks:\n  - from: a\n    to: b\n    magnitude: 7\n    schedule_day: 28\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Simulation.CycleDays != 28 || c.Nodes[0].Meta["x"] != 10 || c.Links[0].ScheduleDay != 28 {
		t.Fatalf("unexpected config: %+v", c)
	}
	c.Links[0].To = "missing"
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid link to be rejected")
	}
}
