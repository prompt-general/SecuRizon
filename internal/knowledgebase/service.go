package knowledgebase

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/securizon/internal/support"
)

type KnowledgeBaseService struct {
	vectorStore  VectorStore
	openaiClient *openai.Client
	articleStore ArticleStore
	config       KBConfig
}

func NewKnowledgeBaseService(vectorStore VectorStore, articleStore ArticleStore, config KBConfig) *KnowledgeBaseService {
	return &KnowledgeBaseService{
		vectorStore:  vectorStore,
		articleStore: articleStore,
		openaiClient: openai.NewClient(config.OpenAIAPIKey),
		config:       config,
	}
}

// Search searches the knowledge base using semantic search
func (kbs *KnowledgeBaseService) Search(ctx context.Context, query string, filters map[string]interface{}) ([]SearchResult, error) {
	// Generate embedding for query
	embedding, err := kbs.generateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %v", err)
	}

	// Perform vector search
	vectorResults, err := kbs.vectorStore.Search(ctx, embedding, kbs.config.MaxResults, kbs.config.SimilarityThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %v", err)
	}

	// Get full articles
	results := make([]SearchResult, 0, len(vectorResults))
	for _, vr := range vectorResults {
		article, err := kbs.articleStore.GetArticle(ctx, vr.ArticleID)
		if err != nil {
			log.Printf("Failed to get article %s: %v", vr.ArticleID, err)
			continue
		}

		// Check filters
		if !kbs.matchesFilters(article, filters) {
			continue
		}

		// Find relevant snippets
		snippets := kbs.extractSnippets(article.Content, query)

		// Determine relevance level
		relevance := kbs.determineRelevance(vr.Score, len(snippets))

		results = append(results, SearchResult{
			Article:   article,
			Score:     vr.Score,
			Relevance: relevance,
			Snippets:  snippets,
		})
	}

	// Sort by relevance score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// SmartSuggest suggests articles based on context (e.g., during ticket creation)
func (kbs *KnowledgeBaseService) SmartSuggest(ctx context.Context, context string, ticketType string, category string) ([]SearchResult, error) {
	// Build enhanced query based on context
	enhancedQuery := kbs.enhanceQuery(ctx, context, ticketType, category)

	// Search with enhanced query
	return kbs.Search(ctx, enhancedQuery, map[string]interface{}{
		"category":  category,
		"published": true,
	})
}

