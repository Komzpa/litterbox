package bundles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Embedder converts mail content into a semantic vector.
type Embedder interface {
	Embed(context.Context, string) ([]float64, error)
}

// OllamaEmbedder calls the local Ollama embeddings API.
type OllamaEmbedder struct {
	Host, Model string
	Client      *http.Client
}

// NewOllamaEmbedder reads OLLAMA_HOST and LITTERBOX_EMBED_MODEL. Empty config
// uses localhost and nomic-embed-text.
func NewOllamaEmbedder() OllamaEmbedder {
	host := strings.TrimRight(os.Getenv("OLLAMA_HOST"), "/")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("LITTERBOX_EMBED_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}
	return OllamaEmbedder{Host: host, Model: model, Client: &http.Client{Timeout: 15 * time.Second}}
}

func (e OllamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}{e.Model, text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(e.Host, "/")+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embeddings: %s", resp.Status)
	}
	var result struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Embeddings) == 0 {
		return nil, errors.New("ollama returned no embeddings")
	}
	return result.Embeddings[0], nil
}

type mailCard struct {
	id                    uuid.UUID
	sender, subject, text string
	vector                []float64
	bundleKey             string
}
type cluster struct {
	key      string
	centroid []float64
	count    int
	cards    []*mailCard
}

// Cluster immediately assigns open mail cards to semantic bundles across accounts.
// Any embedding failure falls back to the established sender-key assignment.
func Cluster(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, embedder Embedder) error {
	if embedder == nil {
		return errors.New("nil bundle embedder")
	}
	rows, err := tx.Query(ctx, `SELECT c.id,c.sender,c.subject,COALESCE(m.text,'') FROM cards c LEFT JOIN LATERAL (SELECT text FROM messages WHERE tenant_id=c.tenant_id AND card_id=c.id ORDER BY received_at DESC LIMIT 1) m ON true WHERE c.tenant_id=$1 AND c.source='mail' AND c.state='open' ORDER BY c.created_at,c.id`, tenant)
	if err != nil {
		return err
	}
	var cards []*mailCard
	for rows.Next() {
		c := new(mailCard)
		if err := rows.Scan(&c.id, &c.sender, &c.subject, &c.text); err != nil {
			rows.Close()
			return err
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, c := range cards {
		v, err := embedder.Embed(ctx, strings.TrimSpace(c.subject+"\n"+c.text))
		if err != nil {
			return fallback(ctx, tx, tenant, cards)
		}
		c.vector = v
	}
	var groups []*cluster
	for _, c := range cards {
		best, score := -1, 0.82
		for i, g := range groups {
			if s := cosine(c.vector, g.centroid); s > score {
				best, score = i, s
			}
		}
		if best < 0 {
			groups = append(groups, &cluster{key: "semantic:" + c.id.String(), centroid: append([]float64(nil), c.vector...), count: 1, cards: []*mailCard{c}})
			continue
		}
		g := groups[best]
		g.count++
		for i := range g.centroid {
			g.centroid[i] += (c.vector[i] - g.centroid[i]) / float64(g.count)
		}
		g.cards = append(g.cards, c)
	}
	for _, g := range groups {
		if len(g.cards) < 2 {
			for _, c := range g.cards {
				if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2`, tenant, c.id); err != nil {
					return err
				}
			}
			continue
		}
		var bundleID uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO bundles (tenant_id,id,title,centroid,bundle_key) VALUES ($1,gen_random_uuid(),$2,'{}',$3) ON CONFLICT (tenant_id,bundle_key) WHERE bundle_key IS NOT NULL DO UPDATE SET title=EXCLUDED.title RETURNING id`, tenant, g.cards[0].subject, g.key).Scan(&bundleID); err != nil {
			return err
		}
		for _, c := range g.cards {
			var excluded bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bundle_exclusions WHERE tenant_id=$1 AND sender_key=$2 AND bundle_key=$3)`, tenant, SenderKey(c.sender), g.key).Scan(&excluded); err != nil {
				return err
			}
			if excluded {
				if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2`, tenant, c.id); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, tenant, c.id, bundleID); err != nil {
				return err
			}
		}
	}
	return nil
}

// AfterIngest is the hook mail producers call after inserting or updating a card.
func AfterIngest(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, embedder Embedder) error {
	if embedder == nil {
		embedder = NewOllamaEmbedder()
	}
	return Cluster(ctx, tx, tenant, embedder)
}

func fallback(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cards []*mailCard) error {
	for _, c := range cards {
		if err := Assign(ctx, tx, tenant, c.id, c.sender, "", "", "normal"); err != nil {
			return err
		}
	}
	return nil
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, aa, bb float64
	for i := range a {
		dot += a[i] * b[i]
		aa += a[i] * a[i]
		bb += b[i] * b[i]
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / math.Sqrt(aa*bb)
}
