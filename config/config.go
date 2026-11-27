package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"gopkg.in/yaml.v3"
)

type Node struct {
	ID    string         `yaml:"id" json:"id"`
	Label string         `yaml:"label" json:"label"`
	Kind  string         `yaml:"kind" json:"kind"`
	Level int64          `yaml:"level" json:"level"`
	Meta  map[string]any `yaml:"meta,omitempty" json:"meta,omitempty"`
}

type Link struct {
	From        string `yaml:"from" json:"from"`
	To          string `yaml:"to" json:"to"`
	Magnitude   int64  `yaml:"magnitude" json:"magnitude"`
	ScheduleDay int    `yaml:"schedule_day" json:"schedule_day"`
}

type Settings struct {
	CycleDays  int     `yaml:"cycle_days" json:"cycle_days"`
	DaysPerSec float64 `yaml:"days_per_sec" json:"days_per_sec"`
}

type Config struct {
	Nodes      []Node   `yaml:"nodes" json:"nodes"`
	Links      []Link   `yaml:"links" json:"links"`
	Simulation Settings `yaml:"simulation" json:"simulation"`
}

func ValidSpeed(speed float64) bool {
	// Keep timer durations representable and avoid busy loops.
	return !math.IsNaN(speed) && !math.IsInf(speed, 0) && speed >= 0.001 && speed <= 1000
}

func Load(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("decode config: %w", err)
	}
	if c.Simulation.CycleDays == 0 {
		c.Simulation.CycleDays = 30
	}
	if c.Simulation.DaysPerSec == 0 {
		c.Simulation.DaysPerSec = 1
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.Simulation.CycleDays < 1 {
		return fmt.Errorf("cycle_days must be positive")
	}
	if !ValidSpeed(c.Simulation.DaysPerSec) {
		return fmt.Errorf("days_per_sec must be between 0.001 and 1000")
	}
	ids := make(map[string]bool)
	for _, n := range c.Nodes {
		if n.ID == "" || ids[n.ID] {
			return fmt.Errorf("node IDs must be nonempty and unique: %q", n.ID)
		}
		if _, err := json.Marshal(n.Meta); err != nil {
			return fmt.Errorf("node %q metadata must be JSON-compatible: %w", n.ID, err)
		}
		ids[n.ID] = true
	}
	for _, l := range c.Links {
		if !ids[l.From] || !ids[l.To] {
			return fmt.Errorf("link %q -> %q references unknown node", l.From, l.To)
		}
		if l.Magnitude < 0 || l.ScheduleDay < 1 || l.ScheduleDay > c.Simulation.CycleDays {
			return fmt.Errorf("invalid link %q -> %q", l.From, l.To)
		}
	}
	return nil
}
