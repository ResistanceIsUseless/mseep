package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mseep/internal/adapters/claude"
	"mseep/internal/config"
	"mseep/internal/health"
	"mseep/internal/style"
)

// HealthReport represents the results of health checks
type HealthReport struct {
	Timestamp time.Time             `json:"timestamp"`
	Results   []health.CheckResult  `json:"results"`
	Summary   HealthSummary         `json:"summary"`
}

// HealthSummary provides aggregate health information
type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Timeout   int `json:"timeout"`
	Error     int `json:"error"`
}

// Health performs health checks on servers
func (a *App) Health(client, serverFilter string, fix bool, jsonOutput bool) (string, error) {
	ctx := context.Background()
	manager := health.NewManager()
	
	// Filter servers based on criteria
	servers := a.getServersForHealthCheck(client, serverFilter)
	if len(servers) == 0 {
		return "", fmt.Errorf("no servers found matching criteria")
	}
	
	// Perform health checks
	results := manager.CheckServers(ctx, servers)
	
	// Create summary
	summary := createHealthSummary(results)
	
	report := HealthReport{
		Timestamp: time.Now(),
		Results:   results,
		Summary:   summary,
	}
	
	// Handle fix flag - disable failing servers
	if fix {
		if err := a.handleHealthFixes(results); err != nil {
			return "", fmt.Errorf("failed to apply health fixes: %w", err)
		}
	}
	
	// Format output
	if jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error formatting json: %w", err)
		}
		return string(output), nil
	}
	
	return a.formatHealthReport(report), nil
}

func (a *App) getServersForHealthCheck(client, serverFilter string) []config.Server {
	var servers []config.Server
	
	// If client is specified, only check servers enabled for that client
	if client != "" {
		switch client {
		case "claude":
			ca := claude.Adapter{}
			if detected, _ := ca.Detect(); !detected {
				return servers
			}
			
			claudeConfig, err := ca.Load()
			if err != nil {
				return servers
			}
			
			// Only include servers that are in Claude config
			for _, srv := range a.Canon.Servers {
				if _, exists := claudeConfig.MCPServers[srv.Name]; exists {
					if serverFilter == "" || matchesFilter(srv, serverFilter) {
						servers = append(servers, srv)
					}
				}
			}
		case "cursor", "cline":
			// TODO: Implement when adapters are available
			return servers
		}
	} else {
		// Check all enabled servers in canonical config
		for _, srv := range a.Canon.Servers {
			if srv.Enabled && (serverFilter == "" || matchesFilter(srv, serverFilter)) {
				servers = append(servers, srv)
			}
		}
	}
	
	return servers
}

func matchesFilter(server config.Server, filter string) bool {
	filter = strings.ToLower(filter)
	
	// Check name
	if strings.Contains(strings.ToLower(server.Name), filter) {
		return true
	}
	
	// Check aliases
	for _, alias := range server.Aliases {
		if strings.Contains(strings.ToLower(alias), filter) {
			return true
		}
	}
	
	// Check tags
	for _, tag := range server.Tags {
		if strings.Contains(strings.ToLower(tag), filter) {
			return true
		}
	}
	
	return false
}

func createHealthSummary(results []health.CheckResult) HealthSummary {
	summary := HealthSummary{Total: len(results)}
	
	for _, result := range results {
		switch result.Status {
		case health.StatusHealthy:
			summary.Healthy++
		case health.StatusUnhealthy:
			summary.Unhealthy++
		case health.StatusTimeout:
			summary.Timeout++
		case health.StatusError:
			summary.Error++
		}
	}
	
	return summary
}

func (a *App) handleHealthFixes(results []health.CheckResult) error {
	var serversToDisable []string
	
	for _, result := range results {
		if result.Status != health.StatusHealthy {
			serversToDisable = append(serversToDisable, result.ServerName)
		}
	}
	
	if len(serversToDisable) == 0 {
		return nil
	}
	
	// Disable unhealthy servers in canonical config
	for i := range a.Canon.Servers {
		for _, name := range serversToDisable {
			if a.Canon.Servers[i].Name == name {
				a.Canon.Servers[i].Enabled = false
				break
			}
		}
	}
	
	// Save canonical config
	if err := config.Save("", a.Canon); err != nil {
		return err
	}
	
	fmt.Print(style.Warning(fmt.Sprintf("Disabled %d unhealthy servers in canonical config", len(serversToDisable))) + "\n")
	fmt.Print(style.Muted("Run 'mseep apply' to sync changes to clients") + "\n")
	
	return nil
}

func (a *App) formatHealthReport(report HealthReport) string {
	var output strings.Builder
	
	output.WriteString(style.Title("Health Check Report"))
	output.WriteString("\n")
	
	// Summary
	output.WriteString(style.Header("Summary"))
	
	var summaryParts []string
	summaryParts = append(summaryParts, fmt.Sprintf("%d total", report.Summary.Total))
	
	if report.Summary.Healthy > 0 {
		summaryParts = append(summaryParts, style.Success(fmt.Sprintf("%d healthy", report.Summary.Healthy)))
	}
	if report.Summary.Unhealthy > 0 {
		summaryParts = append(summaryParts, style.Error(fmt.Sprintf("%d unhealthy", report.Summary.Unhealthy)))
	}
	if report.Summary.Timeout > 0 {
		summaryParts = append(summaryParts, style.Warning(fmt.Sprintf("%d timeout", report.Summary.Timeout)))
	}
	if report.Summary.Error > 0 {
		summaryParts = append(summaryParts, style.Error(fmt.Sprintf("%d error", report.Summary.Error)))
	}
	
	output.WriteString(strings.Join(summaryParts, ", ") + "\n\n")
	
	// Results table
	if len(report.Results) > 0 {
		output.WriteString(style.Header("Health Check Results"))
		
		var tableRows [][]string
		headers := []string{"Server", "Type", "Status", "Duration", "Message"}
		
		for _, result := range report.Results {
			statusStr := string(result.Status)
			switch result.Status {
			case health.StatusHealthy:
				statusStr = "✓ " + statusStr
			case health.StatusUnhealthy:
				statusStr = "✗ " + statusStr
			case health.StatusTimeout:
				statusStr = "⏱ " + statusStr
			case health.StatusError:
				statusStr = "⚠ " + statusStr
			}
			
			duration := result.Duration.Round(time.Millisecond).String()
			message := result.Message
			if len(message) > 50 {
				message = message[:47] + "..."
			}
			
			tableRows = append(tableRows, []string{
				result.ServerName,
				result.Type,
				statusStr,
				duration,
				message,
			})
		}
		
		output.WriteString("\n")
		output.WriteString(style.StatusTable(tableRows, headers))
	}
	
	// Recommendations
	if report.Summary.Unhealthy > 0 || report.Summary.Timeout > 0 || report.Summary.Error > 0 {
		output.WriteString("\n" + style.Header("Recommendations"))
		
		if report.Summary.Unhealthy > 0 {
			output.WriteString(style.Muted("• Check server configurations and dependencies") + "\n")
		}
		if report.Summary.Timeout > 0 {
			output.WriteString(style.Muted("• Consider increasing health check timeouts") + "\n")
		}
		if report.Summary.Error > 0 {
			output.WriteString(style.Muted("• Review health check configurations") + "\n")
		}
		
		output.WriteString(style.Muted("• Use '--fix' flag to auto-disable failing servers") + "\n")
	}
	
	output.WriteString("\n" + style.Muted(fmt.Sprintf("Health check completed at %s", 
		report.Timestamp.Format("2006-01-02 15:04:05"))) + "\n")
	
	return output.String()
}