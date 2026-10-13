package bundles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	ModelName() string
	Embed(context.Context, string) ([]float64, error)
}

// ClusterDB starts top-level transactions. A pgx.Tx deliberately does not
// satisfy this contract: its Begin would keep the caller's transaction open.
type ClusterDB interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
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
	return OllamaEmbedder{Host: host, Model: model, Client: &http.Client{}}
}

func (e OllamaEmbedder) ModelName() string { return e.Model }

func (e OllamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	// Bound each call even when a caller supplies a client without a timeout.
	// Only this child context expires, so Cluster can still commit its fallback.
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
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
	cacheHash             []byte
	bundleKey             string
}
type cluster struct {
	key      string
	centroid []float64
	count    int
	cards    []*mailCard
}

func loadMailCards(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, openOnly bool) ([]*mailCard, error) {
	rows, err := tx.Query(ctx, `SELECT c.id,c.sender,c.subject,COALESCE(m.text,'') FROM cards c LEFT JOIN LATERAL (SELECT text FROM messages WHERE tenant_id=c.tenant_id AND card_id=c.id ORDER BY received_at DESC LIMIT 1) m ON true WHERE c.tenant_id=$1 AND c.source='mail' AND (NOT $2::boolean OR c.state='open') ORDER BY c.created_at,c.id`, tenant, openOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cards []*mailCard
	for rows.Next() {
		c := new(mailCard)
		if err := rows.Scan(&c.id, &c.sender, &c.subject, &c.text); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func loadEmbeddingCache(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, model string, cards []*mailCard) error {
	byID := make(map[uuid.UUID]*mailCard, len(cards))
	for _, c := range cards {
		byID[c.id] = c
	}
	rows, err := tx.Query(ctx, `SELECT e.card_id,e.input_hash,e.vector FROM card_embeddings e JOIN cards c ON c.tenant_id=e.tenant_id AND c.id=e.card_id WHERE e.tenant_id=$1 AND e.model=$2 AND c.source='mail' AND c.state='open'`, tenant, model)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var hash []byte
		var vector []float64
		if err := rows.Scan(&id, &hash, &vector); err != nil {
			return err
		}
		if c := byID[id]; c != nil {
			c.cacheHash, c.vector = hash, vector
		}
	}
	return rows.Err()
}

// Each successful miss is durable before the next slow embed call. No cache
// transaction remains open while waiting for the embedder, even on fallback.
func storeEmbedding(ctx context.Context, db ClusterDB, tenant uuid.UUID, model string, card uuid.UUID, hash []byte, vector []float64) error {
	tx, err := beginClusterTx(ctx, db, tenant)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `INSERT INTO card_embeddings(tenant_id,card_id,model,input_hash,vector) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,card_id,model) DO UPDATE SET input_hash=EXCLUDED.input_hash,vector=EXCLUDED.vector`, tenant, card, model, hash, vector); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReassignGitHub repairs legacy sender bundles in every card state, using the
// same deterministic router and exclusions as ingestion. No embeddings or
// Gmail access are needed; unchanged memberships produce no card changes.
func ReassignGitHub(ctx context.Context, tx pgx.Tx, tenant uuid.UUID) error {
	cards, err := loadMailCards(ctx, tx, tenant, false)
	if err != nil {
		return err
	}
	return assignGitHub(ctx, tx, tenant, cards)
}

// Cluster immediately assigns open mail cards to bundles across accounts.
// GitHub mail is routed deterministically by repo and who must act (see
// RouteGitHubMail) without embeddings. Remaining mail is clustered by
// embeddings; provider boilerplate is stripped before embedding. Any
// embedding failure falls back to structured keys: GitHub mail keeps its
// routed keys and is never keyed by sender alone. The embed error is logged
// once per run so a missing local model is visible instead of silently
// reshaping the inbox. Read and write transactions are tenant-scoped and short;
// all embedding calls run between them with no database transaction open.
func Cluster(ctx context.Context, db ClusterDB, tenant uuid.UUID, embedder Embedder) error {
	if embedder == nil {
		return errors.New("nil bundle embedder")
	}
	tx, err := beginClusterTx(ctx, db, tenant)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	cards, err := loadMailCards(ctx, tx, tenant, true)
	if err != nil {
		return err
	}
	model := embedder.ModelName()
	if err := loadEmbeddingCache(ctx, tx, tenant, model, cards); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	var rest []*mailCard
	for _, c := range cards {
		if _, routed := RouteGitHubMail(c.sender, c.subject, c.text); routed {
			continue
		}
		rest = append(rest, c)
	}
	embeddingFailed := false
	for _, c := range rest {
		text := StripGitHubFooter(c.text)
		// LinkedIn network digests put the topic in the subject; their plain
		// body contains only account statistics and recipient/provider boilerplate.
		// Embedding that shared body makes unrelated notifications look alike.
		if SenderKey(c.sender) == "messages-noreply@linkedin.com" && strings.Contains(text, "You are receiving LinkedIn network update digest email.") {
			text = ""
		}
		input := strings.TrimSpace(c.subject + "\n" + text)
		hash := sha256.Sum256([]byte(input))
		if bytes.Equal(c.cacheHash, hash[:]) {
			continue
		}
		v, err := embedder.Embed(ctx, input)
		if err != nil {
			log.Printf("bundles: embedding failed, using structured fallback: %v", err)
			embeddingFailed = true
			break
		}
		c.vector = v
		if err := storeEmbedding(ctx, db, tenant, model, c.id, hash[:], v); err != nil {
			return err
		}
	}
	tx, err = beginClusterTx(ctx, db, tenant)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := assignGitHub(ctx, tx, tenant, cards); err != nil {
		return err
	}
	if embeddingFailed {
		if err := fallback(ctx, tx, tenant, cards); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	var groups []*cluster
	for _, c := range rest {
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
				if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2 AND bundle_id IS NOT NULL`, tenant, c.id); err != nil {
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
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bundle_exclusions WHERE tenant_id=$1 AND sender_key=$2 AND bundle_key=$3)`, tenant, BundleSenderKey(c.sender), g.key).Scan(&excluded); err != nil {
				return err
			}
			if excluded {
				if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2 AND bundle_id IS NOT NULL`, tenant, c.id); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2 AND bundle_id IS DISTINCT FROM $3`, tenant, c.id, bundleID); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func beginClusterTx(ctx context.Context, db ClusterDB, tenant uuid.UUID) (pgx.Tx, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant.String()); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

// AfterIngest is called after the producer commits its card and message writes.
func AfterIngest(ctx context.Context, db ClusterDB, tenant uuid.UUID, embedder Embedder) error {
	if embedder == nil {
		embedder = NewOllamaEmbedder()
	}
	return Cluster(ctx, db, tenant, embedder)
}

// assignGitHub routes GitHub mail deterministically in the write phase.
// Standalone cards (mentions, human replies on his own threads) get no
// bundle; bundled cards are grouped by their structured key with a human
// title. Routed cards carry c.bundleKey so the semantic path skips them.
// Take-out exclusions are honored per (sender, key).
func assignGitHub(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cards []*mailCard) error {
	groups := map[string][]*mailCard{}
	var order []string
	for _, c := range cards {
		route, ok := RouteGitHubMail(c.sender, c.subject, c.text)
		if !ok {
			continue
		}
		if route.Standalone {
			if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2 AND bundle_id IS NOT NULL`, tenant, c.id); err != nil {
				return err
			}
			c.bundleKey = "standalone"
			continue
		}
		c.bundleKey = route.Key
		if _, dup := groups[route.Key]; !dup {
			order = append(order, route.Key)
		}
		groups[route.Key] = append(groups[route.Key], c)
	}
	titles := map[string]string{}
	for _, c := range cards {
		if c.bundleKey == "" || c.bundleKey == "standalone" {
			continue
		}
		if _, done := titles[c.bundleKey]; done {
			continue
		}
		if route, ok := RouteGitHubMail(c.sender, c.subject, c.text); ok && !route.Standalone {
			titles[c.bundleKey] = route.Title
		}
	}
	for _, key := range order {
		title := titles[key]
		if title == "" {
			title = key
		}
		var bundleID uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO bundles (tenant_id,id,title,centroid,bundle_key) VALUES ($1,gen_random_uuid(),$2,'{}',$3) ON CONFLICT (tenant_id,bundle_key) WHERE bundle_key IS NOT NULL DO UPDATE SET title=EXCLUDED.title RETURNING id`, tenant, title, key).Scan(&bundleID); err != nil {
			return err
		}
		for _, c := range groups[key] {
			var excluded bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bundle_exclusions WHERE tenant_id=$1 AND sender_key=$2 AND bundle_key=$3)`, tenant, BundleSenderKey(c.sender), key).Scan(&excluded); err != nil {
				return err
			}
			if excluded {
				if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2 AND bundle_id IS NOT NULL`, tenant, c.id); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2 AND bundle_id IS DISTINCT FROM $3`, tenant, c.id, bundleID); err != nil {
				return err
			}
		}
	}
	return nil
}

func fallback(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cards []*mailCard) error {
	for _, c := range cards {
		if c.bundleKey != "" {
			continue
		}
		// Without a topic vector or a structured route there is no evidence
		// of a shared topic. Never replace that evidence with a sender key.
		if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2 AND bundle_id IS NOT NULL`, tenant, c.id); err != nil {
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
