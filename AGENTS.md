# CLIProxyAPI Cursor Compatibility - Progress Report

## Project Overview
CLIProxyAPI is a proxy server that translates OpenAI-compatible API requests to various backend providers including Antigravity (Google's Claude/Gemini via OAuth).

## Problem Statement
Cursor IDE sends requests in **Claude/Anthropic format**, but CLIProxyAPI was only handling **OpenAI format**. This caused:
1. Tools to be silently dropped during translation
2. Model falling back to XML-style tool output (`<read_file>`, `<write_to_file>`, etc.)
3. Tool results not being properly passed back in multi-turn conversations

---

## Root Cause Analysis

### Issue 1: Tool Definition Format Mismatch
- **Cursor sends (Claude format):**
  ```json
  {"name": "Shell", "description": "...", "input_schema": {"type": "object", "properties": {...}}}
  ```
- **Expected (OpenAI format):**
  ```json
  {"type": "function", "function": {"name": "Shell", "parameters": {...}}}
  ```

### Issue 2: Tool Call Format Mismatch (Assistant Messages)
- **Cursor sends (Claude format):**
  ```json
  {"type": "tool_use", "id": "...", "name": "Shell", "input": {...}}
  ```
- **Expected (OpenAI format):**
  ```json
  {"tool_calls": [{"id": "...", "type": "function", "function": {"name": "Shell", "arguments": "..."}}]}
  ```

### Issue 3: Tool Result Format Mismatch (User Messages)
- **Cursor sends (Claude format):**
  ```json
  {"type": "tool_result", "tool_use_id": "...", "content": [...]}
  ```
- **Expected (OpenAI format):**
  ```json
  {"role": "tool", "tool_call_id": "...", "content": "..."}
  ```

---

## Fixes Implemented

### Checklist

- [x] **Tool Definition Translation** - `antigravity_openai_request.go`
  - Added handling for `{"name": "...", "input_schema": {...}}` format
  - Converts to Gemini's `functionDeclarations` format
  - Location: Lines 356-388 (tool parsing loop)

- [x] **Tool Call Translation (tool_use)** - `antigravity_openai_request.go`
  - Added `case "tool_use":` in assistant content parsing
  - Extracts `id`, `name`, `input` and converts to `functionCall`
  - Location: Lines 284-302

- [x] **Tool Result Translation (tool_result)** - `antigravity_openai_request.go`
  - Added parsing for `tool_result` in user content
  - Extracts `tool_use_id` and `content` for response mapping
  - Location: Lines 145-168 (second pass - toolResponses cache)

- [x] **ID Mapping for Claude Format** - `antigravity_openai_request.go`
  - Extended first pass to also capture `tool_use` IDs from content array
  - Location: Lines 118-130

- [x] **System Instruction Update** - `antigravity_executor.go`
  - Added critical tool usage instructions to prevent XML output
  - Instructs model to use structured `functionCall` mechanism
  - Location: Lines 51-75

- [x] **Debug Logging** - For troubleshooting
  - Added request dumping to `debug_requests/` folder
  - Logs both Cursor request and Antigravity request
  - Location: `openai_handlers.go` and `antigravity_executor.go`

---

## Files Modified

| File | Changes |
|------|---------|
| `internal/translator/antigravity/openai/chat-completions/antigravity_openai_request.go` | Added Claude format handling for tools, tool_use, tool_result |
| `internal/translator/antigravity/openai/chat-completions/antigravity_openai_request_test.go` | 13 unit tests for Claude format translation |

---

## Testing Checklist

- [x] Basic tool calls work (model uses functionCall instead of XML)
- [x] Multi-turn conversations with tool results work
- [x] Empty user messages with only tool_result are properly skipped
- [x] All 21 Cursor tools properly translated
- [ ] Streaming responses work correctly
- [ ] Non-streaming responses work correctly
- [ ] Extended thinking mode works

---

## Known Issues / TODO

- [x] ~~400 error after first tool call~~ - Fixed: empty user messages were being added
- [x] ~~Clean up debug logging after issues resolved~~ - No debug logging was added to this fork
- [x] ~~Add unit tests for Claude format translation~~ - Added 13 comprehensive tests
- [ ] Test with other Antigravity models (gemini-2.5-flash, etc.)

---

## Debug Commands

```bash
# Check latest Cursor request
cat debug_requests/cursor_request_*.json | python -c "import json,sys; d=json.load(sys.stdin); print(json.dumps(d, indent=2))" | head -100

# Check latest Antigravity request
cat debug_requests/antigravity_request_*.json | python -c "import json,sys; d=json.load(sys.stdin); print(json.dumps(d, indent=2))" | head -100

# List debug files
ls -lt debug_requests/ | head -10
```

---

## Reference: Format Comparison

### Cursor/Claude Tool Definition
```json
{
  "name": "Shell",
  "description": "Executes a command...",
  "input_schema": {
    "type": "object",
    "properties": {
      "command": {"type": "string"}
    },
    "required": ["command"]
  }
}
```

### OpenAI Tool Definition
```json
{
  "type": "function",
  "function": {
    "name": "Shell",
    "description": "Executes a command...",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {"type": "string"}
      },
      "required": ["command"]
    }
  }
}
```

### Gemini functionDeclarations
```json
{
  "functionDeclarations": [{
    "name": "Shell",
    "description": "Executes a command...",
    "parametersJsonSchema": {
      "type": "object",
      "properties": {
        "command": {"type": "string"}
      },
      "required": ["command"]
    }
  }]
}
```

---

## Helpful Resources

- [opencode-google-antigravity-auth](https://github.com/shekohex/opencode-google-antigravity-auth) - Reference implementation for Antigravity integration
- `opencode-antigravity/src/plugin/transform/claude.ts` - Claude transform logic
- `opencode-antigravity/src/plugin/transform/gemini.ts` - Gemini transform logic

---

## Fork Workflow Setup

### Repository Remotes

```
origin    -> https://github.com/safzanpirani/CLIProxyAPIPlus.git (your fork)
upstream  -> https://github.com/router-for-me/CLIProxyAPIPlus (official repo)
```

### Branches

| Branch | Purpose |
|--------|---------|
| `main` | Synced with upstream main |
| `feature/cursor-compatible` | Cursor compatibility changes |

### Push Your Changes

```bash
git push origin feature/cursor-compatible
```

### Sync with Upstream (Get Latest from Official Repo)

```bash
# Fetch latest from upstream
git fetch upstream

# Switch to main and merge upstream changes
git checkout main
git merge upstream/main
git push origin main

# Rebase your feature branch onto latest main
git checkout feature/cursor-compatible
git rebase main
git push origin feature/cursor-compatible --force-with-lease
```

### Create a PR to Upstream (Optional)

```bash
gh pr create --repo router-for-me/CLIProxyAPIPlus \
  --base main \
  --head safzanpirani:feature/cursor-compatible \
  --title "feat: add Cursor IDE compatibility for Claude/Anthropic tool format" \
  --body "Adds support for Cursor IDE which sends requests in Claude/Anthropic format."
```

### Quick Build

```bash
go build -o cliproxyapi.exe ./cmd/server
```

### Setup Commands (One-Time)

If you need to set up remotes from scratch:

```bash
# Clone your fork
git clone https://github.com/safzanpirani/CLIProxyAPIPlus.git
cd CLIProxyAPIPlus

# Add upstream remote
git remote add upstream https://github.com/router-for-me/CLIProxyAPIPlus.git

# Verify remotes
git remote -v
```
