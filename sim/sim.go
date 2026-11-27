package sim

import (
	"fmt"
	"math"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/its-the-vibe/NodeWorld/config"
)

type Snapshot struct {
	SimDay      int           `json:"sim_day"`
	ElapsedDays int64         `json:"elapsed_days"`
	Running     bool          `json:"running"`
	DaysPerSec  float64       `json:"days_per_sec"`
	CycleDays   int           `json:"cycle_days"`
	Nodes       []config.Node `json:"nodes"`
	Links       []config.Link `json:"links"`
}

type Event struct {
	Type        string    `json:"type"`
	Snapshot    *Snapshot `json:"snapshot,omitempty"`
	SimDay      int       `json:"sim_day,omitempty"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to,omitempty"`
	Amount      *int64    `json:"amount,omitempty"`
	SourceLevel *int64    `json:"source_level,omitempty"`
	TargetLevel *int64    `json:"target_level,omitempty"`
	Message     string    `json:"message,omitempty"`
}

type nodeRequest struct {
	delta int64
	reply chan config.Node
}

type scheduledRequest struct {
	day   int
	reply chan []config.Link
}

type nodeActor struct {
	node  config.Node
	links []config.Link
}

func (n *nodeActor) Receive(ctx *actor.Context) {
	switch m := ctx.Message().(type) {
	case nodeRequest:
		n.node.Level += m.delta
		m.reply <- n.node
	case scheduledRequest:
		var links []config.Link
		for _, link := range n.links {
			if link.ScheduleDay == m.day {
				links = append(links, link)
			}
		}
		m.reply <- links
	}
}

type command struct {
	action string
	n      int
	speed  float64
	reply  chan result
	sub    chan Event
}
type result struct {
	snapshot Snapshot
	events   []Event
	err      error
}
type tick struct{ generation uint64 }

type coordinator struct {
	engine     *actor.Engine
	nodes      map[string]*actor.PID
	order      []string
	links      []config.Link
	cycle      int
	day        int
	elapsed    int64
	running    bool
	speed      float64
	generation uint64
	timer      *time.Timer
	subs       map[chan Event]bool
}

func (c *coordinator) node(id string, delta int64) config.Node {
	reply := make(chan config.Node, 1)
	c.engine.Send(c.nodes[id], nodeRequest{delta, reply})
	return <-reply
}

func (c *coordinator) snapshot() Snapshot {
	s := Snapshot{SimDay: c.day, ElapsedDays: c.elapsed, Running: c.running, DaysPerSec: c.speed, CycleDays: c.cycle, Nodes: make([]config.Node, 0, len(c.order)), Links: append([]config.Link{}, c.links...)}
	for _, id := range c.order {
		s.Nodes = append(s.Nodes, c.node(id, 0))
	}
	return s
}

func (c *coordinator) publish(e Event) {
	for ch := range c.subs {
		select {
		case ch <- e:
		default:
			timer := time.NewTimer(time.Second)
			select {
			case ch <- e:
				timer.Stop()
			case <-timer.C:
				close(ch)
				delete(c.subs, ch)
			}
		}
	}
}

func (c *coordinator) advance() ([]Event, error) {
	c.day = c.day%c.cycle + 1
	c.elapsed++
	events := []Event{{Type: "day_tick", SimDay: c.day}}
	c.publish(events[0])
	for _, id := range c.order {
		reply := make(chan []config.Link, 1)
		c.engine.Send(c.nodes[id], scheduledRequest{day: c.day, reply: reply})
		for _, l := range <-reply {
			sourceBefore := c.node(l.From, 0).Level
			targetBefore := sourceBefore
			if l.From != l.To {
				targetBefore = c.node(l.To, 0).Level
			}
			if sourceBefore < math.MinInt64+l.Magnitude ||
				(l.From != l.To && targetBefore > math.MaxInt64-l.Magnitude) {
				err := fmt.Errorf("level overflow on link %q -> %q at day %d", l.From, l.To, c.day)
				c.publish(Event{Type: "error", Message: err.Error()})
				return events, err
			}
			source := c.node(l.From, -l.Magnitude)
			target := c.node(l.To, l.Magnitude)
			amount := l.Magnitude
			e := Event{Type: "flow_event", SimDay: c.day, From: l.From, To: l.To, Amount: &amount, SourceLevel: &source.Level, TargetLevel: &target.Level}
			events = append(events, e)
			c.publish(e)
		}
	}
	return events, nil
}

