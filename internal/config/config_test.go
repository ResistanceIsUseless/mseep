package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalMarshalUnmarshal(t *testing.T) {
	canon := &Canonical{
		Servers: []Server{
			{
				Name:      "test-server",
				Aliases:   []string{"test", "ts"},
				Tags:      []string{"testing", "dev"},
				Command:   "node",
				Args:      []string{"test.js"},
				Env:       map[string]string{"NODE_ENV": "test"},
				Transport: "stdio",
				Enabled:   true,
				Health: &HealthSpec{
					Type:      "http",
					URL:       "http://localhost:3000/health",
					TimeoutMs: 5000,
					Retries:   3,
				},
				Policy: &PolicySpec{
					AutoDisable:      true,
					FailureThreshold: 3,
					WindowHours:      24,
					CooldownHours:    24,
				},
			},
		},
		Profiles: map[string][]string{
			"dev":  {"test-server", "debug-server"},
			"prod": {"prod-server"},
		},
		Meta: Meta{
			Version:   "1.0.0",
			UpdatedAt: time.Now(),
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(canon)
	if err != nil {
		t.Fatalf("failed to marshal canonical: %v", err)
	}

	// Unmarshal back
	var decoded Canonical
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal canonical: %v", err)
	}

	// Verify fields
	if len(decoded.Servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(decoded.Servers))
	}

	srv := decoded.Servers[0]
	if srv.Name != "test-server" {
		t.Errorf("server name = %s, want test-server", srv.Name)
	}
	if !srv.Enabled {
		t.Error("server should be enabled")
	}
	if srv.Health == nil {
		t.Error("health spec should not be nil")
	} else if srv.Health.Type != "http" {
		t.Errorf("health type = %s, want http", srv.Health.Type)
	}
	if srv.Policy == nil {
		t.Error("policy spec should not be nil")
	} else if !srv.Policy.AutoDisable {
		t.Error("policy auto-disable should be true")
	}

	if len(decoded.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(decoded.Profiles))
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}

	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %s", path)
	}

	expectedSuffix := filepath.Join("Library", "Application Support", "mseep", "canonical.json")
	if !strings.HasSuffix(path, expectedSuffix) {
		t.Errorf("path %s doesn't have expected suffix %s", path, expectedSuffix)
	}
}

func TestLoadSave(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-canonical.json")

	// Create test canonical
	canon := &Canonical{
		Servers: []Server{
			{
				Name:    "test-server",
				Command: "test",
				Enabled: true,
			},
		},
		Profiles: map[string][]string{
			"test": {"test-server"},
		},
		Meta: Meta{
			Version:   "1.0.0",
			UpdatedAt: time.Now(),
		},
	}

	// Save
	if err := Save(testPath, canon); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testPath); err != nil {
		t.Fatalf("saved file doesn't exist: %v", err)
	}

	// Load
	loaded, err := Load(testPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded content
	if len(loaded.Servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(loaded.Servers))
	}
	if loaded.Servers[0].Name != "test-server" {
		t.Errorf("server name = %s, want test-server", loaded.Servers[0].Name)
	}
	if len(loaded.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(loaded.Profiles))
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.json")

	canon, err := Load(nonExistentPath)
	if err != nil {
		t.Fatalf("Load() should create default config, got error: %v", err)
	}

	if canon == nil {
		t.Fatal("expected default canonical, got nil")
	}
	// Servers is not initialized by default, which is fine
	if canon.Profiles == nil {
		t.Error("profiles should be initialized")
	}
}

