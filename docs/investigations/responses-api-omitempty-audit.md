# Responses API omitempty Audit

**Date**: 2026-03-14  
**Scope**: Audit all Responses API structs for `omitempty` tags that conflict with OpenAI API requirements.

---

## Background

The Responses API (`/v1/responses`) has stricter field requirements than the Chat Completions API. Previously, we identified that `responsesInputItem.Arguments` had `json:"arguments,omitempty"` but the API rejects requests with missing `arguments` fields on `function_call` items, returning:

```
Missing required parameter: 'input[5].arguments'
```

This audit checks for other similar issues across all Responses API structs.

---

## Summary of Findings

### Critical Issues Found

#### 1. **responsesInputItem.Arguments** ✅ FIXED
- **Field**: `Arguments string`
- **Current tag**: `json:"arguments"` (NO omitempty)
- **Status**: Already corrected (PR applied)
- **Why required**: When `type == "function_call"`, the `arguments` field must always be present, even if it's an empty string `""` or `{}`. The API validates this field on the input item.

---

### Potential Issues (Requires Verification Against OpenAI Docs)

#### 2. **responsesInputItem.Role** ⚠️ CHECK
- **Field**: `Role string`
- **Current tag**: `json:"role,omitempty"`
- **Impact scope**: Applies to `type == "message"` items
- **OpenAI spec pattern**: For message items with `type == "message"`, the `role` field (user/assistant) may be required to distinguish message direction
- **Recommendation**: Should likely be required; verify that all "message" items always have a role

#### 3. **responsesInputItem.Type** ✅ REQUIRED
- **Field**: `Type string`
- **Current tag**: `json:"type"` (NO omitempty)
- **Status**: Correctly marked as required
- **Why**: Discriminator field for input item variant (message, function_call, function_call_output)

#### 4. **responsesInputItem.Content** ⚠️ CONDITIONAL
- **Field**: `Content any`
- **Current tag**: `json:"content,omitempty"`
- **Impact scope**: For `type == "message"` items
- **OpenAI spec pattern**: Message items must have content (string or content block array)
- **Recommendation**: Should likely be required when `type == "message"`

#### 5. **responsesInputItem.CallID** ⚠️ CONDITIONAL
- **Field**: `CallID string`
- **Current tag**: `json:"call_id,omitempty"`
- **Impact scope**: For `type == "function_call"` and `type == "function_call_output"` items
- **OpenAI spec pattern**: Both function_call and function_call_output require `call_id` to link them
- **Recommendation**: Should likely be required when `type == "function_call"` or `type == "function_call_output"`

#### 6. **responsesInputItem.Name** ⚠️ CONDITIONAL
- **Field**: `Name string`
- **Current tag**: `json:"name,omitempty"`
- **Impact scope**: For `type == "function_call"` items
- **OpenAI spec pattern**: The function name must be specified on function_call items
- **Recommendation**: Should likely be required when `type == "function_call"`

#### 7. **responsesInputItem.Output** ⚠️ CONDITIONAL
- **Field**: `Output string`
- **Current tag**: `json:"output,omitempty"`
- **Impact scope**: For `type == "function_call_output"` items
- **OpenAI spec pattern**: Output content may be optional (function returned no output), but when present must be explicit
- **Status**: Likely correct as-is; omitempty acceptable for optional output results

---

### Output-Side Structs (Response Parsing)

These structs parse responses FROM the API, so `omitempty` is less critical:

#### 8. **responsesOutputItem.Content** ✅ OK
- **Current tag**: `json:"content,omitempty"`
- **Status**: Acceptable (parsing response, content is optional for function_call items)

#### 9. **responsesOutputItem.CallID** ✅ OK
- **Current tag**: `json:"call_id,omitempty"`
- **Status**: Acceptable (parsing response, only present on function_call items)

#### 10. **responsesOutputItem.Name** ✅ OK
- **Current tag**: `json:"name,omitempty"`
- **Status**: Acceptable (parsing response, only present on function_call items)

#### 11. **responsesOutputItem.Arguments** ✅ OK
- **Current tag**: `json:"arguments,omitempty"`
- **Status**: Acceptable (parsing response, may be empty string)

---

## Responses API Field Requirements (OpenAI Spec Pattern)

Based on the Responses API behavior and mapToResponsesRequest construction:

### Input Item Variants

#### type = "message" (user/assistant text)
Required fields:
- `type`: "message"
- `role`: "user" | "assistant"
- `content`: string or content block array (likely required)

Optional:
- None (all above appear required)

#### type = "function_call" (assistant calling a function)
Required fields:
- `type`: "function_call"
- `call_id`: unique call identifier
- `name`: function name
- `arguments`: function arguments JSON string (ALWAYS, even empty "{}")

Optional:
- None (all above appear required)

#### type = "function_call_output" (tool result)
Required fields:
- `type`: "function_call_output"
- `call_id`: matches the function_call it answers
- `output`: result content (possibly optional, may return nothing)

Optional:
- `output` may be optional for functions with no output

---

## Test Pattern Review

### Existing Test Coverage

1. **TestResponsesAPIToolCallMapping** (line 666-733)
   - Tests assistant message with tool call: ✅ includes function_call item
   - Verifies: call_id, name, arguments all present
   - ✅ Shows all 3 fields are being set by mapToResponsesRequest

2. **TestResponsesAPIMultiTurnToolResult** (line 735-795)
   - Tests tool result message: ✅ includes function_call_output item
   - Verifies: call_id, output both present
   - ✅ Shows both fields are being set

