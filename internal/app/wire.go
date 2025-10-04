package app

import (
	"mseep/internal/adapters/claude"
	"mseep/internal/adapters/cline"
	"mseep/internal/adapters/cursor"
	"mseep/internal/adapters/vscode"
	"mseep/internal/adapters/warp"
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
	bestMatch, err := fuzzy.SelectBest(query, idx, assumeYes)
	if err != nil { return "", err }
	chosen := bestMatch.Name

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

	// apply to detected clients or specific client
	var diff string
	var lastErr error
	
	adapters := map[string]interface {
		Name() string
		Detect() (bool, error)
		Apply(*config.Canonical) (string, error)
	}{
		"claude": claude.Adapter{},
		"cursor": cursor.Adapter{},
		"vscode": vscode.Adapter{},
		"cline":  cline.Adapter{},
		"warp":   warp.Adapter{},
	}
	
	for name, adapter := range adapters {
		if client != "" && client != name {
			continue // Skip if specific client requested and this isn't it
		}
		
		ok, _ := adapter.Detect()
		if ok {
			d, err := adapter.Apply(a.Canon)
			if err != nil {
				lastErr = err
			} else if d != "" {
				if diff != "" {
					diff += "\n\n" + d
				} else {
					diff = d
				}
			}
		}
	}
	
	return diff, lastErr
}
