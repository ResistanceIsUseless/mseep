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
	Servers  []Server          `json:"servers"`
	Profiles map[string][]string `json:"profiles"` // profile -> enabled server names
	Meta     Meta              `json:"meta"`
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
}

type HealthSpec struct {
	Type      string        `json:"type"`        // stdio|http|tcp
	URL       string        `json:"url,omitempty"`
	TimeoutMs int           `json:"timeoutMs,omitempty"`
	Retries   int           `json:"retries,omitempty"`
}

type PolicySpec struct {
	AutoDisable      bool `json:"autoDisable"`
	FailureThreshold int  `json:"failureThreshold,omitempty"` // default 3
	WindowHours      int  `json:"windowHours,omitempty"`      // default 24
	CooldownHours    int  `json:"cooldownHours,omitempty"`    // default 24
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil { return "", err }
	p := filepath.Join(dir, "mseep", "canonical.json")
	return p, nil
}

func EnsureDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil { return "", err }
	p := filepath.Join(dir, "mseep")
	if err := os.MkdirAll(p, 0o755); err != nil { return "", err }
	return p, nil
}

func Load(path string) (*Canonical, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil { return nil, err }
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
	if err := json.Unmarshal(b, &c); err != nil { return nil, err }
	return &c, nil
}

func Save(path string, c *Canonical) error {
	if c == nil { return fmt.Errorf("nil canonical config") }
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil { return err }
	}
	if _, err := EnsureDir(); err != nil { return err }
	c.Meta.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil { return err }
	return os.WriteFile(path, b, 0o644)
}

func (c *Canonical) FindByName(name string) *Server {
	for i := range c.Servers {
		if c.Servers[i].Name == name { return &c.Servers[i] }
	}
	return nil
}

func (c *Canonical) EnabledSet() map[string]bool {
	m := map[string]bool{}
	for _, s := range c.Servers {
		if s.Enabled { m[s.Name] = true }
	}
	return m
}
