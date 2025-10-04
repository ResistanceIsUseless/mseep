package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// MCPServersOrgSource fetches servers from mcpservers.org
type MCPServersOrgSource struct {
	client *http.Client
}

func (s *MCPServersOrgSource) Name() string {
	return "mcpservers.org"
}

func (s *MCPServersOrgSource) FetchServers(ctx context.Context) ([]ServerEntry, error) {
	// Note: This is a placeholder implementation
	// The actual mcpservers.org API might have a different structure
	url := "https://api.mcpservers.org/servers" // Hypothetical API endpoint
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := s.client.Do(req)
	if err != nil {
		// If the API doesn't exist yet, return empty results instead of failing
		return []ServerEntry{}, nil
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return []ServerEntry{}, nil // Gracefully handle non-200 responses
	}
	
	var apiResponse struct {
		Servers []struct {
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Command     string            `json:"command"`
			Args        []string          `json:"args"`
			Env         map[string]string `json:"env"`
			Tags        []string          `json:"tags"`
			Author      string            `json:"author"`
			Repository  string            `json:"repository"`
			Homepage    string            `json:"homepage"`
			Version     string            `json:"version"`
		} `json:"servers"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	servers := make([]ServerEntry, 0, len(apiResponse.Servers))
	for _, s := range apiResponse.Servers {
		servers = append(servers, ServerEntry{
			Name:        s.Name,
			Description: s.Description,
			Command:     s.Command,
			Args:        s.Args,
			Env:         s.Env,
			Tags:        s.Tags,
			Author:      s.Author,
			Repository:  s.Repository,
			Homepage:    s.Homepage,
			Version:     s.Version,
			Source:      "mcpservers.org",
		})
	}
	
	return servers, nil
}

// GitHubAwesomeSource fetches servers from GitHub awesome lists
type GitHubAwesomeSource struct {
	client *http.Client
}

func (s *GitHubAwesomeSource) Name() string {
	return "github-awesome"
}

func (s *GitHubAwesomeSource) FetchServers(ctx context.Context) ([]ServerEntry, error) {
	// Fetch from awesome-mcp-servers repository (example)
	// This would parse README.md or a servers.json file
	
	// For now, return some curated examples that we know exist
	return []ServerEntry{
		{
			Name:        "filesystem",
			Description: "Secure file system access for MCP clients",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-filesystem"},
			Tags:        []string{"filesystem", "files", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "git",
			Description: "Git repository operations via MCP",
			Command:     "npx", 
			Args:        []string{"@modelcontextprotocol/server-git"},
			Tags:        []string{"git", "vcs", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "github",
			Description: "GitHub API access via MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-github"},
			Env:         map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": ""},
			Tags:        []string{"github", "api", "official"},
			Author:      "Anthropic", 
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "gitlab",
			Description: "GitLab API access via MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-gitlab"},
			Env:         map[string]string{"GITLAB_PERSONAL_ACCESS_TOKEN": ""},
			Tags:        []string{"gitlab", "api", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "postgres",
			Description: "PostgreSQL database access via MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-postgres"},
			Tags:        []string{"database", "postgresql", "sql", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "sqlite",
			Description: "SQLite database access via MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-sqlite"},
			Tags:        []string{"database", "sqlite", "sql", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "slack",
			Description: "Slack workspace integration via MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-slack"},
			Env:         map[string]string{"SLACK_BOT_TOKEN": ""},
			Tags:        []string{"slack", "chat", "communication", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "memory",
			Description: "Persistent memory and knowledge graphs for MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-memory"},
			Tags:        []string{"memory", "knowledge", "persistence", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "brave-search",
			Description: "Brave Search API integration for MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-brave-search"},
			Env:         map[string]string{"BRAVE_SEARCH_API_KEY": ""},
			Tags:        []string{"search", "web", "brave", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
		{
			Name:        "google-drive",
			Description: "Google Drive file access via MCP",
			Command:     "npx",
			Args:        []string{"@modelcontextprotocol/server-google-drive"},
			Tags:        []string{"google", "drive", "files", "cloud", "official"},
			Author:      "Anthropic",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Source:      "github-awesome",
		},
	}, nil
}

// In the future, this could parse actual GitHub repositories:
func (s *GitHubAwesomeSource) fetchFromGitHubAPI(ctx context.Context, repoURL string) ([]ServerEntry, error) {
	// Parse repository URL to extract owner/repo
	parts := strings.Split(strings.TrimPrefix(repoURL, "https://github.com/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub repository URL: %s", repoURL)
	}
	
	owner, repo := parts[0], parts[1]
	
	// Fetch repository contents API to look for servers.json or parse README
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/servers.json", owner, repo)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub API request: %w", err)
	}
	
	// Add GitHub API headers if we had a token
	// req.Header.Set("Authorization", "token " + githubToken)
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from GitHub API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		// servers.json doesn't exist, could try to parse README.md instead
		return []ServerEntry{}, nil
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	// Parse GitHub API response and decode base64 content
	// This is left as an exercise for future enhancement
	return []ServerEntry{}, nil
}