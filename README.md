# checkfor

[![Tests](https://github.com/hegner123/checkfor/actions/workflows/test.yml/badge.svg)](https://github.com/hegner123/checkfor/actions/workflows/test.yml)
[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/hegner123/checkfor)](https://goreportcard.com/report/github.com/hegner123/checkfor)

An MCP server tool for searching files in directories with compact JSON output. Designed for AI-optimized, token-efficient verification during refactoring workflows.

## Features

- **MCP server by default** - Optimized for Claude Code integration
- **Multi-directory search** - Search across multiple directories in controlled, single-depth scans
- **Compact JSON output** - 41% smaller than pretty-printed JSON for maximum token efficiency
- **Per-directory results** - Organized output with relative paths for better AI reasoning
- **Exclude filtering** - Filter out unwanted matches with multiple exclude patterns
- **Filter statistics** - Track original vs filtered match counts per directory
- **File extension filtering** - Target specific file types
- **Case-insensitive search** - Optional case-insensitive matching
- **Whole-word matching** - Avoid false positives from partial matches
- **Context lines** - Show N lines before/after matches for understanding
- **Multi-line search** - Search for patterns spanning multiple lines using `\n` in the search string

## Installation

### 1. Build from source

```bash
just build
```

### 2. Install to PATH (Required for MCP mode)

Installing checkfor to your system PATH allows MCP server integration with Claude Code.

**System-wide installation** (recommended):
```bash
just install
```

**User-local installation** (if you don't have sudo access):
```bash
mkdir -p ~/bin
cp checkfor ~/bin/
# Add ~/bin to PATH if not already (add to ~/.bashrc or ~/.zshrc):
export PATH="$HOME/bin:$PATH"
```

Verify installation:
```bash
checkfor --help
```

### 3. MCP Server Setup (For Claude Code Integration)

To use checkfor as an MCP tool in Claude Code:

**Step 1:** Add to your Claude Code MCP configuration (project `.mcp.json` or global `~/.claude/mcp.json`):

```json
{
  "mcpServers": {
    "checkfor": {
      "command": "checkfor"
    }
  }
}
```

Note: The `--mcp` flag is no longer needed - MCP mode is now the default behavior.

**Step 2:** Restart Claude Code or reload MCP servers

**Step 3 (Optional but Recommended):** Optimize Claude Code's tool selection

Add this to your global `~/.claude/CLAUDE.md` to help Claude Code automatically choose checkfor for verification tasks:

```markdown
## Tool Usage - Search Optimization

### When to use checkfor (MCP tool)
Use the `checkfor` tool for controlled multi-directory searches at single depth (non-recursive). This tool is optimized for token efficiency with compact JSON output and per-directory results.

**Use checkfor when:**
- Verifying refactoring completion across multiple packages/directories
- Checking specific directories for remaining instances of old patterns
- Searching files without recursing into subdirectories
- You need minimal, token-efficient output with per-directory statistics
- Filtering out specific patterns with exclude filters

**Example:**

checkfor tool with:
- dir: ["/path/to/pkg/handlers", "/path/to/pkg/models"] (optional, defaults to current directory)
- search: "oldFunctionName" (required)
- ext: ".go" (optional, filters by extension)
- exclude: ["oldFunctionNames", "testOldFunction"] (optional, filters out matches)
- case_insensitive: false (optional)
- whole_word: false (optional)
- context: 0 (optional, number of context lines)
- hide_filter_stats: false (optional, hides original_matches and filtered_matches)


### When NOT to use checkfor
- Recursive/deep directory searches (use Grep instead)
- Complex regex patterns (use Grep instead)
- When you need full file content (use Read instead)
```

This helps Claude Code automatically choose the most efficient tool for verification tasks.

### 4. Development (Optional)

Run tests to verify your installation:

```bash
just test
```

Run tests with coverage:

```bash
just test-ci
```

## Updating

checkfor includes automatic update notifications. When running as an MCP server, it checks for new versions every 6 hours and notifies you via stderr if an update is available.

### Update to Latest Version

```bash
checkfor --update
```

This will:
1. Check GitHub for the latest release
2. Download and install the latest version using `go install`
3. Notify you to restart your MCP server

### Manual Update

You can also update manually using Go:

```bash
go install github.com/hegner123/checkfor@latest
```

### Update Notifications

When an update is available, you'll see:

```
[checkfor] Update available: v1.0.0 → v1.2.0
[checkfor] GitHub: https://github.com/hegner123/checkfor/releases/tag/v1.2.0
[checkfor] To update: checkfor --update
```

Update checks respect GitHub API rate limits (60 requests/hour for unauthenticated requests) by caching results in `~/.checkfor-update-cache`.

## Usage

### MCP Mode (Default)

By default, checkfor runs as an MCP server:

```bash
checkfor
```

This starts the MCP server and waits for JSON-RPC requests on stdin. This is the primary mode for Claude Code integration.

### CLI Mode

To use checkfor in CLI mode (for testing or scripting), add the `--cli` flag:

```bash
checkfor --cli --search <string> [options]
```

### Required Flags

- `--search` - String to search for

### Optional Flags

- `--cli` - Run in CLI mode (default is MCP server mode)
- `--dir` - Comma-separated list of directories to search (defaults to current directory)
- `--ext` - File extension to filter (e.g., `.go`, `.txt`, `.js`)
- `--exclude` - Comma-separated list of strings to exclude from results
- `--case-insensitive` - Perform case-insensitive search
- `--whole-word` - Match whole words only
- `--context` - Number of context lines before and after each match (default: 0)
- `--hide-filter-stats` - Hide original_matches and filtered_matches from output

## Examples

### Basic search (current directory)
```bash
checkfor --cli --search "oldFunctionName"
```

### Search specific directory
```bash
checkfor --cli --dir ./src --search "oldFunctionName"
```

### Search multiple directories
```bash
checkfor --cli --dir "./pkg/handlers,./pkg/models,./pkg/services" --search "UserModel" --ext .go
```

### Search with exclude filter
```bash
checkfor --cli --dir ./pkg --search "m.Table" --exclude "m.Tables,m.TableName" --ext .go
```

### Case-insensitive search
```bash
checkfor --cli --search "todo" --case-insensitive
```

### Whole word matching
```bash
checkfor --cli --search "log" --whole-word --ext .go
```

### With context lines
```bash
checkfor --cli --dir ./services --search "deprecated" --context 2
```

### Hide filter statistics
```bash
checkfor --cli --search "old" --exclude "older" --hide-filter-stats
```

### Multi-line search (MCP mode via JSON)

In MCP mode, include `\n` in the search string to match across lines:

```json
{
  "search": "if err != nil {\n\treturn err\n}"
}
```

In CLI mode, multi-line searches require shell escaping:
```bash
checkfor --cli --search $'if err != nil {\n\treturn err\n}'
```

## Best Practices

- **Plan verification steps:** Include specific checkfor searches in your refactoring plans
- **Verify incrementally:** Check progress after each major change
- **Track progress per directory:** Use per-directory match counts to see which packages are clean
- **Use exclude filters:** Filter out known false positives or related patterns
- **Monitor filter stats:** Track `original_matches` vs `filtered_matches` to verify exclude patterns work correctly
- **Use whole-word matching:** Add `--whole-word` flag to avoid false positives from similar names
- **Add context when needed:** Use `--context 1` or `--context 2` to understand surrounding code

## Output Format

The tool outputs compact JSON with per-directory results for optimal AI consumption:

### Single Directory

```json
{"directories":[{"dir":".","matches_found":2,"files":[{"path":"handler.go","matches":[{"line":42,"content":"  result := oldFunctionName()"}]}]}]}
```

### Multiple Directories with Exclude Filter

```json
{"directories":[{"dir":"./pkg/handlers","matches_found":3,"original_matches":5,"filtered_matches":2,"files":[{"path":"user.go","matches":[{"line":42,"content":"m.Table(\"users\")"}]}]},{"dir":"./pkg/models","matches_found":0,"files":[]}]}
```

### Pretty-printed Example (for documentation)

```json
{
  "directories": [
    {
      "dir": "./pkg/handlers",
      "matches_found": 2,
      "original_matches": 4,
      "filtered_matches": 2,
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
    },
    {
      "dir": "./pkg/models",
      "matches_found": 0,
      "files": []
    }
  ]
}
```

### Output Fields

**Per Directory:**
- `dir` - Directory path
- `matches_found` - Total matches in this directory after filtering
- `original_matches` - Total matches before filtering (only when exclude is used and not hidden)
- `filtered_matches` - Number of matches filtered out (only when exclude is used and not hidden)
- `files` - Array of files with matches (relative paths)

**Per File:**
- `path` - File path relative to directory
- `matches` - Array of match objects

**Per Match:**
- `line` - Line number (start line for multi-line matches)
- `end_line` - End line number (only present for multi-line matches)
- `content` - Matched content (may contain `\n` for multi-line matches)
- `context_before` - Lines before match (if --context specified)
- `context_after` - Lines after match (if --context specified)

## Performance

checkfor is designed for token efficiency during AI-assisted refactoring workflows:

### Token Savings

- **Compact JSON:** 41% smaller than pretty-printed JSON
- **Relative paths:** 68% reduction in path data for multi-file results
- **Per-directory structure:** Direct access to statistics without parsing

### Real-World Comparison

In a multi-phase refactoring session:

- **~8,000 tokens** used with checkfor
- **~155,250 tokens** would have been used with Read tool (19.4x more efficient)
- **~35,100 tokens** would have been used with Grep tool (4.4x more efficient)

This efficiency prevented exceeding the 200K token context limit and enabled completion in a single session, saving approximately $1.77 in API costs.

**Key benefits:**
- 95% token reduction vs Read tool
- 77% token reduction vs Grep tool
- 41% reduction via compact JSON output
- 68% path data reduction via relative paths
- 3-5x faster response time
- Near-zero error rate with exact counts
- Better AI reasoning with per-directory results

See [detailed case study](docs/CASE_STUDY.md) for full analysis with real-world data.

## Architecture

- **Default mode:** MCP server (JSON-RPC 2.0 over stdin/stdout)
- **CLI mode:** Direct JSON output (requires `--cli` flag)
- **Single-depth:** Non-recursive scanning per directory
- **Multi-directory:** Controlled searches across specific directories
- **Per-directory results:** Organized output with directory-specific statistics

## Exit Codes

- `0` - Success (matches found or not)
- `1` - Error (invalid arguments, directory not found, etc.)

## Platform Support

- macOS
- Linux

Windows is not supported.
