// Package proxy implements the mseep MCP multiplexer used in wrapper mode.
//
// # Architecture
//
// In wrapper mode, each AI client (Claude Code, Cursor, etc.) is configured to
// launch "mseep proxy --client <name>" instead of individual MCP server binaries.
// The proxy process:
//
//  1. Reads the canonical config to find all enabled servers.
//  2. Spawns each enabled stdio MCP server as a child subprocess.
//  3. Reads JSON-RPC 2.0 messages from stdin (the client) and routes them to
//     the appropriate server subprocess.
//  4. Aggregates capability responses (tools/list, resources/list, prompts/list)
//     across all servers, optionally prefixing tool names to avoid collisions.
//  5. Polls canonical.json every N seconds and dynamically starts/stops
//     server subprocesses when servers are enabled/disabled.
//
// # Session isolation
//
// Each client connection spawns its own "mseep proxy" OS process, so each client
// gets its own tree of MCP server subprocesses. There is no shared state between
// client sessions.
//
// # MCP framing
//
// MCP uses JSON-RPC 2.0 over stdio with HTTP-style Content-Length framing:
//
//	Content-Length: <n>\r\n
//	\r\n
//	<json bytes>
//
// # ID remapping
//
// Each server subprocess uses its own ID space. The proxy rewrites request IDs
// when forwarding downstream and correlates responses back to the original
// client-assigned ID before writing to stdout.
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mseep/internal/config"
)

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 types (minimal subset needed for routing)
// ---------------------------------------------------------------------------

// Message is a JSON-RPC 2.0 request or notification.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // number | string | null
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	// Response fields
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// isRequest returns true when the message has a Method (request or notification).
func (m *Message) isRequest() bool { return m.Method != "" }

// isResponse returns true when the message is a response (has Result or Error).
func (m *Message) isResponse() bool { return m.Result != nil || m.Error != nil }

// idString returns a stable string key for the ID (for map lookups).
func (m *Message) idString() string {
	if m.ID == nil {
		return ""
	}
	return string(m.ID)
}

// ---------------------------------------------------------------------------
// Framing: Content-Length delimited JSON-RPC over stdio
// ---------------------------------------------------------------------------

// readMessage reads one framed JSON-RPC message from r.
func readMessage(r *bufio.Reader) (*Message, error) {
	// Parse headers until blank line.
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &msg, nil
}

// writeMessage writes one framed JSON-RPC message to w.
func writeMessage(w io.Writer, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// ---------------------------------------------------------------------------
// Server subprocess management
// ---------------------------------------------------------------------------

// serverProc wraps a running MCP server child process.
type serverProc struct {
	name   string
	server config.Server
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	// pending maps proxy-assigned request IDs back to the client's original ID.
	mu      sync.Mutex
	pending map[string]pendingCall
}

type pendingCall struct {
	clientID  json.RawMessage
	replyCh   chan<- *Message
}

// ---------------------------------------------------------------------------
// Proxy
// ---------------------------------------------------------------------------

// Proxy is the MCP multiplexer. One Proxy instance exists per client session.
type Proxy struct {
	clientName string
	canonPath  string

	mu      sync.RWMutex
	servers map[string]*serverProc // keyed by server name

	// nextID is used to generate proxy-internal request IDs that are unique
	// across all downstream server subprocesses.
	nextID atomic.Int64

	// namespace settings (read from canonical at startup, not re-read on poll)
	namespaceTools bool
	separator      string
	pollInterval   time.Duration

	// toolIndex maps namespaced tool name → server name.
	toolIndex map[string]string
}

// New creates a Proxy for the given client and canonical config path.
func New(clientName, canonPath string) (*Proxy, error) {
	if canonPath == "" {
		var err error
		canonPath, err = config.DefaultPath()
		if err != nil {
			return nil, err
		}
	}

	canon, err := config.Load(canonPath)
	if err != nil {
		return nil, fmt.Errorf("loading canonical: %w", err)
	}

	p := &Proxy{
		clientName:   clientName,
		canonPath:    canonPath,
		servers:      map[string]*serverProc{},
		toolIndex:    map[string]string{},
		namespaceTools: canon.Settings.Wrapper.NamespaceTools,
		separator:    canon.EffectiveNamespaceSeparator(),
		pollInterval: time.Duration(canon.EffectivePollInterval()) * time.Second,
	}

	// Start server subprocesses for all currently enabled servers.
	for _, s := range canon.Servers {
		if s.Enabled && s.Transport != "http" && s.Transport != "sse" {
			if err := p.startServer(s); err != nil {
				fmt.Fprintf(os.Stderr, "[mseep proxy] warn: failed to start %s: %v\n", s.Name, err)
			}
		}
	}

	return p, nil
}

// startServer spawns a child process for the given server and registers it.
// Must be called with mu write-locked or during startup.
func (p *Proxy) startServer(s config.Server) error {
	cmd := exec.Command(s.Command, s.Args...)
	cmd.Env = os.Environ()
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	// Discard server stderr to avoid polluting our client output.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting process: %w", err)
	}

	proc := &serverProc{
		name:    s.Name,
		server:  s,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
		pending: map[string]pendingCall{},
	}
	p.servers[s.Name] = proc

	// Read responses from this server subprocess and dispatch them.
	go p.readServerResponses(proc)

	return nil
}

