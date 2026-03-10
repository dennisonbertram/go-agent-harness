---
name: ollama-ops
description: "Manage local LLM inference with Ollama: pull/run/list/serve models, Modelfile creation, API usage, model management. Trigger: when using Ollama, running local LLMs, ollama pull, ollama run, ollama serve, Modelfile, local model inference"
version: 1
argument-hint: "[pull|run|list|serve|create] [model-name]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Ollama Operations

You are now operating in Ollama local LLM management mode.

## Installation and Server

```bash
# macOS (via Homebrew)
brew install ollama

# Start the Ollama server (runs on port 11434 by default)
ollama serve

# Check if Ollama server is running
curl -s http://localhost:11434/api/tags | jq '.models[].name'

# Verify the server is healthy
curl -s http://localhost:11434/ | jq .
```

## Model Management

```bash
# List all locally available models
ollama list

# Pull a model from the Ollama registry
ollama pull llama3.2
ollama pull qwen2.5:7b
ollama pull mistral:latest
ollama pull nomic-embed-text    # embedding model

# Show model details (parameters, context length, etc.)
ollama show llama3.2

# Remove a model
ollama rm llama3.2

# Copy a model (create an alias)
ollama cp llama3.2 my-llama
```

## Running Models

```bash
# Interactive chat session
ollama run llama3.2

# One-shot prompt (non-interactive)
ollama run llama3.2 "Explain what a mutex is in one sentence."

# Pipe input to model
echo "Summarize: $(cat README.md)" | ollama run llama3.2

# Run with system prompt
ollama run llama3.2 "What is the capital of France?" --system "You are a geography expert."
```

## REST API Usage

Ollama exposes an OpenAI-compatible API on `http://localhost:11434`.

```bash
# Generate a completion (Ollama native API)
curl -s http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.2",
    "prompt": "Why is the sky blue?",
    "stream": false
  }' | jq -r '.response'

# Chat completion (OpenAI-compatible API)
curl -s http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.2",
    "messages": [
      {"role": "user", "content": "Hello, what can you do?"}
    ]
  }' | jq -r '.choices[0].message.content'

# Generate embeddings
curl -s http://localhost:11434/api/embed \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "input": "The quick brown fox jumps over the lazy dog"
  }' | jq '.embeddings[0] | length'
```

## Modelfile: Custom Models

```bash
# Create a Modelfile to customize a base model
cat > Modelfile <<'EOF'
FROM llama3.2

# Set the system prompt
SYSTEM "You are a helpful Go programming assistant. Always provide working code examples."

# Adjust parameters
PARAMETER temperature 0.3
PARAMETER top_p 0.9
PARAMETER num_ctx 4096
EOF

# Build the custom model
ollama create go-assistant -f Modelfile

# Run the custom model
ollama run go-assistant "How do I implement a context-aware HTTP handler in Go?"
```

## Modelfile Parameters

```dockerfile
# Modelfile reference
FROM llama3.2:latest        # base model

SYSTEM "Your system prompt here"

# Model parameters
PARAMETER temperature 0.7   # 0.0 (deterministic) to 1.0 (creative)
PARAMETER top_k 40          # top-k sampling
PARAMETER top_p 0.9         # nucleus sampling
PARAMETER num_ctx 8192      # context window size (tokens)
PARAMETER repeat_penalty 1.1
PARAMETER seed 42           # for reproducible outputs (0 = random)

# Add files to the model
MESSAGE user "What can you help me with?"
MESSAGE assistant "I can help you with Go programming, code reviews, and debugging."
```

## Using Ollama from Go

```go
// Using the OpenAI-compatible API from Go
import "github.com/openai/openai-go"

client := openai.NewClient(
    option.WithBaseURL("http://localhost:11434/v1"),
    option.WithAPIKey("ollama"), // any non-empty string
)

chat, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Model: openai.F("llama3.2"),
    Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Explain Go interfaces."),
    }),
})
```

## Performance and Configuration

```bash
# Set GPU layers (for NVIDIA/AMD GPUs)
OLLAMA_GPU_LAYERS=35 ollama serve

# Set number of parallel requests
OLLAMA_NUM_PARALLEL=4 ollama serve

# Set max loaded models (default: 1)
OLLAMA_MAX_LOADED_MODELS=2 ollama serve

# Check GPU usage during inference
nvidia-smi dmon -s u -d 1

# Check memory usage
ollama ps    # shows loaded models and VRAM usage
```

## Common Models Reference

| Model | Size | Best For |
|-------|------|---------|
| `llama3.2:3b` | 2 GB | Fast responses, simple tasks |
| `llama3.2:latest` | 4.7 GB | General purpose, good quality |
| `qwen2.5:7b` | 4.7 GB | Code, reasoning |
| `mistral:latest` | 4.1 GB | Instruction following |
| `nomic-embed-text` | 274 MB | Text embeddings |
| `mxbai-embed-large` | 669 MB | High-quality embeddings |

## Troubleshooting

```bash
# Check if server is running
curl -s http://localhost:11434/ || echo "Ollama server not running; run: ollama serve"

# Check loaded models and memory
ollama ps

# Restart the Ollama service
pkill ollama && ollama serve

# Model download failed? Resume by re-running pull
ollama pull llama3.2

# Clear model cache (if disk full)
ollama rm $(ollama list --json | jq -r '.[].name' | head -1)
```
