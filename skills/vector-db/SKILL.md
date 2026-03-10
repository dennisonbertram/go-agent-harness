---
name: vector-db
description: "Work with vector databases for semantic search: Chroma, Qdrant, Weaviate. Embeddings, similarity search, collections, upsert, query patterns. Trigger: when using vector databases, semantic search, embeddings, Chroma, Qdrant, Weaviate, similarity search, RAG, vector search"
version: 1
argument-hint: "[chroma|qdrant|weaviate] [collection-name]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Vector Database Operations

You are now operating in vector database management mode.

## Chroma

Chroma is an open-source embedding database. It can run locally (in-memory or persistent) or as a server.

### Start Chroma Server

```bash
# Install Chroma CLI
pip install chromadb

# Start a persistent server
chroma run --path ./chroma-data

# Or with Docker
docker run -d \
  --name chroma \
  -p 8000:8000 \
  -v $(pwd)/chroma-data:/chroma/chroma \
  chromadb/chroma:latest
```

### Chroma Python Usage

```python
import chromadb
from chromadb.utils import embedding_functions

# Connect to local server
client = chromadb.HttpClient(host="localhost", port=8000)

# Use Ollama embeddings (local)
ef = embedding_functions.OllamaEmbeddingFunction(
    url="http://localhost:11434/api/embeddings",
    model_name="nomic-embed-text",
)

# Create or get a collection
collection = client.get_or_create_collection(
    name="documents",
    embedding_function=ef,
    metadata={"hnsw:space": "cosine"},
)

# Add documents
collection.upsert(
    ids=["doc1", "doc2", "doc3"],
    documents=[
        "Go is a statically typed compiled language.",
        "Python is a dynamically typed interpreted language.",
        "Rust focuses on memory safety and performance.",
    ],
    metadatas=[
        {"source": "docs", "language": "go"},
        {"source": "docs", "language": "python"},
        {"source": "docs", "language": "rust"},
    ],
)

# Query by semantic similarity
results = collection.query(
    query_texts=["compiled languages with type safety"],
    n_results=2,
)
print(results["documents"])
print(results["distances"])

# Query with metadata filter
results = collection.query(
    query_texts=["memory management"],
    n_results=1,
    where={"language": {"$in": ["go", "rust"]}},
)
```

## Qdrant

Qdrant is a high-performance vector database written in Rust with a REST and gRPC API.

### Start Qdrant Server

```bash
# Docker (persistent)
docker run -d \
  --name qdrant \
  -p 6333:6333 \
  -p 6334:6334 \
  -v $(pwd)/qdrant-data:/qdrant/storage \
  qdrant/qdrant:latest

# Verify it's running
curl -s http://localhost:6333/collections | jq .
```

### Qdrant REST API

```bash
# Create a collection
curl -s -X PUT http://localhost:6333/collections/documents \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": {
      "size": 768,
      "distance": "Cosine"
    }
  }' | jq .

# Upsert points (vectors with payloads)
curl -s -X PUT http://localhost:6333/collections/documents/points \
  -H "Content-Type: application/json" \
  -d '{
    "points": [
      {
        "id": 1,
        "vector": [0.1, 0.2, 0.3],
        "payload": {"text": "Go language", "lang": "go"}
      }
    ]
  }' | jq .

# Search (semantic similarity)
curl -s -X POST http://localhost:6333/collections/documents/points/search \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.1, 0.2, 0.3],
    "limit": 5,
    "with_payload": true
  }' | jq .

# Search with filter
curl -s -X POST http://localhost:6333/collections/documents/points/search \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.1, 0.2, 0.3],
    "limit": 5,
    "filter": {
      "must": [{"key": "lang", "match": {"value": "go"}}]
    },
    "with_payload": true
  }' | jq .

# List collections
curl -s http://localhost:6333/collections | jq .

# Collection info
curl -s http://localhost:6333/collections/documents | jq .

# Delete a collection
curl -s -X DELETE http://localhost:6333/collections/documents | jq .
```

### Qdrant Go Client

```go
import (
    "github.com/qdrant/go-client/qdrant"
)

client, err := qdrant.NewClient(&qdrant.Config{
    Host: "localhost",
    Port: 6334, // gRPC port
})

// Create collection
client.CreateCollection(ctx, &qdrant.CreateCollection{
    CollectionName: "documents",
    VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
        Size:     768,
        Distance: qdrant.Distance_Cosine,
    }),
})

// Upsert points
client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: "documents",
    Points: []*qdrant.PointStruct{
        {
            Id:      qdrant.NewIDNum(1),
            Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
            Payload: qdrant.NewValueMap(map[string]any{"text": "Go language"}),
        },
    },
})

// Search
results, err := client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: "documents",
    Query:          qdrant.NewQuery(0.1, 0.2, 0.3),
    Limit:          qdrant.PtrOf(uint64(5)),
    WithPayload:    qdrant.NewWithPayload(true),
})
```

## Generating Embeddings (Ollama)

```bash
# Generate embedding via Ollama REST API
curl -s http://localhost:11434/api/embed \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "input": "Go is a compiled language designed for simplicity and performance."
  }' | jq '.embeddings[0] | length'   # should be 768 for nomic-embed-text
```

```python
# Python: get embedding from Ollama
import requests

def embed(text: str, model: str = "nomic-embed-text") -> list[float]:
    resp = requests.post(
        "http://localhost:11434/api/embed",
        json={"model": model, "input": text},
    )
    return resp.json()["embeddings"][0]
```

## RAG Pattern (Retrieval-Augmented Generation)

```python
# Full RAG pattern: embed query, search Chroma, generate with Ollama
import chromadb, requests, json

client = chromadb.HttpClient(host="localhost", port=8000)
collection = client.get_collection("documents")

def embed(text):
    r = requests.post("http://localhost:11434/api/embed",
                      json={"model": "nomic-embed-text", "input": text})
    return r.json()["embeddings"][0]

def rag_query(question: str, n_context: int = 3) -> str:
    # 1. Embed the question
    q_vec = embed(question)

    # 2. Find similar documents
    results = collection.query(query_embeddings=[q_vec], n_results=n_context)
    context = "\n\n".join(results["documents"][0])

    # 3. Generate answer using the retrieved context
    prompt = f"Context:\n{context}\n\nQuestion: {question}\nAnswer:"
    r = requests.post("http://localhost:11434/api/generate",
                      json={"model": "llama3.2", "prompt": prompt, "stream": False})
    return r.json()["response"]

print(rag_query("What compiled languages prioritize type safety?"))
```

## Similarity Metrics

| Metric | Use Case | Formula |
|--------|----------|---------|
| Cosine | Text similarity, NLP (most common) | cos(a, b) |
| Dot Product | Retrieval when vectors are normalized | a · b |
| Euclidean | Image similarity, dense features | ||a - b|| |

## Best Practices

- Normalize embeddings before storing (many models already do this).
- Use the same model for indexing and querying — embedding spaces are model-specific.
- Use metadata filtering to narrow search scope before vector similarity.
- Persist Chroma/Qdrant data with mounted volumes in Docker.
- For production, use Qdrant with replication or Weaviate clusters.
- Store embedding model name in collection metadata to catch model mismatches.