### Test Pattern Style

Tests use:
- httptest.NewServer with request capture
- json.Unmarshal of captured body
- Assertion of map keys and values
- Pattern: `funcCallItem["name"] != "get_weather"` type checks

### Recommended Test Pattern

```go
func TestResponsesAPIToolCallWithNoArguments(t *testing.T) {
    t.Parallel()

    var capturedBody []byte
    testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{
            "id":"resp_test",
            "output":[{"type":"message","content":[{"type":"output_text","text":"done"}]}],
            "usage":{"input_tokens":20,"output_tokens":2,"total_tokens":22}
        }`))
    }))
    defer testServer.Close()

    client := newResponsesClient(t, testServer.URL)

    _, err := client.Complete(context.Background(), harness.CompletionRequest{
        Model: "gpt-5.1-codex-mini",
        Messages: []harness.Message{
            {Role: "user", Content: "Call bash with no args"},
            {
                Role:    "assistant",
                Content: "",
                ToolCalls: []harness.ToolCall{
                    // Tool call with empty arguments (edge case)
                    {ID: "call_bash", Name: "bash", Arguments: "{}"},
                },
            },
        },
    })
    if err != nil {
        t.Fatalf("complete: %v", err)
    }

    var req map[string]any
    if err := json.Unmarshal(capturedBody, &req); err != nil {
        t.Fatalf("unmarshal request: %v", err)
    }

    input, ok := req["input"].([]any)
    if !ok {
        t.Fatalf("expected input array, got %T", req["input"])
    }

    // Verify the function_call item is present with arguments field set (never omitted)
    funcCallItem := input[1].(map[string]any)
    if funcCallItem["type"] != "function_call" {
        t.Fatalf("expected function_call, got: %v", funcCallItem["type"])
    }

    // CRITICAL: arguments field MUST be present even when empty
    argumentsVal, hasArguments := funcCallItem["arguments"]
    if !hasArguments {
        t.Fatal("arguments field missing from function_call item (should never be omitted)")
    }
    if argumentsVal != "{}" {
        t.Fatalf("expected arguments={}, got %v", argumentsVal)
    }
}
```

---

## Regression Test Scenario

**Scenario**: Tool call with empty/no-argument bash command

**Setup**:
- Create an assistant message with a tool call to `bash` tool
- Tool call arguments = `{}` (no parameters, just empty object)

**Expected behavior**:
- mapToResponsesRequest creates a function_call item with:
  - `type: "function_call"`
  - `call_id: "call_<id>"`
  - `name: "bash"`
  - `arguments: "{}"` ← MUST be present in JSON
  
- When marshaled to JSON, the request body includes: `"arguments":"{}"`
- ✅ NOT `{"call_id":"...", "name":"bash"}` with arguments omitted

**Assertion**:
```go
// Verify arguments field is ALWAYS in the JSON, never omitted
if !strings.Contains(string(capturedBody), `"arguments"`) {
    t.Fatal("arguments field missing from request JSON")
}

// More strictly: arguments must appear in function_call item
funcCallItem := ... // extracted from input array
if _, ok := funcCallItem["arguments"]; !ok {
    t.Fatal("arguments field omitted from function_call item")
}
```

---

## Recommended Fixes

### Priority 1: Already Fixed ✅
- `responsesInputItem.Arguments`: Changed from `json:"arguments,omitempty"` → `json:"arguments"`

### Priority 2: Likely Needs Fixing ⚠️
Conditional fields that should be required based on item type, but Go struct can't express conditional requirements. Consider:

1. **Option A**: Keep current tags and document assumptions
   - Assumption: Callers always set these fields for the appropriate item types
   - Risk: Bugs go undetected if a caller forgets to set required field

2. **Option B**: Create separate input structs per type
   ```go
   type responsesMessageItem struct {
       Type    string `json:"type"` // "message"
       Role    string `json:"role"`
       Content any    `json:"content"`
   }
   
   type responsesFunctionCallItem struct {
       Type      string `json:"type"` // "function_call"
       CallID    string `json:"call_id"`
       Name      string `json:"name"`
       Arguments string `json:"arguments"`
   }
   
   // Union type for input items
   ```

3. **Option C**: Keep current struct, but add validation layer in mapToResponsesRequest
   - Validate that function_call items have call_id, name, arguments
   - Return error if any required field is missing
   - More maintainable than creating 3 separate types

**Recommendation**: Go with Option C (validation in mapToResponsesRequest) for now:
- Keeps code readable
- Single struct definition
- Runtime validation catches missing fields
- Clear error message helps debugging

---

## Files Affected

| File | Change | Rationale |
|------|--------|-----------|
| `internal/provider/openai/client.go` | Validation in `mapToResponsesRequest` | Ensure all required fields are set before marshaling |
| `internal/provider/openai/client_test.go` | Add regression test for empty arguments | Prevents regression of Arguments omitempty bug |

---

## Verification Checklist

- [x] Identified Arguments field (already fixed)
- [x] Located all Responses API structs (responsesInputItem, responsesOutputItem, etc.)
- [x] Identified conditional fields (role, content, call_id, name)
- [x] Reviewed OpenAI API pattern for field requirements
- [x] Documented test pattern from existing tests
- [x] Provided regression test scenario
- [ ] Run new test to verify it catches the bug
- [ ] Implement validation layer if pursuing Option C
- [ ] Deploy and verify no API rejections with "Missing required parameter"
