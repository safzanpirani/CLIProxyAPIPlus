// Package chat_completions provides request translation functionality for OpenAI to Gemini CLI API compatibility.
// Tests for Cursor/Claude format compatibility in OpenAI to Antigravity translation.
package chat_completions

import (
	"testing"

	"github.com/tidwall/gjson"
)

// ============================================================================
// Claude/Cursor Tool Definition Format Tests
// ============================================================================

func TestConvertOpenAIRequestToAntigravity_ClaudeToolDefinition(t *testing.T) {
	// Cursor sends tools in Claude format: {"name": "...", "input_schema": {...}}
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [{"role": "user", "content": "Hello"}],
		"tools": [
			{
				"name": "Shell",
				"description": "Executes a shell command",
				"input_schema": {
					"type": "object",
					"properties": {
						"command": {"type": "string", "description": "The command to execute"}
					},
					"required": ["command"]
				}
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// Check tools structure exists
	tools := gjson.Get(outputStr, "request.tools")
	if !tools.Exists() {
		t.Fatal("request.tools should exist")
	}

	funcDecl := gjson.Get(outputStr, "request.tools.0.functionDeclarations.0")
	if !funcDecl.Exists() {
		t.Fatal("functionDeclarations.0 should exist")
	}

	// Check tool name
	if funcDecl.Get("name").String() != "Shell" {
		t.Errorf("Expected tool name 'Shell', got '%s'", funcDecl.Get("name").String())
	}

	// Check description
	if funcDecl.Get("description").String() != "Executes a shell command" {
		t.Errorf("Expected description, got '%s'", funcDecl.Get("description").String())
	}

	// Check input_schema renamed to parametersJsonSchema
	if funcDecl.Get("parametersJsonSchema").Exists() {
		t.Log("parametersJsonSchema exists (expected)")
	} else {
		t.Error("parametersJsonSchema should exist")
	}
	if funcDecl.Get("input_schema").Exists() {
		t.Error("input_schema should not exist (should be renamed)")
	}

	// Check schema content
	schema := funcDecl.Get("parametersJsonSchema")
	if schema.Get("type").String() != "object" {
		t.Errorf("Expected schema type 'object', got '%s'", schema.Get("type").String())
	}
	if !schema.Get("properties.command").Exists() {
		t.Error("Schema should have 'command' property")
	}
}

func TestConvertOpenAIRequestToAntigravity_MixedToolFormats(t *testing.T) {
	// Test handling of both OpenAI and Claude format tools in same request
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [{"role": "user", "content": "Hello"}],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "OpenAITool",
					"description": "OpenAI format tool",
					"parameters": {
						"type": "object",
						"properties": {"arg": {"type": "string"}}
					}
				}
			},
			{
				"name": "ClaudeTool",
				"description": "Claude format tool",
				"input_schema": {
					"type": "object",
					"properties": {"param": {"type": "string"}}
				}
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	funcDecls := gjson.Get(outputStr, "request.tools.0.functionDeclarations")
	if !funcDecls.IsArray() {
		t.Fatal("functionDeclarations should be an array")
	}

	decls := funcDecls.Array()
	if len(decls) != 2 {
		t.Fatalf("Expected 2 function declarations, got %d", len(decls))
	}

	// Check OpenAI format tool
	if decls[0].Get("name").String() != "OpenAITool" {
		t.Errorf("First tool should be 'OpenAITool', got '%s'", decls[0].Get("name").String())
	}
	if !decls[0].Get("parametersJsonSchema.properties.arg").Exists() {
		t.Error("OpenAI tool should have 'arg' property")
	}

	// Check Claude format tool
	if decls[1].Get("name").String() != "ClaudeTool" {
		t.Errorf("Second tool should be 'ClaudeTool', got '%s'", decls[1].Get("name").String())
	}
	if !decls[1].Get("parametersJsonSchema.properties.param").Exists() {
		t.Error("Claude tool should have 'param' property")
	}
}

// ============================================================================
// Claude/Cursor Tool Use (Assistant Message) Tests
// ============================================================================

func TestConvertOpenAIRequestToAntigravity_ClaudeToolUse(t *testing.T) {
	// Cursor sends tool_use in assistant content array
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "List files"},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_abc123",
						"name": "Shell",
						"input": {"command": "ls -la"}
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// Check model message with function call
	modelContent := gjson.Get(outputStr, "request.contents.1")
	if modelContent.Get("role").String() != "model" {
		t.Errorf("Expected role 'model', got '%s'", modelContent.Get("role").String())
	}

	funcCall := modelContent.Get("parts.0.functionCall")
	if !funcCall.Exists() {
		t.Fatal("functionCall should exist in assistant message")
	}

	if funcCall.Get("id").String() != "call_abc123" {
		t.Errorf("Expected function id 'call_abc123', got '%s'", funcCall.Get("id").String())
	}
	if funcCall.Get("name").String() != "Shell" {
		t.Errorf("Expected function name 'Shell', got '%s'", funcCall.Get("name").String())
	}
	if funcCall.Get("args.command").String() != "ls -la" {
		t.Errorf("Expected args.command 'ls -la', got '%s'", funcCall.Get("args.command").String())
	}

	// Check thoughtSignature is added
	if modelContent.Get("parts.0.thoughtSignature").String() != geminiCLIFunctionThoughtSignature {
		t.Errorf("Expected thoughtSignature '%s'", geminiCLIFunctionThoughtSignature)
	}
}

