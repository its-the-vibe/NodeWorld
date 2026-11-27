package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/its-the-vibe/NodeWorld/sim"
)

func printJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func Run(s *sim.Simulation, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "nodeworld> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		args := strings.Fields(scanner.Text())
		if len(args) == 0 {
			continue
		}
		var err error
		switch args[0] {
		case "quit":
			if len(args) == 1 {
				return nil
			}
			err = fmt.Errorf("usage: quit")
		case "pause":
			if len(args) != 1 {
				err = fmt.Errorf("usage: pause")
			} else {
				s.Pause()
			}
		case "play":
			speed := s.Snapshot().DaysPerSec
			if len(args) > 2 {
				err = fmt.Errorf("usage: play [days_per_sec]")
			} else if len(args) == 2 {
				speed, err = strconv.ParseFloat(args[1], 64)
			}
			if err == nil {
				err = s.Play(speed)
			}
		case "step":
			n := 1
			if len(args) > 2 {
				err = fmt.Errorf("usage: step [n]")
			} else if len(args) == 2 {
				n, err = strconv.Atoi(args[1])
			}
			if err == nil {
				var snapshot sim.Snapshot
				snapshot, _, err = s.Step(n)
				if err == nil {
					err = printJSON(out, snapshot)
				}
			}
		case "status":
			if len(args) != 1 {
				err = fmt.Errorf("usage: status")
			} else {
				err = printJSON(out, s.Snapshot())
			}
		case "node":
			if len(args) != 2 {
				err = fmt.Errorf("usage: node <id>")
				break
			}
			snapshot := s.Snapshot()
			found := false
			for _, node := range snapshot.Nodes {
				if node.ID == args[1] {
					var links []any
					for _, link := range snapshot.Links {
						if link.From == node.ID {
							links = append(links, link)
						}
					}
					err = printJSON(out, struct {
						Node  any   `json:"node"`
						Links []any `json:"links"`
					}{node, links})
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("unknown node %q", args[1])
			}
		default:
			err = fmt.Errorf("unknown command %q", args[0])
		}
		if err != nil {
			fmt.Fprintln(out, "error:", err)
		}
	}
}
