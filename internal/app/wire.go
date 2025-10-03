package app

import (
	"fmt"

	"mseep/internal/adapters/claude"
	"mseep/internal/config"
	"mseep/internal/fuzzy"
)

type App struct {
	Canon *config.Canonical
}

func LoadApp() (*App, error) {
	c, err := config.Load("")
	if err != nil { return nil, err }
	return &App{Canon: c}, nil
}

// Basic fuzzy enable/disable/toggle for Claude only (MVP)
func (a *App) Toggle(mode, query, client string, assumeYes bool) (string, error) {
	// index
	idx := make([]fuzzy.Index, 0, len(a.Canon.Servers))
	for _, s := range a.Canon.Servers {
		idx = append(idx, fuzzy.Index{Name: s.Name, Aliases: s.Aliases, Tags: s.Tags})
	}
	cands := fuzzy.Candidates(query, idx)
	if len(cands) == 0 { return "", fmt.Errorf("no match for %q", query) }
	chosen := cands[0].Index.Name // simple best match for MVP

	// flip state in canonical
	for i := range a.Canon.Servers {
		if a.Canon.Servers[i].Name == chosen {
			switch mode {
			case "enable": a.Canon.Servers[i].Enabled = true
			case "disable": a.Canon.Servers[i].Enabled = false
			case "toggle": a.Canon.Servers[i].Enabled = !a.Canon.Servers[i].Enabled
			}
			break
		}
	}
	if err := config.Save("", a.Canon); err != nil { return "", err }

	// apply to Claude if present or if client==claude/empty
	aCl := claude.Adapter{}
	if client == "" || client == "claude" {
		ok, _ := aCl.Detect()
		if ok {
			d, err := aCl.Apply(a.Canon)
			return d, err
		}
	}
	return "", nil
}
