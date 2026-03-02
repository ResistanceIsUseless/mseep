package app

import (
	"github.com/ResistanceIsUseless/mseep/internal/adapters/claude"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/claudecode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/cline"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/crush"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/cursor"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/goose"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/lmstudio"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/opencode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/vscode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/warp"
	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/fuzzy"
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
		"claude":      claude.Adapter{},
		"claude-code": claudecode.Adapter{},
		"cursor":      cursor.Adapter{},
		"vscode":      vscode.Adapter{},
		"cline":       cline.Adapter{},
		"warp":        warp.Adapter{},
		"crush":       crush.Adapter{},
		"opencode":    opencode.Adapter{},
		"lmstudio":    lmstudio.Adapter{},
		"goose":       goose.Adapter{},
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