// TrainFromTickets trains the knowledge base from resolved tickets
func (kbs *KnowledgeBaseService) TrainFromTickets(ctx context.Context, tickets []support.Ticket) error {
	for _, ticket := range tickets {
		// Skip if ticket wasn't resolved with a solution
		if ticket.Status != support.StatusResolved || ticket.Metadata["solution"] == nil {
			continue
		}

		// Create article from ticket
		article := &Article{
			ID:        uuid.New().String(),
			Title:     fmt.Sprintf("How to resolve: %s", ticket.Subject),
			Content:   ticket.Metadata["solution"].(string),
			Category:  ticket.Category,
			Tags:      append(ticket.Tags, "from-ticket", fmt.Sprintf("ticket-%s", ticket.ID)),
			Published: true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Generate embedding
		embedding, err := kbs.generateEmbedding(ctx,
			fmt.Sprintf("%s\n\n%s", article.Title, article.Content))
		if err != nil {
			log.Printf("Failed to generate embedding for ticket %s: %v", ticket.ID, err)
			continue
		}

		article.Embedding = embedding

		// Store article
		if err := kbs.articleStore.CreateArticle(ctx, article); err != nil {
			log.Printf("Failed to create article from ticket %s: %v", ticket.ID, err)
			continue
		}

		// Store embedding
		if err := kbs.vectorStore.StoreEmbedding(ctx, article.ID, embedding); err != nil {
			log.Printf("Failed to store embedding for article %s: %v", article.ID, err)
		}
	}

	return nil
}

// GenerateAnswer uses AI to generate answers based on knowledge base
func (kbs *KnowledgeBaseService) GenerateAnswer(ctx context.Context, question string, context string) (*GeneratedAnswer, error) {
	// Search for relevant articles
	results, err := kbs.Search(ctx, question, map[string]interface{}{
		"published": true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge base: %v", err)
	}

	// Build context from top articles
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Context from knowledge base:\n\n")

	for i, result := range results {
		if i >= 3 { // Use top 3 articles
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("Article: %s\n", result.Article.Title))
		contextBuilder.WriteString(fmt.Sprintf("Content: %s\n\n", result.Article.Content))
	}

	// Add user context if provided
	if context != "" {
		contextBuilder.WriteString("Additional context:\n")
		contextBuilder.WriteString(context)
		contextBuilder.WriteString("\n\n")
	}

	// Generate answer using OpenAI
	prompt := fmt.Sprintf(`You are a SecuRizon support assistant. Use the following context to answer the question.

%s

Question: %s

Provide a helpful, accurate answer based on the context. If you're not sure, say so and suggest contacting support.`,
		contextBuilder.String(), question)

	resp, err := kbs.openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful SecuRizon support assistant.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   500,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %v", err)
	}

	answer := &GeneratedAnswer{
		Answer:     resp.Choices[0].Message.Content,
		Sources:    make([]Article, 0, len(results)),
		Confidence: kbs.calculateConfidence(results),
	}

	// Add source articles
	for _, result := range results {
		if result.Score > kbs.config.SimilarityThreshold {
			answer.Sources = append(answer.Sources, *result.Article)
		}
	}

	return answer, nil
}

// Track article helpfulness
func (kbs *KnowledgeBaseService) TrackHelpfulness(ctx context.Context, articleID string, helpful bool) error {
	article, err := kbs.articleStore.GetArticle(ctx, articleID)
	if err != nil {
		return err
	}

	if helpful {
		article.Helpful++
	} else {
		article.NotHelpful++
	}

	// If article is frequently marked as not helpful, flag for review
	totalFeedback := article.Helpful + article.NotHelpful
	if totalFeedback > 10 && float64(article.NotHelpful)/float64(totalFeedback) > 0.5 {
		if article.Metadata == nil {
			article.Metadata = make(map[string]interface{})
		}
		article.Metadata["needs_review"] = true
		article.Metadata["review_reason"] = "Low helpfulness score"
	}

	return kbs.articleStore.UpdateArticle(ctx, article)
}

// Helper methods

func (kbs *KnowledgeBaseService) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := kbs.openaiClient.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}

func (kbs *KnowledgeBaseService) matchesFilters(article *Article, filters map[string]interface{}) bool {
	for k, v := range filters {
		switch k {
		case "category":
			if article.Category != v.(string) {
				return false
			}
		case "published":
			if article.Published != v.(bool) {
				return false
			}
		}
	}
	return true
}

func (kbs *KnowledgeBaseService) extractSnippets(content, query string) []string {
	// Simple snippet extraction logic
	var snippets []string
	words := strings.Fields(query)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		for _, word := range words {
			if strings.Contains(strings.ToLower(line), strings.ToLower(word)) {
				snippets = append(snippets, strings.TrimSpace(line))
				break
			}
		}
		if len(snippets) >= 3 {
			break
		}
	}
	return snippets
}

func (kbs *KnowledgeBaseService) determineRelevance(score float64, snippetCount int) string {
	if score > 0.9 {
		return "exact"
	}
	if score > 0.7 {
		return "high"
	}
	if score > 0.5 {
		return "medium"
	}
	return "low"
}

func (kbs *KnowledgeBaseService) enhanceQuery(ctx context.Context, context, ticketType, category string) string {
	return fmt.Sprintf("%s %s %s", context, ticketType, category)
}

func (kbs *KnowledgeBaseService) calculateConfidence(results []SearchResult) float64 {
	if len(results) == 0 {
		return 0
	}
	return results[0].Score
}
