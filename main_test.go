package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test helper: create temporary directory with test files
func setupTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "checkfor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tmpDir
}

func cleanupTestDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Errorf("Failed to cleanup temp dir: %v", err)
	}
}

func createTestFile(t *testing.T, dir, name, content string) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}

// Core search function tests

func TestContainsWholeWord(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		word     string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"word in sentence", "hello world", "hello", true},
		{"word at end", "say hello", "hello", true},
		{"word with punctuation", "hello, world", "hello", true},
		{"partial match should fail", "helloworld", "hello", false},
		{"substring should fail", "superhello", "hello", false},
		{"underscore is word char", "hello_world", "hello", false},
		{"space is word boundary", "hello world", "world", true},
		{"case sensitive", "Hello", "hello", false},
		{"multiple occurrences", "log logger log", "log", true},
		{"no match", "goodbye", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsWholeWord(tt.text, tt.word)
			if result != tt.expected {
				t.Errorf("containsWholeWord(%q, %q) = %v, want %v",
					tt.text, tt.word, result, tt.expected)
			}
		})
	}
}

func TestIsWordChar(t *testing.T) {
	wordChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	for _, ch := range wordChars {
		if !isWordChar(ch) {
			t.Errorf("isWordChar(%q) = false, want true", ch)
		}
	}

	nonWordChars := " !@#$%^&*()-+=[]{}|;:'\",.<>?/\\"
	for _, ch := range nonWordChars {
		if isWordChar(ch) {
			t.Errorf("isWordChar(%q) = true, want false", ch)
		}
	}
}

