package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/its-the-vibe/NodeWorld/cli"
	"github.com/its-the-vibe/NodeWorld/config"
	"github.com/its-the-vibe/NodeWorld/sim"
	"github.com/its-the-vibe/NodeWorld/ws"
)

func run() error {
	configPath := flag.String("config", "examples/sample.yaml", "YAML configuration file")
	days := flag.Int("days", 0, "run N days headlessly (0 starts the REPL)")
	output := flag.String("output", "", "write headless JSON output to this file")
	address := flag.String("ws", "", "WebSocket listen address, e.g. localhost:8080")
	flag.Parse()
	if *days < 0 {
		return fmt.Errorf("days must not be negative")
	}
	if *output != "" && *days == 0 {
		return fmt.Errorf("output requires --days")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	s, err := sim.New(cfg)
	if err != nil {
		return err
	}
	defer s.Close()
	if *address != "" {
		listener, err := net.Listen("tcp", *address)
		if err != nil {
			return err
		}
		handler := ws.New(s)
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
		defer handler.Close()
		defer server.Close()
		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				fmt.Fprintln(os.Stderr, "WebSocket server:", err)
			}
		}()
		fmt.Fprintln(os.Stderr, "WebSocket listening on", listener.Addr())
	}
	if *days == 0 {
		return cli.Run(s, os.Stdin, os.Stdout)
	}
	snapshot, events, err := s.Step(*days)
	if err != nil {
		return err
	}
	out := os.Stdout
	if *output != "" {
		out, err = os.Create(*output)
		if err != nil {
			return err
		}
		defer out.Close()
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Snapshot sim.Snapshot `json:"snapshot"`
		Events   []sim.Event  `json:"events"`
	}{snapshot, events})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "nodeworld:", err)
		os.Exit(1)
	}
}
