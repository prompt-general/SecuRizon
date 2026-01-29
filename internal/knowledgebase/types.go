package knowledgebase

import (
	"time"
)

type Article struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Category    string                 `json:"category"`
	Subcategory string                 `json:"subcategory,omitempty"`
	Tags        []string               `json:"tags"`
	Views       int64                  `json:"views"`
	Helpful     int64                  `json:"helpful"`
	NotHelpful  int64                  `json:"not_helpful"`
	Published   bool                   `json:"published"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Embedding   []float32              `json:"-"`
}

type SearchResult struct {
	Article   *Article `json:"article"`
	Score     float64  `json:"score"`
	Relevance string   `json:"relevance"` // exact, high, medium, low
	Snippets  []string `json:"snippets"`
}

type GeneratedAnswer struct {
	Answer     string    `json:"answer"`
	Sources    []Article `json:"sources"`
	Confidence float64   `json:"confidence"`
}

type KBConfig struct {
	OpenAIAPIKey        string  `yaml:"openai_api_key"`
	EmbeddingModel      string  `yaml:"embedding_model"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	MaxResults          int     `yaml:"max_results"`
}
