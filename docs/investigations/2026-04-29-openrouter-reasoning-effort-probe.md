# OpenRouter Reasoning Effort Serialization Probe

**Date:** 2026-04-29
**Issue:** #552
**Question:** Does OpenRouter accept the flat OpenAI-style `reasoning_effort: "high"` form, or does it require the nested `reasoning: { effort: "high" }` form for DeepSeek V4 reasoning?

## Conclusion

**No code change required.** OpenRouter accepts **both** the flat `reasoning_effort` form and the nested `reasoning: { effort }` form. Both are fully honored — they produce identical reasoning token counts and the same reasoning output. A code change would only be needed if the flat form were silently dropped or rejected.

---

## Evidence

### 1. OpenRouter OpenAPI Spec (Authoritative Docs)

Source: `https://openrouter.ai/openapi.json` — the machine-readable spec for OpenRouter's API.

The `ChatRequest` schema defines the reasoning parameter as a **nested object**:

```json
"reasoning": {
  "description": "Configuration options for reasoning models",
  "properties": {
    "effort": {
      "enum": ["xhigh", "high", "medium", "low", "minimal", "none", null],
      "type": "string"
    },
    "summary": { "$ref": "#/components/schemas/ChatReasoningSummaryVerbosityEnum" }
  },
  "type": "object"
}
```

There is **no `reasoning_effort` flat string field** in the `ChatRequest` schema.

**Interpretation:** The nested `reasoning: { effort }` form is the canonical/documented form. The flat `reasoning_effort` field is NOT documented in OpenRouter's spec.

### 2. Live Probe Results

With `OPENROUTER_API_KEY` set, three requests were sent to `deepseek/deepseek-v4-flash` (cheapest SKU, ~$0.00001 per probe call):

| Form | Payload | reasoning_tokens | reasoning_text populated |
|------|---------|-----------------|--------------------------|
| Flat (undocumented) | `"reasoning_effort": "high"` | 22 | Yes |
| Nested (canonical) | `"reasoning": {"effort": "high"}` | 22 | Yes |
| Flat with `"none"` | `"reasoning_effort": "none"` | 0 | No |
| Nested with `"none"` | `"reasoning": {"effort": "none"}` | 0 | No |
| Control (no param) | (omitted) | 22 | Yes |

**Findings:**
- Both `reasoning_effort: "high"` (flat) and `reasoning: {effort: "high"}` (nested) produce identical results (22 reasoning tokens, reasoning text populated).
- Both `reasoning_effort: "none"` (flat) and `reasoning: {effort: "none"}` (nested) successfully **disable** reasoning (0 reasoning tokens, no reasoning text). This confirms the flat form is **not silently dropped** — it is actively processed.
- Control (no param): DeepSeek V4 Flash reasons by default (22 reasoning tokens even without any effort parameter).

**The flat form is therefore not dropped but fully honored by OpenRouter despite not being listed in their OpenAPI spec.**

### 3. Interpretation

OpenRouter appears to maintain backwards compatibility with the OpenAI-style flat `reasoning_effort` field (which OpenAI uses for o-series models). This is consistent with OpenRouter's positioning as an OpenAI-compatible proxy — they accept fields from both the OpenAI API shape and their own extended shape.

---

## Decision: No Code Change

The current `client.go` serialization at line 312 (`reasoning_effort` flat string) works correctly with OpenRouter for DeepSeek V4 models. A per-provider serialization branch is **not needed**.

```go
// internal/provider/openai/client.go line 312 — current code, no change needed
ReasoningEffort   string `json:"reasoning_effort,omitempty"`
```

---

## Probe Cost

Total cost: ~$0.000045 USD (5 probe calls × ~$0.000009 each).

---

## Sources

- OpenRouter OpenAPI spec: `https://openrouter.ai/openapi.json` — `ChatRequest.reasoning` schema
- OpenRouter reasoning docs: `https://openrouter.ai/docs/use-cases/reasoning-tokens`
- Prior investigation: `docs/investigations/2026-04-28-openrouter-deepseek-v4.md` §5 item 5
- DeepSeek V4 Flash on OpenRouter: `https://openrouter.ai/deepseek/deepseek-v4-flash`