func TestGetContextBefore(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}

	tests := []struct {
		name        string
		currentIdx  int
		count       int
		expected    []string
		description string
	}{
		{"middle with 2 lines", 3, 2, []string{"line2", "line3"}, "Get 2 lines before index 3"},
		{"start boundary", 1, 2, []string{"line1"}, "Only 1 line available before index 1"},
		{"at start", 0, 2, []string{}, "No lines before index 0"},
		{"no context", 2, 0, []string{}, "Count is 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContextBefore(lines, tt.currentIdx, tt.count)
			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetContextAfter(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}

	tests := []struct {
		name       string
		currentIdx int
		count      int
		expected   []string
	}{
		{"middle with 2 lines", 2, 2, []string{"line4", "line5"}},
		{"end boundary", 3, 2, []string{"line5"}},
		{"at end", 4, 2, []string{}},
		{"no context", 2, 0, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContextAfter(lines, tt.currentIdx, tt.count)
			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// File filtering tests

func TestSearchFileCaseInsensitive(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "Hello World\nGoodbye World\nHELLO again"
	createTestFile(t, tmpDir, "test.rtf", content)

	config := Config{
		Search:          "hello",
		CaseInsensitive: true,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.rtf"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}

	if matches[0].Line != 1 || matches[1].Line != 3 {
		t.Errorf("Expected lines 1 and 3, got %d and %d", matches[0].Line, matches[1].Line)
	}
}

func TestSearchFileWholeWord(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "log message\nlogger info\nlog\ncatalog"
	createTestFile(t, tmpDir, "test.rtf", content)

	config := Config{
		Search:    "log",
		WholeWord: true,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.rtf"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (lines 1 and 3), got %d", len(matches))
	}
}

func TestSearchFileWithContext(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "line1\nline2\ntarget\nline4\nline5"
	createTestFile(t, tmpDir, "test.rtf", content)

	config := Config{
		Search:  "target",
		Context: 1,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.rtf"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	match := matches[0]
	if len(match.ContextBefore) != 1 || match.ContextBefore[0] != "line2" {
		t.Errorf("Expected context before to be [line2], got %v", match.ContextBefore)
	}
	if len(match.ContextAfter) != 1 || match.ContextAfter[0] != "line4" {
		t.Errorf("Expected context after to be [line4], got %v", match.ContextAfter)
	}
}

func TestSearchFileWithExclude(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "m.Table user\nm.Tables schema\nm.Table orders\nm.TableName config"
	createTestFile(t, tmpDir, "test.go", content)

	config := Config{
		Search:  "m.Table",
		Exclude: []string{"m.Tables", "m.TableName"},
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.go"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (lines 1 and 3), got %d", len(matches))
	}

	if len(matches) >= 2 {
		if matches[0].Line != 1 || matches[1].Line != 3 {
			t.Errorf("Expected lines 1 and 3, got %d and %d", matches[0].Line, matches[1].Line)
		}
	}
}

func TestSearchFileWithExcludeCaseInsensitive(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "m.Table user\nM.TABLES schema\nm.table orders"
	createTestFile(t, tmpDir, "test.go", content)

	config := Config{
		Search:          "m.table",
		Exclude:         []string{"m.tables"},
		CaseInsensitive: true,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.go"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (lines 1 and 3), got %d", len(matches))
	}
}

func TestSearchDirectoryWithFilterStats(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	createTestFile(t, tmpDir, "file1.go", "m.Table user\nm.Tables schema\nm.Table orders\nm.TableName config")
	createTestFile(t, tmpDir, "file2.go", "m.Table product\nm.Table category")

	config := Config{
		Dirs:    []string{tmpDir},
		Search:  "m.Table",
		Exclude: []string{"m.Tables", "m.TableName"},
		Ext:     ".go",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectory failed: %v", err)
	}

	if len(result.Directories) != 1 {
		t.Fatalf("Expected 1 directory, got %d", len(result.Directories))
	}

	dir := result.Directories[0]

	if dir.MatchesFound != 4 {
		t.Errorf("Expected 4 matches found, got %d", dir.MatchesFound)
	}

	if dir.OriginalMatches != 6 {
		t.Errorf("Expected 6 original matches, got %d", dir.OriginalMatches)
	}

	if dir.FilteredMatches != 2 {
		t.Errorf("Expected 2 filtered matches, got %d", dir.FilteredMatches)
	}
}

func TestSearchDirectoryHideFilterStats(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	createTestFile(t, tmpDir, "file1.go", "m.Table user\nm.Tables schema\nm.Table orders")

	config := Config{
		Dirs:            []string{tmpDir},
		Search:          "m.Table",
		Exclude:         []string{"m.Tables"},
		HideFilterStats: true,
		Ext:             ".go",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectory failed: %v", err)
	}

	if len(result.Directories) != 1 {
		t.Fatalf("Expected 1 directory, got %d", len(result.Directories))
	}

	dir := result.Directories[0]

	if dir.MatchesFound != 2 {
		t.Errorf("Expected 2 matches found, got %d", dir.MatchesFound)
	}

	if dir.OriginalMatches != 0 {
		t.Errorf("Expected 0 original matches (hidden), got %d", dir.OriginalMatches)
	}

	if dir.FilteredMatches != 0 {
		t.Errorf("Expected 0 filtered matches (hidden), got %d", dir.FilteredMatches)
	}
}

// Integration tests

func TestSearchDirectoryExtensionFilter(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	createTestFile(t, tmpDir, "file1.go", "package main")
	createTestFile(t, tmpDir, "file2.rtf", "package main")
	createTestFile(t, tmpDir, "file3.go", "package main")

	config := Config{
		Dirs:   []string{tmpDir},
		Search: "package",
		Ext:    ".go",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectory failed: %v", err)
	}

	if len(result.Directories) != 1 {
		t.Fatalf("Expected 1 directory, got %d", len(result.Directories))
	}

	dir := result.Directories[0]

	if len(dir.Files) != 2 {
		t.Errorf("Expected 2 .go files with matches, got %d", len(dir.Files))
	}

	if dir.MatchesFound != 2 {
		t.Errorf("Expected 2 total matches, got %d", dir.MatchesFound)
	}
}

func TestSearchDirectoryNoMatches(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	createTestFile(t, tmpDir, "file.rtf", "hello world")

	config := Config{
		Dirs:   []string{tmpDir},
		Search: "nonexistent",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectory failed: %v", err)
	}

	if len(result.Directories) != 1 {
		t.Fatalf("Expected 1 directory, got %d", len(result.Directories))
	}

	dir := result.Directories[0]

	if len(dir.Files) != 0 {
		t.Errorf("Expected 0 files with matches, got %d", len(dir.Files))
	}

	if dir.MatchesFound != 0 {
		t.Errorf("Expected 0 matches, got %d", dir.MatchesFound)
	}
}

func TestSearchDirectoryNonRecursive(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	// Create file in root
	createTestFile(t, tmpDir, "root.rtf", "target")

	// Create subdirectory with file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	createTestFile(t, subDir, "sub.rtf", "target")

	config := Config{
		Dirs:   []string{tmpDir},
		Search: "target",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectory failed: %v", err)
	}

	// Should only find the root file, not the subdirectory file
	if len(result.Directories) != 1 {
		t.Fatalf("Expected 1 directory, got %d", len(result.Directories))
	}

	dir := result.Directories[0]

	if len(dir.Files) != 1 {
		t.Errorf("Expected 1 file (non-recursive), got %d", len(dir.Files))
	}

	if dir.Files[0].Path != "root.rtf" {
		t.Errorf("Expected root.rtf, got %s", dir.Files[0].Path)
	}
}

func TestSearchMultipleDirectories(t *testing.T) {
	tmpDir1 := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir1)

	tmpDir2 := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir2)

	// Create files in first directory
	createTestFile(t, tmpDir1, "file1.go", "package main\nfunc main() {}")
	createTestFile(t, tmpDir1, "file2.go", "package utils\nfunc helper() {}")

	// Create files in second directory
	createTestFile(t, tmpDir2, "file3.go", "package models")
	createTestFile(t, tmpDir2, "file4.go", "package handlers\nfunc handler() {}")

	config := Config{
		Dirs:   []string{tmpDir1, tmpDir2},
		Search: "package",
		Ext:    ".go",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectories failed: %v", err)
	}

	// Should find results in both directories
	if len(result.Directories) != 2 {
		t.Fatalf("Expected 2 directories, got %d", len(result.Directories))
	}

	// Verify first directory results
	dir1 := result.Directories[0]
	if dir1.Dir != tmpDir1 {
		t.Errorf("Expected first directory to be %s, got %s", tmpDir1, dir1.Dir)
	}
	if dir1.MatchesFound != 2 {
		t.Errorf("Expected 2 matches in first directory, got %d", dir1.MatchesFound)
	}
	if len(dir1.Files) != 2 {
		t.Errorf("Expected 2 files in first directory, got %d", len(dir1.Files))
	}

	// Verify second directory results
	dir2 := result.Directories[1]
	if dir2.Dir != tmpDir2 {
		t.Errorf("Expected second directory to be %s, got %s", tmpDir2, dir2.Dir)
	}
	if dir2.MatchesFound != 2 {
		t.Errorf("Expected 2 matches in second directory, got %d", dir2.MatchesFound)
	}
	if len(dir2.Files) != 2 {
		t.Errorf("Expected 2 files in second directory, got %d", len(dir2.Files))
	}
}

func TestSearchSpecificFiles(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	// Create test files
	createTestFile(t, tmpDir, "file1.go", "package main\nfunc main() {}")
	createTestFile(t, tmpDir, "file2.go", "package utils\nfunc helper() {}")
	createTestFile(t, tmpDir, "file3.go", "package models\nfunc model() {}")

	file1Path := filepath.Join(tmpDir, "file1.go")
	file3Path := filepath.Join(tmpDir, "file3.go")

	config := Config{
		Files:  []string{file1Path, file3Path},
		Search: "package",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectories failed: %v", err)
	}

	// Should have one directory result labeled "(files)"
	if len(result.Directories) != 1 {
		t.Fatalf("Expected 1 directory result, got %d", len(result.Directories))
	}

	dir := result.Directories[0]
	if dir.Dir != "(files)" {
		t.Errorf("Expected dir to be '(files)', got %s", dir.Dir)
	}

	// Should find matches in both files
	if dir.MatchesFound != 2 {
		t.Errorf("Expected 2 matches, got %d", dir.MatchesFound)
	}

	if len(dir.Files) != 2 {
		t.Errorf("Expected 2 files with matches, got %d", len(dir.Files))
	}
}

func TestSearchFilesWithExtFilter(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	// Create test files
	createTestFile(t, tmpDir, "file1.go", "package main")
	createTestFile(t, tmpDir, "file2.txt", "package main")

	file1Path := filepath.Join(tmpDir, "file1.go")
	file2Path := filepath.Join(tmpDir, "file2.txt")

	config := Config{
		Files:  []string{file1Path, file2Path},
		Search: "package",
		Ext:    ".go",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectories failed: %v", err)
	}

	dir := result.Directories[0]

	// Should only match the .go file
	if dir.MatchesFound != 1 {
		t.Errorf("Expected 1 match (only .go file), got %d", dir.MatchesFound)
	}

	if len(dir.Files) != 1 {
		t.Errorf("Expected 1 file with matches, got %d", len(dir.Files))
	}
}

func TestSearchFilesAndDirectories(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	// Create files in directory
	createTestFile(t, tmpDir, "dir_file.go", "package dir")

	// Create a separate file to search directly
	tmpDir2 := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir2)
	createTestFile(t, tmpDir2, "direct_file.go", "package direct")

	directFilePath := filepath.Join(tmpDir2, "direct_file.go")

	config := Config{
		Dirs:   []string{tmpDir},
		Files:  []string{directFilePath},
		Search: "package",
		Ext:    ".go",
	}

	result, err := searchDirectories(config)
	if err != nil {
		t.Fatalf("searchDirectories failed: %v", err)
	}

	// Should have 2 directory results: one for dir, one for files
	if len(result.Directories) != 2 {
		t.Fatalf("Expected 2 directory results, got %d", len(result.Directories))
	}

	// First should be the actual directory
	if result.Directories[0].Dir != tmpDir {
		t.Errorf("Expected first dir to be %s, got %s", tmpDir, result.Directories[0].Dir)
	}
	if result.Directories[0].MatchesFound != 1 {
		t.Errorf("Expected 1 match in directory, got %d", result.Directories[0].MatchesFound)
	}

	// Second should be "(files)"
	if result.Directories[1].Dir != "(files)" {
		t.Errorf("Expected second dir to be '(files)', got %s", result.Directories[1].Dir)
	}
	if result.Directories[1].MatchesFound != 1 {
		t.Errorf("Expected 1 match in files, got %d", result.Directories[1].MatchesFound)
	}
}

