package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Canonical struct {
	Servers  []Server            `json:"servers"`
	Profiles map[string][]string `json:"profiles"` // profile -> enabled server names
	Settings Settings            `json:"settings"`
	Meta     Meta                `json:"meta"`
}

// Settings controls global mseep behaviour.
type Settings struct {
	// Mode selects how servers are delivered to clients.
	//   "basic"   – write individual server entries directly to each client config (default)
	//   "wrapper" – write a single "mseep proxy" entry; mseep multiplexes servers at runtime
	Mode    string          `json:"mode,omitempty"`
	Wrapper WrapperSettings `json:"wrapper,omitempty"`
}

// WrapperSettings tune wrapper-mode behaviour.
type WrapperSettings struct {
	// PollIntervalSeconds is how often the proxy polls canonical.json for changes.
	// Default 5. Set to 0 to disable polling (static load at startup only).
	PollIntervalSeconds int `json:"pollIntervalSeconds,omitempty"`

	// NamespaceTools prefixes each tool name with "<serverName>__" to avoid
	// collisions when multiple servers expose tools with the same name.
	NamespaceTools bool `json:"namespaceTools,omitempty"`

	// NamespaceSeparator overrides the default "__" separator.
	NamespaceSeparator string `json:"namespaceSeparator,omitempty"`
}

type Meta struct {
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Server struct {
	Name      string            `json:"name"`
	Aliases   []string          `json:"aliases,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Transport string            `json:"transport,omitempty"` // stdio|http|tcp
	Enabled   bool              `json:"enabled"`
	Health    *HealthSpec       `json:"healthCheck,omitempty"`
	Policy    *PolicySpec       `json:"policy,omitempty"`

	// Setup metadata - helps users configure complex servers
	Description   string       `json:"description,omitempty"`   // What the server does
	Repository    string       `json:"repository,omitempty"`    // GitHub/GitLab URL
	Homepage      string       `json:"homepage,omitempty"`      // Documentation URL
	Prerequisites []string     `json:"prerequisites,omitempty"` // e.g. ["python3", "pip", "ghidra"]
	SetupSteps    []string     `json:"setupSteps,omitempty"`    // Human-readable setup instructions
	Secrets       []SecretSpec `json:"secrets,omitempty"`       // Required env vars with descriptions
}

// SecretSpec describes a required secret/API key for a server.
type SecretSpec struct {
	Name        string `json:"name"`                  // Env var name (e.g. GITHUB_TOKEN)
	Description string `json:"description,omitempty"` // What this secret is for
	Required    bool   `json:"required"`              // Whether server fails without it
	URL         string `json:"url,omitempty"`         // Where to obtain this secret
}

type HealthSpec struct {
	Type      string `json:"type"` // stdio|http|tcp
	URL       string `json:"url,omitempty"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
	Retries   int    `json:"retries,omitempty"`
}

type PolicySpec struct {
	AutoDisable      bool `json:"autoDisable"`
	FailureThreshold int  `json:"failureThreshold,omitempty"` // default 3
	WindowHours      int  `json:"windowHours,omitempty"`      // default 24
	CooldownHours    int  `json:"cooldownHours,omitempty"`    // default 24
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "mseep", "canonical.json")
	return p, nil
}

func EnsureDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "mseep")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}

func Load(path string) (*Canonical, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c := &Canonical{Meta: Meta{Version: "1", UpdatedAt: time.Now()}, Profiles: map[string][]string{}}
			return c, nil
		}
		return nil, err
	}
	var c Canonical
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Save(path string, c *Canonical) error {
	if c == nil {
		return fmt.Errorf("nil canonical config")
	}
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if _, err := EnsureDir(); err != nil {
		return err
	}
	c.Meta.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// EffectiveMode returns the resolved mode, defaulting to "basic".
func (c *Canonical) EffectiveMode() string {
	if c.Settings.Mode == "wrapper" {
		return "wrapper"
	}
	return "basic"
}

// EffectivePollInterval returns the wrapper poll interval, defaulting to 5s.
func (c *Canonical) EffectivePollInterval() int {
	if c.Settings.Wrapper.PollIntervalSeconds > 0 {
		return c.Settings.Wrapper.PollIntervalSeconds
	}
	return 5
}

// EffectiveNamespaceSeparator returns the separator for tool namespacing, defaulting to "__".
func (c *Canonical) EffectiveNamespaceSeparator() string {
	if c.Settings.Wrapper.NamespaceSeparator != "" {
		return c.Settings.Wrapper.NamespaceSeparator
	}
	return "__"
}

func (c *Canonical) FindByName(name string) *Server {
	for i := range c.Servers {
		if c.Servers[i].Name == name {
			return &c.Servers[i]
		}
	}
	return nil
}

func (c *Canonical) EnabledSet() map[string]bool {
	m := map[string]bool{}
	for _, s := range c.Servers {
		if s.Enabled {
			m[s.Name] = true
		}
	}
	return m
}
