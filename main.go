package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Match struct {
	Line          int      `json:"line"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

type FileMatch struct {
	Path    string  `json:"path"`
	Matches []Match `json:"matches"`
}

type Result struct {
	MatchesFound int         `json:"matches_found"`
	Files        []FileMatch `json:"files"`
}

type Config struct {
	Dir             string
	Search          string
	Ext             string
	CaseInsensitive bool
	WholeWord       bool
	Context         int
	MCPMode         bool
}

// MCP JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools map[string]bool `json:"tools"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ToolCallResult struct {
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func main() {
	config := parseFlags()

	if config.MCPMode {
		runMCPServer()
		return
	}

	runCLI(config)
}

func parseFlags() Config {
	config := Config{}

	flag.BoolVar(&config.MCPMode, "mcp", false, "Run as MCP server")
	flag.StringVar(&config.Dir, "dir", "", "Directory to search (required in CLI mode)")
	flag.StringVar(&config.Search, "search", "", "String to search for (required in CLI mode)")
	flag.StringVar(&config.Ext, "ext", "", "File extension to filter (e.g., .go, .txt)")
	flag.BoolVar(&config.CaseInsensitive, "case-insensitive", false, "Perform case-insensitive search")
	flag.BoolVar(&config.WholeWord, "whole-word", false, "Match whole words only")
	flag.IntVar(&config.Context, "context", 0, "Number of context lines before and after match")

	flag.Parse()

	return config
}

func runCLI(config Config) {
	if config.Dir == "" || config.Search == "" {
		fmt.Fprintln(os.Stderr, "Error: --dir and --search are required")
		flag.Usage()
		os.Exit(1)
	}

	result, err := searchDirectory(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}

func runMCPServer() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "Parse error")
			continue
		}

		handleRequest(req)
	}
}

func handleRequest(req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		handleInitialize(req)
	case "tools/list":
		handleToolsList(req)
	case "tools/call":
		handleToolsCall(req)
	default:
		sendError(req.ID, -32601, "Method not found")
	}
}

func handleInitialize(req JSONRPCRequest) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "checkfor",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools: map[string]bool{
				"list": true,
				"call": true,
			},
		},
	}
	sendResponse(req.ID, result)
}

func handleToolsList(req JSONRPCRequest) {
	result := ToolsListResult{
		Tools: []Tool{
			{
				Name:        "checkfor",
				Description: "Search files in a directory for a string pattern. Single-depth (non-recursive) scanning with optional extension filtering, case-insensitive search, whole-word matching, and context lines.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"dir": {
							Type:        "string",
							Description: "Directory path to search (absolute path required)",
						},
						"search": {
							Type:        "string",
							Description: "String pattern to search for",
						},
						"ext": {
							Type:        "string",
							Description: "File extension to filter (e.g., '.go', '.txt'). Optional.",
						},
						"case_insensitive": {
							Type:        "boolean",
							Description: "Perform case-insensitive search. Optional, defaults to false.",
							Default:     false,
						},
						"whole_word": {
							Type:        "boolean",
							Description: "Match whole words only. Optional, defaults to false.",
							Default:     false,
						},
						"context": {
							Type:        "integer",
							Description: "Number of context lines before and after each match. Optional, defaults to 0.",
							Default:     0,
						},
					},
					Required: []string{"dir", "search"},
				},
			},
		},
	}
	sendResponse(req.ID, result)
}

func handleToolsCall(req JSONRPCRequest) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params")
		return
	}

	if params.Name != "checkfor" {
		sendError(req.ID, -32602, "Unknown tool")
		return
	}

	dir, ok := params.Arguments["dir"].(string)
	if !ok {
		sendError(req.ID, -32602, "Missing or invalid 'dir' parameter")
		return
	}

	search, ok := params.Arguments["search"].(string)
	if !ok {
		sendError(req.ID, -32602, "Missing or invalid 'search' parameter")
		return
	}

	config := Config{
		Dir:    dir,
		Search: search,
	}

	if ext, ok := params.Arguments["ext"].(string); ok {
		config.Ext = ext
	}

	if caseInsensitive, ok := params.Arguments["case_insensitive"].(bool); ok {
		config.CaseInsensitive = caseInsensitive
	}

	if wholeWord, ok := params.Arguments["whole_word"].(bool); ok {
		config.WholeWord = wholeWord
	}

	if context, ok := params.Arguments["context"].(float64); ok {
		config.Context = int(context)
	}

	result, err := searchDirectory(config)
	if err != nil {
		sendError(req.ID, -32603, fmt.Sprintf("Search failed: %v", err))
		return
	}

	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		sendError(req.ID, -32603, "Failed to marshal result")
		return
	}

	response := ToolCallResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: string(jsonResult),
			},
		},
	}

	sendResponse(req.ID, response)
}

func sendResponse(id any, result any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func sendError(id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal error response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func searchDirectory(config Config) (*Result, error) {
	entries, err := os.ReadDir(config.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	result := &Result{
		Files: make([]FileMatch, 0),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		if config.Ext != "" && !strings.HasSuffix(filename, config.Ext) {
			continue
		}

		fullPath := filepath.Join(config.Dir, filename)
		matches, err := searchFile(fullPath, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to search %s: %v\n", fullPath, err)
			continue
		}

		if len(matches) > 0 {
			result.Files = append(result.Files, FileMatch{
				Path:    filename,
				Matches: matches,
			})
			result.MatchesFound += len(matches)
		}
	}

	return result, nil
}

func searchFile(path string, config Config) ([]Match, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file %s: %v\n", path, cerr)
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var matches []Match
	searchTerm := config.Search
	if config.CaseInsensitive {
		searchTerm = strings.ToLower(searchTerm)
	}

	for i, line := range lines {
		lineToCheck := line
		if config.CaseInsensitive {
			lineToCheck = strings.ToLower(line)
		}

		found := false
		if config.WholeWord {
			found = containsWholeWord(lineToCheck, searchTerm)
		} else {
			found = strings.Contains(lineToCheck, searchTerm)
		}

		if found {
			match := Match{
				Line:    i + 1,
				Content: line,
			}

			if config.Context > 0 {
				match.ContextBefore = getContextBefore(lines, i, config.Context)
				match.ContextAfter = getContextAfter(lines, i, config.Context)
			}

			matches = append(matches, match)
		}
	}

	return matches, nil
}

func containsWholeWord(text, word string) bool {
	if !strings.Contains(text, word) {
		return false
	}

	startIdx := 0
	for {
		idx := strings.Index(text[startIdx:], word)
		if idx == -1 {
			return false
		}

		actualIdx := startIdx + idx

		beforeOk := actualIdx == 0 || !isWordChar(rune(text[actualIdx-1]))
		afterIdx := actualIdx + len(word)
		afterOk := afterIdx >= len(text) || !isWordChar(rune(text[afterIdx]))

		if beforeOk && afterOk {
			return true
		}

		startIdx = actualIdx + 1
	}
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func getContextBefore(lines []string, currentIdx, count int) []string {
	start := currentIdx - count
	if start < 0 {
		start = 0
	}
	return lines[start:currentIdx]
}

func getContextAfter(lines []string, currentIdx, count int) []string {
	end := currentIdx + count + 1
	if end > len(lines) {
		end = len(lines)
	}
	return lines[currentIdx+1 : end]
}
