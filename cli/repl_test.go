package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/its-the-vibe/NodeWorld/config"
	"github.com/its-the-vibe/NodeWorld/sim"
)

func TestREPL(t *testing.T) {
	s, err := sim.New(config.Config{Nodes: []config.Node{{ID: "a"}}, Simulation: config.Settings{CycleDays: 30, DaysPerSec: 1}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var out bytes.Buffer
	if err := Run(s, strings.NewReader("step 2\nnode a\nplay 2\npause\nstatus\nquit\n"), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"sim_day": 2`, `"id": "a"`, `"running": false`, `"days_per_sec": 2`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s in %s", want, out.String())
		}
	}
}
