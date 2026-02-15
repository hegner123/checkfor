package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const VERSION = "1.0.0"

const (
	updateCacheFile     = ".checkfor-update-cache"
	updateCheckInterval = 6 * time.Hour // Check every 6 hours during alpha
	githubReleasesURL   = "https://api.github.com/repos/hegner123/checkfor/releases/latest"
	httpTimeout         = 3 * time.Second
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type UpdateCache struct {
	LastCheck   time.Time `json:"last_check"`
	LastVersion string    `json:"last_version"`
}

type Match struct {
	Line          int      `json:"line"`
	EndLine       int      `json:"end_line,omitempty"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

type FileMatch struct {
	Path    string  `json:"path"`
	Matches []Match `json:"matches"`
}

type DirectoryResult struct {
	Dir             string      `json:"dir"`
	MatchesFound    int         `json:"matches_found"`
	OriginalMatches int         `json:"original_matches,omitempty"`
	FilteredMatches int         `json:"filtered_matches,omitempty"`
	Files           []FileMatch `json:"files"`
}

type Result struct {
	Directories []DirectoryResult `json:"directories"`
}

type Config struct {
	Dirs            []string
	Files           []string
	Search          string
	Ext             string
	Exclude         []string
	CaseInsensitive bool
	WholeWord       bool
	Context         int
	HideFilterStats bool
	CLIMode         bool
	Update          bool
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

	if config.Update {
		runUpdate()
		return
	}

	if config.CLIMode {
		runCLI(config)
		return
	}

	// Background update check for MCP server mode
	go checkForUpdatesBackground()

	runMCPServer()
}

func parseFlags() Config {
	config := Config{}
	var dirStr string
	var fileStr string
	var excludeStr string

	flag.BoolVar(&config.Update, "update", false, "Update checkfor to the latest version")
	flag.BoolVar(&config.CLIMode, "cli", false, "Run in CLI mode (default is MCP server mode)")
	flag.StringVar(&dirStr, "dir", "", "Comma-separated list of directories to search (defaults to current directory)")
	flag.StringVar(&fileStr, "file", "", "Comma-separated list of files to search")
	flag.StringVar(&config.Search, "search", "", "String to search for (required)")
	flag.StringVar(&config.Ext, "ext", "", "File extension to filter (e.g., .go, .rtf)")
	flag.StringVar(&excludeStr, "exclude", "", "Comma-separated list of strings to exclude from results")
	flag.BoolVar(&config.CaseInsensitive, "case-insensitive", false, "Perform case-insensitive search")
	flag.BoolVar(&config.WholeWord, "whole-word", false, "Match whole words only")
	flag.IntVar(&config.Context, "context", 0, "Number of context lines before and after match")
	flag.BoolVar(&config.HideFilterStats, "hide-filter-stats", false, "Hide original_matches and filtered_matches from output")

	flag.Parse()

	if dirStr != "" {
		config.Dirs = strings.Split(dirStr, ",")
		for i := range config.Dirs {
			config.Dirs[i] = strings.TrimSpace(config.Dirs[i])
		}
	}

	if fileStr != "" {
		config.Files = strings.Split(fileStr, ",")
		for i := range config.Files {
			config.Files[i] = strings.TrimSpace(config.Files[i])
		}
	}

	// Default to current directory only if no dirs and no files specified
	if len(config.Dirs) == 0 && len(config.Files) == 0 {
		config.Dirs = []string{"."}
	}

	if excludeStr != "" {
		config.Exclude = strings.Split(excludeStr, ",")
		for i := range config.Exclude {
			config.Exclude[i] = strings.TrimSpace(config.Exclude[i])
		}
	}

	return config
}

func runCLI(config Config) {
	if config.Search == "" {
		fmt.Fprintln(os.Stderr, "Error: --search is required")
		flag.Usage()
		os.Exit(1)
	}

	result, err := searchDirectories(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	output, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}

func runMCPServer() {
	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		os.Exit(0)
	}()

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

	// Scanner finished - stdin closed or EOF, exit gracefully
	// Don't log anything to stderr on clean shutdown
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
			Version: VERSION,
		},
		Capabilities: Capabilities{
			Tools: map[string]bool{
				"list":        true,
				"call":        true,
				"listChanged": true,
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
				Description: "Search files in directories for a string pattern. Single-depth (non-recursive) scanning with optional extension filtering, case-insensitive search, whole-word matching, context lines, and multi-line search.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"dir": {
							Type:        "array",
							Description: "Array of directory paths to search. Can also accept a single string for backwards compatibility. Defaults to current directory if neither dir nor file is provided.",
						},
						"file": {
							Type:        "array",
							Description: "Array of file paths to search directly. Can also accept a single string. Bypasses directory scanning.",
						},
						"search": {
							Type:        "string",
							Description: "String pattern to search for. Supports multi-line search: \\n in the string matches literal newlines spanning multiple lines.",
						},
						"ext": {
							Type:        "string",
							Description: "File extension to filter (e.g., '.go', '.rtf'). Optional.",
						},
						"exclude": {
							Type:        "array",
							Description: "Array of strings to exclude from results. Matches containing any of these strings will be filtered out. Optional.",
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
						"hide_filter_stats": {
							Type:        "boolean",
							Description: "Hide original_matches and filtered_matches from output. Optional, defaults to false.",
							Default:     false,
						},
					},
					Required: []string{"search"},
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

	search, ok := params.Arguments["search"].(string)
	if !ok {
		sendError(req.ID, -32602, "Missing or invalid 'search' parameter")
		return
	}

	config := Config{
		Search: search,
	}

	if dirParam, exists := params.Arguments["dir"]; exists {
		switch v := dirParam.(type) {
		case string:
			config.Dirs = []string{v}
		case []any:
			config.Dirs = make([]string, 0, len(v))
			for _, d := range v {
				if str, ok := d.(string); ok {
					config.Dirs = append(config.Dirs, str)
				}
			}
		}
	}

	if fileParam, exists := params.Arguments["file"]; exists {
		switch v := fileParam.(type) {
		case string:
			config.Files = []string{v}
		case []any:
			config.Files = make([]string, 0, len(v))
			for _, f := range v {
				if str, ok := f.(string); ok {
					config.Files = append(config.Files, str)
				}
			}
		}
	}

	// Default to current directory only if no dirs and no files specified
	if len(config.Dirs) == 0 && len(config.Files) == 0 {
		config.Dirs = []string{"."}
	}

	if ext, ok := params.Arguments["ext"].(string); ok {
		config.Ext = ext
	}

	if excludeArray, ok := params.Arguments["exclude"].([]any); ok {
		config.Exclude = make([]string, 0, len(excludeArray))
		for _, v := range excludeArray {
			if str, ok := v.(string); ok {
				config.Exclude = append(config.Exclude, str)
			}
		}
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

	if hideFilterStats, ok := params.Arguments["hide_filter_stats"].(bool); ok {
		config.HideFilterStats = hideFilterStats
	}

	result, err := searchDirectories(config)
	if err != nil {
		sendError(req.ID, -32603, fmt.Sprintf("Search failed: %v", err))
		return
	}

	jsonResult, err := json.Marshal(result)
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

// Update checking and installation

func checkForUpdatesBackground() {
	cache, err := loadUpdateCache()
	if err == nil && time.Since(cache.LastCheck) < updateCheckInterval {
		return // Skip check, too soon
	}

	latestVersion, releaseURL, err := fetchLatestVersion()
	if err != nil {
		// Silently fail - don't spam stderr on network issues
		return
	}

	if compareVersions(latestVersion, VERSION) > 0 {
		fmt.Fprintf(os.Stderr, "[checkfor] Update available: v%s → %s\n", VERSION, latestVersion)
		fmt.Fprintf(os.Stderr, "[checkfor] GitHub: %s\n", releaseURL)
		fmt.Fprintf(os.Stderr, "[checkfor] To update: checkfor --update\n")
	}

	// Save cache
	cache = &UpdateCache{
		LastCheck:   time.Now(),
		LastVersion: latestVersion,
	}
	_ = saveUpdateCache(cache) // Ignore errors
}

func runUpdate() {
	fmt.Println("Checking for updates...")

	latestVersion, releaseURL, err := fetchLatestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Current version: v%s\n", VERSION)
	fmt.Printf("Latest version: %s\n", latestVersion)

	comparison := compareVersions(latestVersion, VERSION)
	if comparison <= 0 {
		fmt.Println("You are already on the latest version.")
		return
	}

	fmt.Printf("\nUpdating checkfor to %s...\n", latestVersion)
	fmt.Printf("Release: %s\n\n", releaseURL)

	// Run go install
	cmd := exec.Command("go", "install", "github.com/hegner123/checkfor@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nUpdate complete! Restart your MCP server to use the new version.")
}

func fetchLatestVersion() (version, url string, err error) {
	client := &http.Client{Timeout: httpTimeout}

	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return "", "", err
	}

	// Set User-Agent to identify our tool
	req.Header.Set("User-Agent", fmt.Sprintf("checkfor/%s", VERSION))

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	// Strip 'v' prefix if present
	version = strings.TrimPrefix(release.TagName, "v")

	return version, release.HTMLURL, nil
}

func compareVersions(v1, v2 string) int {
	// Simple semantic version comparison (major.minor.patch)
	// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal

	parts1 := strings.Split(strings.TrimPrefix(v1, "v"), ".")
	parts2 := strings.Split(strings.TrimPrefix(v2, "v"), ".")

	for i := 0; i < 3; i++ {
		var n1, n2 int

		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}

func getUpdateCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, updateCacheFile), nil
}

func loadUpdateCache() (*UpdateCache, error) {
	cachePath, err := getUpdateCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache UpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

func saveUpdateCache(cache *UpdateCache) error {
	cachePath, err := getUpdateCachePath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func searchDirectories(config Config) (*Result, error) {
	result := &Result{
		Directories: make([]DirectoryResult, 0, len(config.Dirs)+1),
	}

	for _, dir := range config.Dirs {
		dirResult, err := searchDirectory(dir, config)
		if err != nil {
			return nil, err
		}

		result.Directories = append(result.Directories, *dirResult)
	}

	// Handle direct file searches
	if len(config.Files) > 0 {
		fileResult, err := searchFiles(config)
		if err != nil {
			return nil, err
		}
		result.Directories = append(result.Directories, *fileResult)
	}

	return result, nil
}

func searchFiles(config Config) (*DirectoryResult, error) {
	dirResult := &DirectoryResult{
		Dir:   "(files)",
		Files: make([]FileMatch, 0),
	}

	for _, filePath := range config.Files {
		// Apply extension filter if specified
		if config.Ext != "" && !strings.HasSuffix(filePath, config.Ext) {
			continue
		}

		matches, originalCount, filteredCount, err := searchFile(filePath, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to search %s: %v\n", filePath, err)
			continue
		}

		if !config.HideFilterStats && len(config.Exclude) > 0 {
			dirResult.OriginalMatches += originalCount
			dirResult.FilteredMatches += filteredCount
		}

		if len(matches) > 0 {
			dirResult.Files = append(dirResult.Files, FileMatch{
				Path:    filePath,
				Matches: matches,
			})
			dirResult.MatchesFound += len(matches)
		}
	}

	return dirResult, nil
}

func searchDirectory(dir string, config Config) (*DirectoryResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	dirResult := &DirectoryResult{
		Dir:   dir,
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

		fullPath := filepath.Join(dir, filename)
		matches, originalCount, filteredCount, err := searchFile(fullPath, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to search %s: %v\n", fullPath, err)
			continue
		}

		if !config.HideFilterStats && len(config.Exclude) > 0 {
			dirResult.OriginalMatches += originalCount
			dirResult.FilteredMatches += filteredCount
		}

		if len(matches) > 0 {
			dirResult.Files = append(dirResult.Files, FileMatch{
				Path:    filename,
				Matches: matches,
			})
			dirResult.MatchesFound += len(matches)
		}
	}

	return dirResult, nil
}

func isMultiline(search string) bool {
	return strings.Contains(search, "\n")
}

// indexOfWholeWord finds the first whole-word occurrence of word in text,
// returning the byte index or -1 if not found.
func indexOfWholeWord(text, word string) int {
	offset := 0
	for {
		idx := strings.Index(text[offset:], word)
		if idx == -1 {
			return -1
		}

		actualIdx := offset + idx

		beforeOk := actualIdx == 0 || !isWordChar(rune(text[actualIdx-1]))
		afterIdx := actualIdx + len(word)
		afterOk := afterIdx >= len(text) || !isWordChar(rune(text[afterIdx]))

		if beforeOk && afterOk {
			return actualIdx
		}

		offset = actualIdx + 1
	}
}

func searchFileMultiline(path string, config Config) ([]Match, int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}

	content := string(data)

	// Detect line ending style
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}

	// Normalize search string to match file's line endings
	search := config.Search
	if lineEnding == "\r\n" {
		search = strings.ReplaceAll(search, "\n", "\r\n")
	}

	searchTerm := search
	contentToSearch := content
	if config.CaseInsensitive {
		searchTerm = strings.ToLower(search)
		contentToSearch = strings.ToLower(content)
	}

	// Split content into lines for context extraction
	lines := strings.Split(content, lineEnding)

	var matches []Match
	originalCount := 0
	filteredCount := 0

	offset := 0
	for {
		var idx int
		if config.WholeWord {
			idx = indexOfWholeWord(contentToSearch[offset:], searchTerm)
		} else {
			idx = strings.Index(contentToSearch[offset:], searchTerm)
		}

		if idx == -1 {
			break
		}

		actualIdx := offset + idx
		matchEnd := actualIdx + len(searchTerm)

		originalCount++

		// Extract matched content from original (preserving case)
		matchedContent := content[actualIdx:matchEnd]

		// Determine start and end line numbers
		startLine := strings.Count(content[:actualIdx], "\n") + 1
		endLine := startLine
		if matchEnd > actualIdx {
			endLine = strings.Count(content[:matchEnd-1], "\n") + 1
		}

		// Check exclude patterns against all lines the match spans
		excluded := false
		if len(config.Exclude) > 0 {
			// Find the start of the first line containing the match
			lineStart := strings.LastIndex(content[:actualIdx], lineEnding)
			if lineStart == -1 {
				lineStart = 0
			} else {
				lineStart += len(lineEnding)
			}

			// Find the end of the last line containing the match
			lineEndPos := strings.Index(content[matchEnd:], lineEnding)
			if lineEndPos == -1 {
				lineEndPos = len(content)
			} else {
				lineEndPos = matchEnd + lineEndPos
			}

			spanningLines := content[lineStart:lineEndPos]

			for _, excludePattern := range config.Exclude {
				patternToCheck := excludePattern
				linesToCheck := spanningLines
				if config.CaseInsensitive {
					patternToCheck = strings.ToLower(excludePattern)
					linesToCheck = strings.ToLower(spanningLines)
				}
				if strings.Contains(linesToCheck, patternToCheck) {
					excluded = true
					break
				}
			}
		}

		if excluded {
			filteredCount++
		} else {
			match := Match{
				Line:    startLine,
				Content: matchedContent,
			}

			if startLine != endLine {
				match.EndLine = endLine
			}

			if config.Context > 0 {
				match.ContextBefore = getContextBefore(lines, startLine-1, config.Context)
				match.ContextAfter = getContextAfter(lines, endLine-1, config.Context)
			}

			matches = append(matches, match)
		}

		offset = matchEnd
	}

	return matches, originalCount, filteredCount, nil
}

func searchFile(path string, config Config) ([]Match, int, int, error) {
	if isMultiline(config.Search) {
		return searchFileMultiline(path, config)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
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
		return nil, 0, 0, err
	}

	var matches []Match
	originalCount := 0
	filteredCount := 0
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
			originalCount++
			excluded := false
			for _, excludePattern := range config.Exclude {
				excludeToCheck := excludePattern
				lineForExclude := line
				if config.CaseInsensitive {
					excludeToCheck = strings.ToLower(excludePattern)
					lineForExclude = lineToCheck
				}
				if strings.Contains(lineForExclude, excludeToCheck) {
					excluded = true
					break
				}
			}

			if excluded {
				filteredCount++
			} else {
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
	}

	return matches, originalCount, filteredCount, nil
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
