package knowledgebase

import (
	"context"
)

type VectorStore interface {
	Search(ctx context.Context, embedding []float32, limit int, threshold float64) ([]VectorResult, error)
	StoreEmbedding(ctx context.Context, articleID string, embedding []float32) error
}

type VectorResult struct {
	ArticleID string
	Score     float64
}

type ArticleStore interface {
	GetArticle(ctx context.Context, id string) (*Article, error)
	CreateArticle(ctx context.Context, article *Article) error
	UpdateArticle(ctx context.Context, article *Article) error
	ListArticles(ctx context.Context, filters map[string]interface{}) ([]*Article, error)
}
