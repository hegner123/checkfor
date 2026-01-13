# checkfor

[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/hegner123/checkfor)](https://goreportcard.com/report/github.com/hegner123/checkfor)

A lightweight CLI tool for searching files in a directory with JSON output. Designed for token-efficient verification during refactoring tasks.

## Features

- Single-depth directory scanning (non-recursive)
- File extension filtering
- Case-insensitive search option
- Whole-word matching option
- Context lines (show N lines before/after matches)
- JSON output for easy parsing

## Installation

### Build from source

```bash
go build -o checkfor
```

### Install to system PATH

```bash
sudo cp checkfor /usr/local/bin/
```

Or install to user-local bin (ensure `~/bin` or `~/.local/bin` is in your PATH):

```bash
mkdir -p ~/bin
cp checkfor ~/bin/
```

### Run tests

```bash
go test -v
```

## Usage

```bash
checkfor --dir <directory> --search <string> [options]
```

### Required Flags

- `--dir` - Directory to search
- `--search` - String to search for

### Optional Flags

- `--ext` - File extension to filter (e.g., `.go`, `.txt`, `.js`)
- `--case-insensitive` - Perform case-insensitive search
- `--whole-word` - Match whole words only
- `--context` - Number of context lines before and after each match (default: 0)

## Examples

### Basic search
```bash
checkfor --dir ./src --search "oldFunctionName"
```

### Search Go files only
```bash
checkfor --dir ./pkg/handlers --search "UserModel" --ext .go
```

### Case-insensitive search
```bash
checkfor --dir ./templates --search "todo" --case-insensitive
```

### Whole word matching
```bash
checkfor --dir ./utils --search "log" --whole-word --ext .go
```

### With context lines
```bash
checkfor --dir ./services --search "deprecated" --context 2
```

## Output Format

The tool outputs JSON with the following structure:

```json
{
  "matches_found": 2,
  "files": [
    {
      "path": "handler.go",
      "matches": [
        {
          "line": 42,
          "content": "  result := oldFunctionName()",
          "context_before": [
            "func processRequest() {",
            "  // Process the request"
          ],
          "context_after": [
            "  return result",
            "}"
          ]
        }
      ]
    }
  ]
}
```

## Use Case

This tool is optimized for refactoring verification:
1. You refactor code across multiple files
2. Run `checkfor` to verify no instances of old patterns remain
3. JSON output is minimal and token-efficient
4. Single-depth scanning focuses on specific packages/modules

## MCP Server Mode

`checkfor` can run as an MCP (Model Context Protocol) server for integration with Claude Code and other MCP clients.

### Setup

1. Build and install the binary to your PATH (see Installation above)

2. Add to your Claude Code MCP configuration (`~/.claude/mcp.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "checkfor": {
      "command": "checkfor",
      "args": ["--mcp"]
    }
  }
}
```

3. Restart Claude Code or reload MCP servers

### Usage

Once configured, the `checkfor` tool will be available in Claude Code as an MCP tool with the same parameters as the CLI:

- `dir` - Directory path (absolute path required)
- `search` - String pattern to search for
- `ext` - File extension filter (optional)
- `case_insensitive` - Case-insensitive search (optional)
- `whole_word` - Match whole words only (optional)
- `context` - Number of context lines (optional)

## Exit Codes

- `0` - Success (matches found or not)
- `1` - Error (invalid arguments, directory not found, etc.)