// MCP JSON-RPC handler tests

func TestHandleInitialize(t *testing.T) {
	// Test the initialize result structure directly
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

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "checkfor" {
		t.Errorf("Expected server name checkfor, got %s", result.ServerInfo.Name)
	}

	if !result.Capabilities.Tools["list"] || !result.Capabilities.Tools["call"] {
		t.Errorf("Expected tools capabilities for list and call")
	}
}

func TestToolsListResult(t *testing.T) {
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
					},
					Required: []string{"dir", "search"},
				},
			},
		},
	}

	if len(result.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result.Tools))
	}

	tool := result.Tools[0]
	if tool.Name != "checkfor" {
		t.Errorf("Expected tool name checkfor, got %s", tool.Name)
	}

	if len(tool.InputSchema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(tool.InputSchema.Required))
	}

	if tool.InputSchema.Properties["dir"].Type != "string" {
		t.Errorf("Expected dir property to be string type")
	}
}

func TestToolCallParamsMarshaling(t *testing.T) {
	jsonData := `{
		"name": "checkfor",
		"arguments": {
			"dir": "/test/path",
			"search": "pattern",
			"case_insensitive": true,
			"context": 2
		}
	}`

	var params ToolCallParams
	if err := json.Unmarshal([]byte(jsonData), &params); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if params.Name != "checkfor" {
		t.Errorf("Expected name checkfor, got %s", params.Name)
	}

	if params.Arguments["dir"] != "/test/path" {
		t.Errorf("Expected dir /test/path, got %v", params.Arguments["dir"])
	}

	if params.Arguments["search"] != "pattern" {
		t.Errorf("Expected search pattern, got %v", params.Arguments["search"])
	}

	if params.Arguments["case_insensitive"] != true {
		t.Errorf("Expected case_insensitive true, got %v", params.Arguments["case_insensitive"])
	}

	// JSON numbers unmarshal as float64
	if params.Arguments["context"] != float64(2) {
		t.Errorf("Expected context 2, got %v", params.Arguments["context"])
	}
}