// stopServer gracefully terminates a server subprocess.
// Must be called with mu write-locked.
func (p *Proxy) stopServer(name string) {
	proc, ok := p.servers[name]
	if !ok {
		return
	}
	_ = proc.stdin.Close()
	// Give it a moment to exit gracefully before killing.
	done := make(chan struct{})
	go func() { _ = proc.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = proc.cmd.Process.Kill()
	}
	delete(p.servers, name)
	fmt.Fprintf(os.Stderr, "[mseep proxy] stopped %s\n", name)
}

// readServerResponses reads JSON-RPC responses from a server subprocess and
// dispatches them to the pending call's reply channel.
func (p *Proxy) readServerResponses(proc *serverProc) {
	for {
		msg, err := readMessage(proc.stdout)
		if err != nil {
			// Subprocess closed stdout (exited). Clean up.
			p.mu.Lock()
			delete(p.servers, proc.name)
			p.mu.Unlock()
			fmt.Fprintf(os.Stderr, "[mseep proxy] server %s exited: %v\n", proc.name, err)
			return
		}
		if msg.isResponse() {
			// Look up the client's original ID and dispatch.
			proxyID := msg.idString()
			proc.mu.Lock()
			call, ok := proc.pending[proxyID]
			if ok {
				delete(proc.pending, proxyID)
			}
			proc.mu.Unlock()
			if ok {
				// Restore original client ID before sending back.
				msg.ID = call.clientID
				call.replyCh <- msg
			}
		}
		// Notifications from servers are currently discarded; a future version
		// could forward tools/list_changed notifications upstream.
	}
}

// ---------------------------------------------------------------------------
// Config polling — hot-reload on canonical changes
// ---------------------------------------------------------------------------

// PollConfig watches canonical.json and starts/stops servers as needed.
// Run in a goroutine; exits when ctx is cancelled.
func (p *Proxy) PollConfig(ctx context.Context) {
	if p.pollInterval == 0 {
		return // polling disabled
	}

	var lastMod time.Time
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(p.canonPath)
			if err != nil {
				continue
			}
			if !info.ModTime().After(lastMod) {
				continue // no change
			}
			lastMod = info.ModTime()

			canon, err := config.Load(p.canonPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[mseep proxy] poll: failed to reload config: %v\n", err)
				continue
			}
			p.reconcileServers(canon)
		}
	}
}

