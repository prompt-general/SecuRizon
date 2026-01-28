package tenant

import (
	"context"
)

// Store defines the interface for tenant storage operations
type Store interface {
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	ListActiveTenants(ctx context.Context) ([]*Tenant, error)
	UpdateTenant(ctx context.Context, tenant *Tenant) error
}
