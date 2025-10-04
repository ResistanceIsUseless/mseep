package marketplace

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"mseep/internal/config"
)

// ServerEntry represents an MCP server available in the marketplace
type ServerEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Author      string            `json:"author,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Version     string            `json:"version,omitempty"`
	Source      string            `json:"source"` // "mcpservers.org", "github", etc.
	Installed   bool              `json:"installed,omitempty"`
}

// Marketplace aggregates MCP servers from multiple sources
type Marketplace struct {
	cache      map[string][]ServerEntry
	cacheTime  time.Time
	cacheTTL   time.Duration
	httpClient *http.Client
}

// NewMarketplace creates a new marketplace instance
func NewMarketplace() *Marketplace {
	return &Marketplace{
		cache:      make(map[string][]ServerEntry),
		cacheTTL:   30 * time.Minute, // Cache for 30 minutes
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Sources defines the data sources for MCP servers
type Source interface {
	Name() string
	FetchServers(ctx context.Context) ([]ServerEntry, error)
}

// GetServers fetches all available servers from all sources
func (m *Marketplace) GetServers(ctx context.Context, canonical *config.Canonical) ([]ServerEntry, error) {
	// Check cache first
	if time.Since(m.cacheTime) < m.cacheTTL && len(m.cache) > 0 {
		return m.mergeCachedServers(canonical), nil
	}

	// Initialize sources
	sources := []Source{
		&MCPServersOrgSource{client: m.httpClient},
		&GitHubAwesomeSource{client: m.httpClient},
	}

	// Fetch from all sources
	allServers := make([]ServerEntry, 0)
	for _, source := range sources {
		servers, err := source.FetchServers(ctx)
		if err != nil {
			// Log error but continue with other sources
			fmt.Printf("Warning: Failed to fetch from %s: %v\n", source.Name(), err)
			continue
		}
		allServers = append(allServers, servers...)
	}

	// Deduplicate and cache
	deduplicated := m.deduplicateServers(allServers)
	m.cache["all"] = deduplicated
	m.cacheTime = time.Now()

	return m.mergeCachedServers(canonical), nil
}

// SearchServers searches for servers matching the query
func (m *Marketplace) SearchServers(ctx context.Context, query string, canonical *config.Canonical) ([]ServerEntry, error) {
	allServers, err := m.GetServers(ctx, canonical)
	if err != nil {
		return nil, err
	}

	if query == "" {
		return allServers, nil
	}

	query = strings.ToLower(strings.TrimSpace(query))
	var matches []ServerEntry

	for _, server := range allServers {
		if m.matchesQuery(server, query) {
			matches = append(matches, server)
		}
	}

	// Sort by relevance (simple scoring for now)
	sort.Slice(matches, func(i, j int) bool {
		scoreI := m.calculateRelevanceScore(matches[i], query)
		scoreJ := m.calculateRelevanceScore(matches[j], query)
		return scoreI > scoreJ
	})

	return matches, nil
}

// InstallServer installs a server from the marketplace to the canonical config
func (m *Marketplace) InstallServer(serverEntry ServerEntry, canonical *config.Canonical) error {
	// Check if server already exists
	for _, existing := range canonical.Servers {
		if existing.Name == serverEntry.Name {
			return fmt.Errorf("server %q already exists in canonical config", serverEntry.Name)
		}
	}

	// Create new server entry
	newServer := config.Server{
		Name:      serverEntry.Name,
		Command:   serverEntry.Command,
		Args:      serverEntry.Args,
		Env:       serverEntry.Env,
		Tags:      serverEntry.Tags,
		Transport: "stdio", // Default to stdio transport
		Enabled:   false,   // Default to disabled until user enables
	}

	// Add to canonical config
	canonical.Servers = append(canonical.Servers, newServer)

	// Save the updated config
	if err := config.Save("", canonical); err != nil {
		return fmt.Errorf("failed to save config after installing server: %w", err)
	}

	return nil
}

// Helper methods

func (m *Marketplace) mergeCachedServers(canonical *config.Canonical) []ServerEntry {
	servers := m.cache["all"]
	if canonical == nil {
		return servers
	}

	// Mark installed servers
	installedMap := make(map[string]bool)
	for _, server := range canonical.Servers {
		installedMap[server.Name] = true
	}

	result := make([]ServerEntry, len(servers))
	for i, server := range servers {
		result[i] = server
		result[i].Installed = installedMap[server.Name]
	}

	return result
}

func (m *Marketplace) deduplicateServers(servers []ServerEntry) []ServerEntry {
	seen := make(map[string]ServerEntry)
	
	for _, server := range servers {
		key := server.Name
		if existing, exists := seen[key]; exists {
			// Prefer entries with more information or from more authoritative sources
			if len(server.Description) > len(existing.Description) || 
			   server.Source == "mcpservers.org" {
				seen[key] = server
			}
		} else {
			seen[key] = server
		}
	}

	result := make([]ServerEntry, 0, len(seen))
	for _, server := range seen {
		result = append(result, server)
	}

	// Sort alphabetically by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

func (m *Marketplace) matchesQuery(server ServerEntry, query string) bool {
	searchFields := []string{
		strings.ToLower(server.Name),
		strings.ToLower(server.Description),
		strings.ToLower(server.Author),
		strings.ToLower(strings.Join(server.Tags, " ")),
	}

	for _, field := range searchFields {
		if strings.Contains(field, query) {
			return true
		}
	}

	return false
}

func (m *Marketplace) calculateRelevanceScore(server ServerEntry, query string) int {
	score := 0
	
	// Exact name match gets highest score
	if strings.ToLower(server.Name) == query {
		score += 100
	} else if strings.Contains(strings.ToLower(server.Name), query) {
		score += 50
	}
	
	// Description matches
	if strings.Contains(strings.ToLower(server.Description), query) {
		score += 20
	}
	
	// Tag matches
	for _, tag := range server.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 10
		}
	}
	
	// Author matches
	if strings.Contains(strings.ToLower(server.Author), query) {
		score += 5
	}
	
	return score
}