// reconcileServers starts newly enabled servers and stops disabled ones.
func (p *Proxy) reconcileServers(canon *config.Canonical) {
	p.mu.Lock()
	defer p.mu.Unlock()

	desired := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled && s.Transport != "http" && s.Transport != "sse" {
			desired[s.Name] = s
		}
	}

	// Stop servers that are no longer enabled.
	for name := range p.servers {
		if _, keep := desired[name]; !keep {
			fmt.Fprintf(os.Stderr, "[mseep proxy] hot-stop: %s\n", name)
			p.stopServer(name)
		}
	}

	// Start servers that are newly enabled.
	for name, s := range desired {
		if _, running := p.servers[name]; !running {
			fmt.Fprintf(os.Stderr, "[mseep proxy] hot-start: %s\n", name)
			if err := p.startServer(s); err != nil {
				fmt.Fprintf(os.Stderr, "[mseep proxy] hot-start %s failed: %v\n", name, err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Request routing
// ---------------------------------------------------------------------------

// Run reads JSON-RPC messages from stdin and routes them until stdin closes.
// This is the main event loop — call it from the proxy subcommand.
func (p *Proxy) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil // client closed connection
			}
			return fmt.Errorf("read from client: %w", err)
		}

		switch msg.Method {
		case "initialize":
			// Respond immediately with our own capabilities; do not forward.
			resp := p.buildInitializeResponse(msg)
			if err := writeMessage(os.Stdout, resp); err != nil {
				return fmt.Errorf("write initialize response: %w", err)
			}

		case "tools/list":
			resp, err := p.aggregateToolsList(msg)
			if err != nil {
				resp = errorResponse(msg.ID, -32603, err.Error())
			}
			if err := writeMessage(os.Stdout, resp); err != nil {
				return fmt.Errorf("write tools/list response: %w", err)
			}

		case "resources/list":
			resp, err := p.aggregateResourcesList(msg)
			if err != nil {
				resp = errorResponse(msg.ID, -32603, err.Error())
			}
			if err := writeMessage(os.Stdout, resp); err != nil {
				return fmt.Errorf("write resources/list response: %w", err)
			}

		case "prompts/list":
			resp, err := p.aggregatePromptsList(msg)
			if err != nil {
				resp = errorResponse(msg.ID, -32603, err.Error())
			}
			if err := writeMessage(os.Stdout, resp); err != nil {
				return fmt.Errorf("write prompts/list response: %w", err)
			}

		case "tools/call":
			resp, err := p.routeToolCall(msg)
			if err != nil {
				resp = errorResponse(msg.ID, -32603, err.Error())
			}
			if err := writeMessage(os.Stdout, resp); err != nil {
				return fmt.Errorf("write tools/call response: %w", err)
			}

		default:
			// For all other methods, broadcast to all servers and return the
			// first successful response (or an aggregate error).
			resp := p.broadcast(msg)
			if resp != nil {
				if err := writeMessage(os.Stdout, resp); err != nil {
					return fmt.Errorf("write response: %w", err)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Capability aggregation
// ---------------------------------------------------------------------------

// buildInitializeResponse returns a proxy-generated initialize response.
func (p *Proxy) buildInitializeResponse(req *Message) *Message {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "mseep-proxy",
			"version": "0.1.0",
		},
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
			"prompts":   map[string]interface{}{},
		},
	}
	raw, _ := json.Marshal(result)
	return &Message{JSONRPC: "2.0", ID: req.ID, Result: raw}
}

// aggregateToolsList collects tools from all running servers, applying
// namespace prefixing when configured.
func (p *Proxy) aggregateToolsList(req *Message) (*Message, error) {
	p.mu.RLock()
	procs := p.snapshotProcs()
	p.mu.RUnlock()

	type toolEntry struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	}
	type toolsResult struct {
		Tools []toolEntry `json:"tools"`
	}

	var allTools []toolEntry
	// Reset tool index (rebuilt here on each list call).
	newIndex := map[string]string{}

	for serverName, proc := range procs {
		resp, err := p.callServer(proc, req.Method, req.Params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[mseep proxy] tools/list from %s failed: %v\n", serverName, err)
			continue
		}
		var result toolsResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			continue
		}
		for _, t := range result.Tools {
			displayName := t.Name
			if p.namespaceTools {
				displayName = serverName + p.separator + t.Name
			}
			newIndex[displayName] = serverName
			allTools = append(allTools, toolEntry{
				Name:        displayName,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	p.mu.Lock()
	p.toolIndex = newIndex
	p.mu.Unlock()

	raw, _ := json.Marshal(toolsResult{Tools: allTools})
	return &Message{JSONRPC: "2.0", ID: req.ID, Result: raw}, nil
}

// aggregateResourcesList collects resources from all running servers.
func (p *Proxy) aggregateResourcesList(req *Message) (*Message, error) {
	p.mu.RLock()
	procs := p.snapshotProcs()
	p.mu.RUnlock()

	type resourceEntry map[string]json.RawMessage
	type resourcesResult struct {
		Resources []resourceEntry `json:"resources"`
	}

	var all []resourceEntry
	for serverName, proc := range procs {
		resp, err := p.callServer(proc, req.Method, req.Params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[mseep proxy] resources/list from %s failed: %v\n", serverName, err)
			continue
		}
		var result resourcesResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			continue
		}
		all = append(all, result.Resources...)
	}

	raw, _ := json.Marshal(resourcesResult{Resources: all})
	return &Message{JSONRPC: "2.0", ID: req.ID, Result: raw}, nil
}

// aggregatePromptsList collects prompts from all running servers.
func (p *Proxy) aggregatePromptsList(req *Message) (*Message, error) {
	p.mu.RLock()
	procs := p.snapshotProcs()
	p.mu.RUnlock()

	type promptEntry map[string]json.RawMessage
	type promptsResult struct {
		Prompts []promptEntry `json:"prompts"`
	}

	var all []promptEntry
	for serverName, proc := range procs {
		resp, err := p.callServer(proc, req.Method, req.Params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[mseep proxy] prompts/list from %s failed: %v\n", serverName, err)
			continue
		}
		var result promptsResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			continue
		}
		all = append(all, result.Prompts...)
	}

	raw, _ := json.Marshal(promptsResult{Prompts: all})
	return &Message{JSONRPC: "2.0", ID: req.ID, Result: raw}, nil
}

// routeToolCall looks up which server owns the tool and forwards the call.
// If namespace prefixing is on, the prefix is stripped from the tool name
// before forwarding so the server sees its original tool name.
func (p *Proxy) routeToolCall(req *Message) (*Message, error) {
	// Extract tool name from params.
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid tools/call params: %w", err)
	}

	p.mu.RLock()
	serverName, ok := p.toolIndex[params.Name]
	proc := p.servers[serverName]
	p.mu.RUnlock()

	if !ok || proc == nil {
		return errorResponse(req.ID, -32601, fmt.Sprintf("tool not found: %s", params.Name)), nil
	}

	// Strip namespace prefix before forwarding.
	forwardName := params.Name
	if p.namespaceTools {
		prefix := serverName + p.separator
		if strings.HasPrefix(forwardName, prefix) {
			forwardName = forwardName[len(prefix):]
		}
	}

	// Rebuild params with un-namespaced name.
	fwdParams, _ := json.Marshal(map[string]interface{}{
		"name":      forwardName,
		"arguments": params.Arguments,
	})
	req.Params = fwdParams

	return p.callServer(proc, req.Method, req.Params)
}

// broadcast sends a request to all servers and returns the first non-error
// response, or a synthesised error if all fail.
func (p *Proxy) broadcast(req *Message) *Message {
	p.mu.RLock()
	procs := p.snapshotProcs()
	p.mu.RUnlock()

	if len(procs) == 0 {
		return errorResponse(req.ID, -32603, "no MCP servers running")
	}

	for _, proc := range procs {
		resp, err := p.callServer(proc, req.Method, req.Params)
		if err == nil && resp.Error == nil {
			resp.ID = req.ID
			return resp
		}
	}
	return errorResponse(req.ID, -32603, "all servers returned errors for "+req.Method)
}

// ---------------------------------------------------------------------------
// Server call with ID remapping and timeout
// ---------------------------------------------------------------------------

// callServer sends a request to a specific server subprocess and waits for
// the response, remapping the request ID in both directions.
func (p *Proxy) callServer(proc *serverProc, method string, params json.RawMessage) (*Message, error) {
	// Assign a unique proxy-internal ID.
	proxyID := p.nextID.Add(1)
	proxyIDRaw, _ := json.Marshal(proxyID)

	replyCh := make(chan *Message, 1)

	proc.mu.Lock()
	proc.pending[strconv.FormatInt(proxyID, 10)] = pendingCall{
		clientID: proxyIDRaw,
		replyCh:  replyCh,
	}
	proc.mu.Unlock()

	// Forward the request with the remapped ID.
	fwd := &Message{
		JSONRPC: "2.0",
		ID:      proxyIDRaw,
		Method:  method,
		Params:  params,
	}
	if err := writeMessage(proc.stdin, fwd); err != nil {
		proc.mu.Lock()
		delete(proc.pending, strconv.FormatInt(proxyID, 10))
		proc.mu.Unlock()
		return nil, fmt.Errorf("write to %s: %w", proc.name, err)
	}

	// Wait for response with a generous timeout.
	select {
	case resp := <-replyCh:
		return resp, nil
	case <-time.After(30 * time.Second):
		proc.mu.Lock()
		delete(proc.pending, strconv.FormatInt(proxyID, 10))
		proc.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response from %s", proc.name)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// snapshotProcs returns a shallow copy of the servers map (caller must hold RLock).
func (p *Proxy) snapshotProcs() map[string]*serverProc {
	snap := make(map[string]*serverProc, len(p.servers))
	for k, v := range p.servers {
		snap[k] = v
	}
	return snap
}

func errorResponse(id json.RawMessage, code int, message string) *Message {
	return &Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
}

// Shutdown gracefully stops all server subprocesses.
func (p *Proxy) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name := range p.servers {
		p.stopServer(name)
	}
}