func TestResultJSONOutput(t *testing.T) {
	result := Result{
		Directories: []DirectoryResult{
			{
				Dir:          "/tmp/test",
				MatchesFound: 1,
				Files: []FileMatch{
					{
						Path: "test.go",
						Matches: []Match{
							{
								Line:          42,
								Content:       "target line",
								ContextBefore: []string{"line before"},
								ContextAfter:  []string{"line after"},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(decoded.Directories) != 1 {
		t.Fatalf("Expected 1 directory, got %d", len(decoded.Directories))
	}

	dir := decoded.Directories[0]

	if dir.MatchesFound != 1 {
		t.Errorf("Expected matches_found 1, got %d", dir.MatchesFound)
	}

	if len(dir.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(dir.Files))
	}

	if dir.Files[0].Path != "test.go" {
		t.Errorf("Expected path test.go, got %s", dir.Files[0].Path)
	}

	if dir.Files[0].Matches[0].Line != 42 {
		t.Errorf("Expected line 42, got %d", dir.Files[0].Matches[0].Line)
	}
}

// Multi-line search tests

func TestIsMultiline(t *testing.T) {
	tests := []struct {
		name     string
		search   string
		expected bool
	}{
		{"single line", "hello", false},
		{"contains newline", "hello\nworld", true},
		{"newline only", "\n", true},
		{"empty string", "", false},
		{"trailing newline", "hello\n", true},
		{"leading newline", "\nhello", true},
		{"multiple newlines", "a\nb\nc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMultiline(tt.search)
			if result != tt.expected {
				t.Errorf("isMultiline(%q) = %v, want %v", tt.search, result, tt.expected)
			}
		})
	}
}

func TestIndexOfWholeWord(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		word     string
		expected int
	}{
		{"simple match", "hello world", "hello", 0},
		{"match at end", "say hello", "hello", 4},
		{"no match - partial", "helloworld", "hello", -1},
		{"no match at all", "goodbye", "hello", -1},
		{"match after partial", "superhello hello", "hello", 11},
		{"multi-line text", "func main() {\nfmt.Println", "main", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOfWholeWord(tt.text, tt.word)
			if result != tt.expected {
				t.Errorf("indexOfWholeWord(%q, %q) = %v, want %v", tt.text, tt.word, result, tt.expected)
			}
		})
	}
}

func TestSearchFileMultiline_Basic(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "line1\nline2\nline3\nline4\nline5"
	createTestFile(t, tmpDir, "test.txt", content)

	config := Config{
		Search: "line2\nline3",
	}

	matches, originalCount, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if originalCount != 1 {
		t.Errorf("Expected originalCount 1, got %d", originalCount)
	}

	match := matches[0]
	if match.Line != 2 {
		t.Errorf("Expected start line 2, got %d", match.Line)
	}
	if match.EndLine != 3 {
		t.Errorf("Expected end line 3, got %d", match.EndLine)
	}
	if match.Content != "line2\nline3" {
		t.Errorf("Expected content %q, got %q", "line2\nline3", match.Content)
	}
}

func TestSearchFileMultiline_CaseInsensitive(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "Hello World\nGoodbye World\nHELLO again\nGOODBYE again"
	createTestFile(t, tmpDir, "test.txt", content)

	config := Config{
		Search:          "hello world\ngoodbye",
		CaseInsensitive: true,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if matches[0].Line != 1 {
		t.Errorf("Expected start line 1, got %d", matches[0].Line)
	}
	if matches[0].EndLine != 2 {
		t.Errorf("Expected end line 2, got %d", matches[0].EndLine)
	}
}

func TestSearchFileMultiline_WholeWord(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "func main() {\n\tfmt.Println(\"hello\")\n}\n\nfunc mainHelper() {\n\tfmt.Println(\"helper\")\n}"
	createTestFile(t, tmpDir, "test.go", content)

	config := Config{
		Search:    "main() {\n\tfmt.Println",
		WholeWord: true,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.go"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	// "main" at start should match (word boundary before 'm' is '(' which is not a word char)
	// "mainHelper" should NOT match because 'H' after "main" is a word char
	// But we're searching for "main() {..." not just "main", so the whole search string
	// boundaries matter: before 'm' and after 'n' in "Println"
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if matches[0].Line != 1 {
		t.Errorf("Expected start line 1, got %d", matches[0].Line)
	}
}

func TestSearchFileMultiline_WithExclude(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "func setup() {\n\tdb.Connect()\n}\n\nfunc setup() {\n\tdb.ConnectTest()\n}"
	createTestFile(t, tmpDir, "test.go", content)

	config := Config{
		Search:  "func setup() {\n\tdb.Connect",
		Exclude: []string{"ConnectTest"},
	}

	matches, originalCount, filteredCount, err := searchFile(filepath.Join(tmpDir, "test.go"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if originalCount != 2 {
		t.Errorf("Expected 2 original matches, got %d", originalCount)
	}

	if filteredCount != 1 {
		t.Errorf("Expected 1 filtered match, got %d", filteredCount)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match after exclude, got %d", len(matches))
	}

	if matches[0].Line != 1 {
		t.Errorf("Expected match on line 1, got %d", matches[0].Line)
	}
}

func TestSearchFileMultiline_MultipleOccurrences(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "start\nend\nother\nstart\nend\nmore\nstart\nend"
	createTestFile(t, tmpDir, "test.txt", content)

	config := Config{
		Search: "start\nend",
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 3 {
		t.Fatalf("Expected 3 matches, got %d", len(matches))
	}

	expectedLines := []int{1, 4, 7}
	for i, m := range matches {
		if m.Line != expectedLines[i] {
			t.Errorf("Match %d: expected line %d, got %d", i, expectedLines[i], m.Line)
		}
		if m.EndLine != expectedLines[i]+1 {
			t.Errorf("Match %d: expected end line %d, got %d", i, expectedLines[i]+1, m.EndLine)
		}
	}
}

func TestSearchFileMultiline_WithContext(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "before1\nbefore2\ntarget1\ntarget2\nafter1\nafter2"
	createTestFile(t, tmpDir, "test.txt", content)

	config := Config{
		Search:  "target1\ntarget2",
		Context: 1,
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	match := matches[0]
	if len(match.ContextBefore) != 1 || match.ContextBefore[0] != "before2" {
		t.Errorf("Expected context before [before2], got %v", match.ContextBefore)
	}
	if len(match.ContextAfter) != 1 || match.ContextAfter[0] != "after1" {
		t.Errorf("Expected context after [after1], got %v", match.ContextAfter)
	}
}

func TestSearchFileMultiline_CRLF(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	// File uses \r\n line endings
	content := "line1\r\nline2\r\nline3\r\nline4"
	createTestFile(t, tmpDir, "test.txt", content)

	// Search string uses \n (user input via JSON)
	config := Config{
		Search: "line2\nline3",
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if matches[0].Line != 2 {
		t.Errorf("Expected start line 2, got %d", matches[0].Line)
	}
	if matches[0].EndLine != 3 {
		t.Errorf("Expected end line 3, got %d", matches[0].EndLine)
	}
}

func TestSearchFileMultiline_ThreeLines(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "if err != nil {\n\treturn err\n}\nother code"
	createTestFile(t, tmpDir, "test.go", content)

	config := Config{
		Search: "if err != nil {\n\treturn err\n}",
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.go"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if matches[0].Line != 1 {
		t.Errorf("Expected start line 1, got %d", matches[0].Line)
	}
	if matches[0].EndLine != 3 {
		t.Errorf("Expected end line 3, got %d", matches[0].EndLine)
	}
}

func TestSearchFileMultiline_NoMatch(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "line1\nline2\nline3"
	createTestFile(t, tmpDir, "test.txt", content)

	config := Config{
		Search: "line1\nline3",
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

func TestSearchFileMultiline_SingleLineEndLine(t *testing.T) {
	// When the match spans only one line (shouldn't happen with multiline search
	// in practice, but verify EndLine is omitted)
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	// A file where "a\n" matches but ends at a line boundary
	content := "a\nb\nc"
	createTestFile(t, tmpDir, "test.txt", content)

	config := Config{
		Search: "a\nb",
	}

	matches, _, _, err := searchFile(filepath.Join(tmpDir, "test.txt"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if matches[0].Line != 1 {
		t.Errorf("Expected line 1, got %d", matches[0].Line)
	}
	if matches[0].EndLine != 2 {
		t.Errorf("Expected end line 2, got %d", matches[0].EndLine)
	}
}

func TestSearchFileMultiline_ExcludeCaseInsensitive(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer cleanupTestDir(t, tmpDir)

	content := "func A() {\n\tDEBUG_LOG()\n}\n\nfunc A() {\n\tRun()\n}"
	createTestFile(t, tmpDir, "test.go", content)

	config := Config{
		Search:          "func a() {\n\t",
		Exclude:         []string{"debug"},
		CaseInsensitive: true,
	}

	matches, originalCount, filteredCount, err := searchFile(filepath.Join(tmpDir, "test.go"), config)
	if err != nil {
		t.Fatalf("searchFile failed: %v", err)
	}

	if originalCount != 2 {
		t.Errorf("Expected 2 original matches, got %d", originalCount)
	}
	if filteredCount != 1 {
		t.Errorf("Expected 1 filtered, got %d", filteredCount)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match after exclude, got %d", len(matches))
	}
	if matches[0].Line != 5 {
		t.Errorf("Expected match on line 5, got %d", matches[0].Line)
	}
}

func TestSearchFileMultiline_EndLineJSON(t *testing.T) {
	// Verify EndLine appears in JSON when multi-line, absent when single-line
	match := Match{
		Line:    1,
		EndLine: 3,
		Content: "a\nb\nc",
	}

	data, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, "end_line") {
		t.Errorf("Expected end_line in JSON output: %s", jsonStr)
	}

	// Single-line match should omit end_line
	singleMatch := Match{
		Line:    5,
		Content: "hello",
	}

	data2, err := json.Marshal(singleMatch)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr2 := string(data2)
	if contains(jsonStr2, "end_line") {
		t.Errorf("Expected no end_line in single-line JSON: %s", jsonStr2)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
