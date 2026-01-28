package billing

import (
	"context"
	"time"
)

type UsageService struct{}

func NewUsageService() *UsageService {
	return &UsageService{}
}

func (s *UsageService) RecordUsage(ctx context.Context, record *UsageRecord) error {
	// Implement actual storage logic here
	return nil
}

func (s *UsageService) GetUsage(ctx context.Context, tenantID string, start, end time.Time) (map[string]int64, error) {
	// Implement actual retrieval logic here
	return map[string]int64{}, nil
}
