package bundles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIEmbeddingsTransport(t *testing.T) {
	var gotPath, gotModel string
	var gotInput []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		gotModel, gotInput = req.Model, req.Input
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"embedding": []float64{0.25, -0.5}}}})
	}))
	defer server.Close()

	t.Setenv("LITTERBOX_EMBED_URL", server.URL+"/v1/")
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	t.Setenv("LITTERBOX_EMBED_MODEL", "all-minilm:latest")
	e := NewOllamaEmbedder()
	if !e.OpenAI || e.ModelName() != "all-minilm:latest" {
		t.Fatalf("embedder=%+v, want OpenAI transport keeping the cache model name", e)
	}
	v, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/embeddings" || gotModel != "all-minilm:latest" || len(gotInput) != 1 || gotInput[0] != "hello" {
		t.Fatalf("request path=%q model=%q input=%v", gotPath, gotModel, gotInput)
	}
	if len(v) != 2 || v[0] != 0.25 || v[1] != -0.5 {
		t.Fatalf("vector=%v", v)
	}
}

func TestOllamaTransportStaysDefault(t *testing.T) {
	t.Setenv("LITTERBOX_EMBED_URL", "")
	t.Setenv("OLLAMA_HOST", "http://ollama.test:11434/")
	e := NewOllamaEmbedder()
	if e.OpenAI || e.Host != "http://ollama.test:11434" {
		t.Fatalf("embedder=%+v, want Ollama transport", e)
	}
}

func TestOpenAIEmbeddingsAgainstLlamaSwap(t *testing.T) {
	url := "http://172.17.0.1:11450/v1"
	if testing.Short() {
		t.Skip("live llama-swap probe")
	}
	e := OllamaEmbedder{Host: url, Model: "all-minilm:latest", OpenAI: true}
	v, err := e.Embed(context.Background(), "PostGIS raster overflow")
	if err != nil {
		t.Skipf("llama-swap not reachable: %v", err)
	}
	if len(v) != 384 {
		t.Fatalf("dims=%d, want 384 (all-minilm)", len(v))
	}
}

func TestOpenAIEmbeddingsTruncatesOverWindowInput(t *testing.T) {
	var sentTokens []int
	var tokenizePath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upstream/all-minilm:latest/tokenize":
			tokenizePath = r.URL.Path
			toks := []int{101}
			for i := 0; i < 600; i++ {
				toks = append(toks, 1000+i)
			}
			json.NewEncoder(w).Encode(map[string]any{"tokens": append(toks, 102)})
		case "/v1/embeddings":
			var req struct {
				Input []json.RawMessage `json:"input"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if len(req.Input) == 1 && json.Unmarshal(req.Input[0], &sentTokens) == nil {
				json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"embedding": []float64{1, 2}}}})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"code":500,"message":"input (602 tokens) is too large to process. increase the physical batch size (current batch size: 512)"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	e := OllamaEmbedder{Host: server.URL + "/v1", Model: "all-minilm:latest", OpenAI: true}
	v, err := e.Embed(context.Background(), "long mail")
	if err != nil {
		t.Fatal(err)
	}
	if tokenizePath == "" || len(v) != 2 {
		t.Fatalf("tokenize=%q vector=%v", tokenizePath, v)
	}
	if len(sentTokens) != 512 || sentTokens[0] != 101 || sentTokens[1] != 1000 || sentTokens[510] != 1509 || sentTokens[511] != 102 {
		t.Fatalf("sent %d tokens: head=%v tail=%v", len(sentTokens), sentTokens[:2], sentTokens[len(sentTokens)-2:])
	}
}

func TestOpenAIEmbeddingsLongInputAgainstLlamaSwap(t *testing.T) {
	if testing.Short() {
		t.Skip("live llama-swap probe")
	}
	e := OllamaEmbedder{Host: "http://172.17.0.1:11450/v1", Model: "all-minilm:latest", OpenAI: true}
	long := "PostGIS security advisory raster overflow"
	for i := 0; i < 400; i++ {
		long += " lorem ipsum dolor"
	}
	v, err := e.Embed(context.Background(), long)
	if err != nil {
		if strings.Contains(err.Error(), "connect") {
			t.Skipf("llama-swap not reachable: %v", err)
		}
		t.Fatal(err)
	}
	if len(v) != 384 {
		t.Fatalf("dims=%d", len(v))
	}
}
