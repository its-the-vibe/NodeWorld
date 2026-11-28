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

func TestREPLHelp(t *testing.T) {
	tests := []struct {
		name, input string
		want        []string
	}{
		{
			name:  "all commands",
			input: "help\nquit\n",
			want: []string{
				"step [n] - Advance", "play [days_per_sec] - Start", "pause - Pause",
				"status - Display", "node <id> - Display", "quit - Exit", "help [command] - Display",
			},
		},
		{"step details", "help step\nquit\n", []string{"Usage: step [n]", "positive integer", "defaults to 1", "while playing"}},
		{"play details", "help play\nquit\n", []string{"Usage: play [days_per_sec]", "current speed", "0.001 and 1000"}},
		{"pause details", "help pause\nquit\n", []string{"Usage: pause", "Pause the simulation"}},
		{"status details", "help status\nquit\n", []string{"Usage: status", "current simulation status"}},
		{"node details", "help node\nquit\n", []string{"Usage: node <id>", "outgoing links", "existing node"}},
		{"quit details", "help quit\nquit\n", []string{"Usage: quit", "Exit the REPL"}},
		{"help details", "help help\nquit\n", []string{"Usage: help [command]", "command name"}},
		{"unknown command", "help missing\nstatus\nquit\n", []string{`error: unknown command "missing"`, "use help", `"sim_day": 0`}},
		{"too many arguments", "help step extra\nstatus\nquit\n", []string{"error: usage: help [command]", `"sim_day": 0`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := sim.New(config.Config{Nodes: []config.Node{{ID: "a"}}, Simulation: config.Settings{CycleDays: 30, DaysPerSec: 1}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var out bytes.Buffer
			if err := Run(s, strings.NewReader(tt.input), &out); err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %s", want, out.String())
				}
			}
		})
	}
}
