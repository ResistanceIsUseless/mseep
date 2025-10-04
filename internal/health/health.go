package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"mseep/internal/config"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	ServerName string        `json:"server_name"`
	Type       string        `json:"type"`
	Status     CheckStatus   `json:"status"`
	Message    string        `json:"message"`
	Duration   time.Duration `json:"duration"`
	Timestamp  time.Time     `json:"timestamp"`
}

// CheckStatus represents the health check status
type CheckStatus string

const (
	StatusHealthy   CheckStatus = "healthy"
	StatusUnhealthy CheckStatus = "unhealthy"
	StatusTimeout   CheckStatus = "timeout"
	StatusError     CheckStatus = "error"
)

// Checker interface for different health check types
type Checker interface {
	Check(ctx context.Context, server config.Server) CheckResult
}

// Manager manages health checks for multiple servers
type Manager struct {
	checkers map[string]Checker
}

// NewManager creates a new health check manager
func NewManager() *Manager {
	return &Manager{
		checkers: map[string]Checker{
			"stdio": &StdioChecker{},
			"http":  &HTTPChecker{},
			"tcp":   &TCPChecker{},
		},
	}
}

// CheckServer performs a health check on a single server
func (m *Manager) CheckServer(ctx context.Context, server config.Server) CheckResult {
	start := time.Now()
	
	// Use default health check if none specified
	healthSpec := server.Health
	if healthSpec == nil {
		// Default to stdio check
		healthSpec = &config.HealthSpec{
			Type:      "stdio",
			TimeoutMs: 5000,
			Retries:   1,
		}
	}
	
	checker, exists := m.checkers[healthSpec.Type]
	if !exists {
		return CheckResult{
			ServerName: server.Name,
			Type:       healthSpec.Type,
			Status:     StatusError,
			Message:    fmt.Sprintf("unknown health check type: %s", healthSpec.Type),
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
		}
	}
	
	// Apply timeout
	timeout := time.Duration(healthSpec.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Perform check with retries
	retries := healthSpec.Retries
	if retries == 0 {
		retries = 1
	}
	
	var lastResult CheckResult
	for i := 0; i < retries; i++ {
		if i > 0 {
			// Brief delay between retries
			select {
			case <-checkCtx.Done():
				return CheckResult{
					ServerName: server.Name,
					Type:       healthSpec.Type,
					Status:     StatusTimeout,
					Message:    "health check timed out during retries",
					Duration:   time.Since(start),
					Timestamp:  time.Now(),
				}
			case <-time.After(500 * time.Millisecond):
			}
		}
		
		lastResult = checker.Check(checkCtx, server)
		if lastResult.Status == StatusHealthy {
			break
		}
	}
	
	lastResult.Duration = time.Since(start)
	lastResult.Timestamp = time.Now()
	return lastResult
}

// CheckServers performs health checks on multiple servers concurrently
func (m *Manager) CheckServers(ctx context.Context, servers []config.Server) []CheckResult {
	results := make([]CheckResult, len(servers))
	
	// Use a semaphore to limit concurrent checks
	sem := make(chan struct{}, 10)
	done := make(chan struct{}, len(servers))
	
	for i, server := range servers {
		go func(idx int, srv config.Server) {
			sem <- struct{}{}
			defer func() { <-sem }()
			defer func() { done <- struct{}{} }()
			
			results[idx] = m.CheckServer(ctx, srv)
		}(i, server)
	}
	
	// Wait for all checks to complete
	for i := 0; i < len(servers); i++ {
		select {
		case <-done:
		case <-ctx.Done():
			// Fill remaining results with timeout status
			for j := i; j < len(servers); j++ {
				if results[j].ServerName == "" {
					results[j] = CheckResult{
						ServerName: servers[j].Name,
						Type:       "unknown",
						Status:     StatusTimeout,
						Message:    "health check cancelled",
						Duration:   0,
						Timestamp:  time.Now(),
					}
				}
			}
			return results
		}
	}
	
	return results
}

// StdioChecker performs health checks by executing the server command
type StdioChecker struct{}

func (c *StdioChecker) Check(ctx context.Context, server config.Server) CheckResult {
	result := CheckResult{
		ServerName: server.Name,
		Type:       "stdio",
		Status:     StatusError,
		Message:    "",
	}
	
	if server.Command == "" {
		result.Message = "no command specified"
		return result
	}
	
	// Create command
	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	
	// Set environment variables
	if len(server.Env) > 0 {
		env := make([]string, 0, len(server.Env))
		for k, v := range server.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}
	
	// Try to start the process
	err := cmd.Start()
	if err != nil {
		result.Message = fmt.Sprintf("failed to start command: %v", err)
		return result
	}
	
	// Wait briefly to see if the process crashes immediately
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("command exited with error: %v", err)
		} else {
			// Command completed successfully (unusual for MCP servers)
			result.Status = StatusHealthy
			result.Message = "command completed successfully"
		}
	case <-time.After(1 * time.Second):
		// Process is still running after 1 second, consider it healthy
		result.Status = StatusHealthy
		result.Message = "command started successfully"
		
		// Try to terminate the process gracefully
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	case <-ctx.Done():
		result.Status = StatusTimeout
		result.Message = "health check timed out"
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
	
	return result
}

// HTTPChecker performs HTTP health checks
type HTTPChecker struct{}

func (c *HTTPChecker) Check(ctx context.Context, server config.Server) CheckResult {
	result := CheckResult{
		ServerName: server.Name,
		Type:       "http",
		Status:     StatusError,
		Message:    "",
	}
	
	healthSpec := server.Health
	if healthSpec == nil || healthSpec.URL == "" {
		result.Message = "no health check URL specified"
		return result
	}
	
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", healthSpec.URL, nil)
	if err != nil {
		result.Message = fmt.Sprintf("failed to create request: %v", err)
		return result
	}
	
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = StatusTimeout
			result.Message = "HTTP request timed out"
		} else {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("HTTP request failed: %v", err)
		}
		return result
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	
	return result
}

// TCPChecker performs TCP connection health checks
type TCPChecker struct{}

func (c *TCPChecker) Check(ctx context.Context, server config.Server) CheckResult {
	result := CheckResult{
		ServerName: server.Name,
		Type:       "tcp",
		Status:     StatusError,
		Message:    "",
	}
	
	healthSpec := server.Health
	if healthSpec == nil || healthSpec.URL == "" {
		result.Message = "no TCP address specified in health check URL"
		return result
	}
	
	// Parse address from URL
	address := healthSpec.URL
	if strings.HasPrefix(address, "tcp://") {
		address = strings.TrimPrefix(address, "tcp://")
	}
	
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = StatusTimeout
			result.Message = "TCP connection timed out"
		} else {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("TCP connection failed: %v", err)
		}
		return result
	}
	defer conn.Close()
	
	result.Status = StatusHealthy
	result.Message = "TCP connection successful"
	return result
}