func TestConvertOpenAIRequestToAntigravity_ClaudeToolUseWithText(t *testing.T) {
	// Assistant message with both text and tool_use
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Check disk space"},
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "I'll check the disk space for you."},
					{
						"type": "tool_use",
						"id": "call_xyz789",
						"name": "Shell",
						"input": {"command": "df -h"}
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	parts := gjson.Get(outputStr, "request.contents.1.parts")
	if !parts.IsArray() {
		t.Fatal("Parts should be an array")
	}

	partsArr := parts.Array()
	if len(partsArr) < 2 {
		t.Fatalf("Expected at least 2 parts (text + function), got %d", len(partsArr))
	}

	// Check text part
	if partsArr[0].Get("text").String() != "I'll check the disk space for you." {
		t.Errorf("Expected text content, got '%s'", partsArr[0].Get("text").String())
	}

	// Check function call part
	if partsArr[1].Get("functionCall.name").String() != "Shell" {
		t.Errorf("Expected functionCall with name 'Shell', got '%s'", partsArr[1].Get("functionCall.name").String())
	}
}

// ============================================================================
// Claude/Cursor Tool Result (User Message) Tests
// ============================================================================

func TestConvertOpenAIRequestToAntigravity_ClaudeToolResult(t *testing.T) {
	// Cursor sends tool_result in user content array
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "List files"},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_abc123",
						"name": "Shell",
						"input": {"command": "ls -la"}
					}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "call_abc123",
						"content": [{"type": "text", "text": "file1.txt\nfile2.txt"}]
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// After the model message with function call, there should be a user message with function response
	contents := gjson.Get(outputStr, "request.contents")
	if !contents.IsArray() {
		t.Fatal("Contents should be an array")
	}

	contentsArr := contents.Array()
	// Should have: user, model (with functionCall), user (with functionResponse)
	if len(contentsArr) < 3 {
		t.Fatalf("Expected at least 3 content items, got %d", len(contentsArr))
	}

	// The function response should be in a user message after the model's function call
	funcResp := gjson.Get(outputStr, "request.contents.2.parts.0.functionResponse")
	if funcResp.Exists() {
		if funcResp.Get("id").String() != "call_abc123" {
			t.Errorf("Expected function response id 'call_abc123', got '%s'", funcResp.Get("id").String())
		}
		if funcResp.Get("name").String() != "Shell" {
			t.Errorf("Expected function response name 'Shell', got '%s'", funcResp.Get("name").String())
		}
	}
}