func (c *coordinator) schedule(pid *actor.PID) {
	c.generation++
	if c.timer != nil {
		c.timer.Stop()
	}
	if c.running {
		generation := c.generation
		c.timer = time.AfterFunc(time.Duration(float64(time.Second)/c.speed), func() {
			c.engine.Send(pid, tick{generation})
		})
	}
}

func (c *coordinator) Receive(ctx *actor.Context) {
	switch m := ctx.Message().(type) {
	case tick:
		if c.running && m.generation == c.generation {
			if _, err := c.advance(); err != nil {
				c.running = false
			}
			c.schedule(ctx.PID())
		}
	case command:
		r := result{}
		switch m.action {
		case "snapshot":
			r.snapshot = c.snapshot()
		case "subscribe":
			r.snapshot = c.snapshot()
			c.subs[m.sub] = true
		case "unsubscribe":
			if c.subs[m.sub] {
				delete(c.subs, m.sub)
				close(m.sub)
			}
		case "step":
			if m.n < 1 {
				r.err = fmt.Errorf("step count must be positive")
			} else {
				for i := 0; i < m.n; i++ {
					var events []Event
					events, r.err = c.advance()
					r.events = append(r.events, events...)
					if r.err != nil {
						c.running = false
						c.schedule(ctx.PID())
						break
					}
				}
				r.snapshot = c.snapshot()
			}
		case "play":
			if !config.ValidSpeed(m.speed) {
				r.err = fmt.Errorf("days_per_sec must be between 0.001 and 1000")
			} else {
				c.speed, c.running = m.speed, true
				c.schedule(ctx.PID())
			}
		case "speed":
			if !config.ValidSpeed(m.speed) {
				r.err = fmt.Errorf("days_per_sec must be between 0.001 and 1000")
			} else {
				c.speed = m.speed
				c.schedule(ctx.PID())
			}
		case "resume":
			c.running = true
			c.schedule(ctx.PID())
		case "pause":
			c.running = false
			c.schedule(ctx.PID())
		case "close":
			c.running = false
			c.schedule(ctx.PID())
			for ch := range c.subs {
				close(ch)
				delete(c.subs, ch)
			}
		}
		m.reply <- r
	}
}

type Simulation struct {
	engine *actor.Engine
	clock  *actor.PID
	nodes  []*actor.PID
}

func New(cfg config.Config) (*Simulation, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	engine, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		return nil, err
	}
	s := &Simulation{engine: engine}
	c := &coordinator{engine: engine, nodes: make(map[string]*actor.PID), links: append([]config.Link{}, cfg.Links...), cycle: cfg.Simulation.CycleDays, speed: cfg.Simulation.DaysPerSec, subs: make(map[chan Event]bool)}
	outgoing := make(map[string][]config.Link)
	for _, l := range cfg.Links {
		outgoing[l.From] = append(outgoing[l.From], l)
	}
	for _, node := range cfg.Nodes {
		n := node
		pid := engine.Spawn(func() actor.Receiver { return &nodeActor{node: n, links: outgoing[n.ID]} }, "node", actor.WithID(n.ID))
		c.nodes[n.ID] = pid
		c.order = append(c.order, n.ID)
		s.nodes = append(s.nodes, pid)
	}
	s.clock = engine.Spawn(func() actor.Receiver { return c }, "clock")
	return s, nil
}

func (s *Simulation) call(c command) result {
	c.reply = make(chan result, 1)
	s.engine.Send(s.clock, c)
	return <-c.reply
}

func (s *Simulation) Snapshot() Snapshot { return s.call(command{action: "snapshot"}).snapshot }
func (s *Simulation) Step(n int) (Snapshot, []Event, error) {
	r := s.call(command{action: "step", n: n})
	return r.snapshot, r.events, r.err
}
func (s *Simulation) Play(speed float64) error {
	return s.call(command{action: "play", speed: speed}).err
}
func (s *Simulation) SetSpeed(speed float64) error {
	return s.call(command{action: "speed", speed: speed}).err
}
func (s *Simulation) Resume() { s.call(command{action: "resume"}) }
func (s *Simulation) Pause()  { s.call(command{action: "pause"}) }
func (s *Simulation) Subscribe() (Snapshot, chan Event) {
	ch := make(chan Event, 256)
	r := s.call(command{action: "subscribe", sub: ch})
	return r.snapshot, ch
}
func (s *Simulation) Unsubscribe(ch chan Event) { s.call(command{action: "unsubscribe", sub: ch}) }
func (s *Simulation) Close() {
	s.call(command{action: "close"})
	<-s.engine.Stop(s.clock).Done()
	for _, pid := range s.nodes {
		<-s.engine.Stop(pid).Done()
	}
}