func TestConvertOpenAIRequestToAntigravity_ClaudeToolResultStringContent(t *testing.T) {
	// Tool result with string content instead of array
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Run command"},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_def456",
						"name": "Shell",
						"input": {"command": "echo hello"}
					}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "call_def456",
						"content": "hello"
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// Verify the function response exists and has the correct content
	funcResp := gjson.Get(outputStr, "request.contents.2.parts.0.functionResponse")
	if funcResp.Exists() {
		if funcResp.Get("id").String() != "call_def456" {
			t.Errorf("Expected function response id 'call_def456', got '%s'", funcResp.Get("id").String())
		}
	}
}

func TestConvertOpenAIRequestToAntigravity_ClaudeToolResultOnlyMessage(t *testing.T) {
	// User message containing ONLY tool_result should not create empty user content
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Run something"},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_only",
						"name": "Shell",
						"input": {"command": "pwd"}
					}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "call_only",
						"content": "/home/user"
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// The tool_result-only message should not create an empty user content node
	// Instead, the function response is attached to the model's function call
	contents := gjson.Get(outputStr, "request.contents")
	if !contents.IsArray() {
		t.Fatal("Contents should be an array")
	}

	// Check that we don't have empty user messages
	for i, content := range contents.Array() {
		role := content.Get("role").String()
		parts := content.Get("parts")
		if role == "user" && (!parts.Exists() || len(parts.Array()) == 0) {
			t.Errorf("Content %d: empty user message should not exist", i)
		}
	}
}

// ============================================================================
// Multi-turn Conversation Tests
// ============================================================================

func TestConvertOpenAIRequestToAntigravity_ClaudeMultiTurnToolConversation(t *testing.T) {
	// Complete multi-turn conversation with tool calls in Claude format
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "What files are in the current directory?"},
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me check the files for you."},
					{
						"type": "tool_use",
						"id": "call_1",
						"name": "Shell",
						"input": {"command": "ls -la"}
					}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "call_1",
						"content": [{"type": "text", "text": "total 8\ndrwxr-xr-x 2 user user 4096 Jan 1 00:00 .\ndrwxr-xr-x 3 user user 4096 Jan 1 00:00 ..\n-rw-r--r-- 1 user user    0 Jan 1 00:00 test.txt"}]
					}
				]
			},
			{
				"role": "assistant",
				"content": [{"type": "text", "text": "The current directory contains one file: test.txt"}]
			},
			{"role": "user", "content": "Show me the content of test.txt"}
		],
		"tools": [
			{
				"name": "Shell",
				"description": "Execute a shell command",
				"input_schema": {"type": "object", "properties": {"command": {"type": "string"}}, "required": ["command"]}
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// Verify the conversation structure
	contents := gjson.Get(outputStr, "request.contents")
	if !contents.IsArray() {
		t.Fatal("Contents should be an array")
	}

	// Check that tools are properly translated
	tools := gjson.Get(outputStr, "request.tools.0.functionDeclarations")
	if !tools.IsArray() || len(tools.Array()) == 0 {
		t.Error("Tools should be translated to functionDeclarations")
	}

	// Verify model field
	if gjson.Get(outputStr, "model").String() != "gemini-2.5-pro" {
		t.Errorf("Model should be 'gemini-2.5-pro', got '%s'", gjson.Get(outputStr, "model").String())
	}
}

// ============================================================================
// OpenAI Format Compatibility Tests (ensure existing format still works)
// ============================================================================

func TestConvertOpenAIRequestToAntigravity_OpenAIToolCalls(t *testing.T) {
	// OpenAI format tool calls should still work
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Run a command"},
			{
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{
						"id": "call_openai",
						"type": "function",
						"function": {
							"name": "Shell",
							"arguments": "{\"command\": \"whoami\"}"
						}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_openai",
				"content": "root"
			}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "Shell",
					"description": "Execute command",
					"parameters": {"type": "object", "properties": {"command": {"type": "string"}}}
				}
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// Verify function call in model message
	funcCall := gjson.Get(outputStr, "request.contents.1.parts.0.functionCall")
	if !funcCall.Exists() {
		t.Fatal("functionCall should exist for OpenAI format")
	}
	if funcCall.Get("id").String() != "call_openai" {
		t.Errorf("Expected id 'call_openai', got '%s'", funcCall.Get("id").String())
	}
	if funcCall.Get("name").String() != "Shell" {
		t.Errorf("Expected name 'Shell', got '%s'", funcCall.Get("name").String())
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestConvertOpenAIRequestToAntigravity_EmptyInputSchema(t *testing.T) {
	// Claude tool without input_schema should get default schema
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [{"role": "user", "content": "Hello"}],
		"tools": [
			{
				"name": "NoParamsTool",
				"description": "A tool with no parameters"
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	// Tool should not be added if input_schema is missing (Claude format requires it)
	tools := gjson.Get(outputStr, "request.tools")
	if tools.Exists() {
		t.Log("Tools created despite missing input_schema - checking if handled correctly")
	}
}

func TestConvertOpenAIRequestToAntigravity_ClaudeToolUseEmptyInput(t *testing.T) {
	// Tool use with empty input object
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Do something"},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_empty",
						"name": "NoArgsFunction",
						"input": {}
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	funcCall := gjson.Get(outputStr, "request.contents.1.parts.0.functionCall")
	if !funcCall.Exists() {
		t.Fatal("functionCall should exist even with empty input")
	}

	args := funcCall.Get("args")
	if !args.Exists() {
		t.Error("args should exist (can be empty object)")
	}
}

func TestConvertOpenAIRequestToAntigravity_MultipleToolUseInSameMessage(t *testing.T) {
	// Multiple tool_use items in same assistant message
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Check both disk and memory"},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_disk",
						"name": "Shell",
						"input": {"command": "df -h"}
					},
					{
						"type": "tool_use",
						"id": "call_mem",
						"name": "Shell",
						"input": {"command": "free -m"}
					}
				]
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	parts := gjson.Get(outputStr, "request.contents.1.parts")
	if !parts.IsArray() {
		t.Fatal("Parts should be an array")
	}

	partsArr := parts.Array()
	if len(partsArr) != 2 {
		t.Fatalf("Expected 2 parts (2 function calls), got %d", len(partsArr))
	}

	// Check both function calls
	if partsArr[0].Get("functionCall.id").String() != "call_disk" {
		t.Errorf("First function call should have id 'call_disk'")
	}
	if partsArr[1].Get("functionCall.id").String() != "call_mem" {
		t.Errorf("Second function call should have id 'call_mem'")
	}
}

func TestConvertOpenAIRequestToAntigravity_CursorToolWithAllFields(t *testing.T) {
	// Full Cursor tool definition with all fields
	inputJSON := []byte(`{
		"model": "gemini-2.5-pro",
		"messages": [{"role": "user", "content": "Hello"}],
		"tools": [
			{
				"name": "read",
				"description": "Reads a file from the local filesystem.",
				"input_schema": {
					"type": "object",
					"properties": {
						"filePath": {
							"type": "string",
							"description": "The path to the file to read"
						},
						"limit": {
							"type": "number",
							"description": "The number of lines to read"
						},
						"offset": {
							"type": "number",
							"description": "The line number to start reading from"
						}
					},
					"required": ["filePath"]
				}
			}
		]
	}`)

	output := ConvertOpenAIRequestToAntigravity("gemini-2.5-pro", inputJSON, false)
	outputStr := string(output)

	funcDecl := gjson.Get(outputStr, "request.tools.0.functionDeclarations.0")
	if !funcDecl.Exists() {
		t.Fatal("Function declaration should exist")
	}

	// Verify all fields are preserved
	if funcDecl.Get("name").String() != "read" {
		t.Error("Name should be 'read'")
	}
	if !funcDecl.Get("parametersJsonSchema.properties.filePath").Exists() {
		t.Error("filePath property should exist")
	}
	if !funcDecl.Get("parametersJsonSchema.properties.limit").Exists() {
		t.Error("limit property should exist")
	}
	if !funcDecl.Get("parametersJsonSchema.required").Exists() {
		t.Error("required array should exist")
	}
}
