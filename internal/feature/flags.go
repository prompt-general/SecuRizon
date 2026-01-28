package feature

import (
    "context"
    "fmt"
    "hash/fnv"
    "sync"
    "time"

    "github.com/patrickmn/go-cache"
)

type FeatureBackend interface {
    GetFlag(ctx context.Context, name string) (FeatureFlag, error)
}

type FeatureNotifier interface {
    Notify(flag FeatureFlag)
}

type UserContext struct {
    ID          string
    Email       string
    Groups      []string
    Environment string
}

type FeatureFlagManager struct {
    flags    map[string]FeatureFlag
    mu       sync.RWMutex
    cache    *cache.Cache
    backend  FeatureBackend
    notifier FeatureNotifier
}

type FeatureFlag struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Enabled     bool                   `json:"enabled"`
    Type        string                 `json:"type"` // boolean, percentage, user, environment
    Percentage  int                    `json:"percentage,omitempty"`
    Users       []string               `json:"users,omitempty"`
    Groups      []string               `json:"groups,omitempty"`
    StartTime   *time.Time             `json:"start_time,omitempty"`
    EndTime     *time.Time             `json:"end_time,omitempty"`
    Metadata    map[string]interface{} `json:"metadata"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
}

func NewFeatureFlagManager(backend FeatureBackend) *FeatureFlagManager {
    return &FeatureFlagManager{
        flags:   make(map[string]FeatureFlag),
        cache:   cache.New(5*time.Minute, 10*time.Minute),
        backend: backend,
    }
}

func (ffm *FeatureFlagManager) IsEnabled(ctx context.Context, flagName string, userContext UserContext) (bool, error) {
    // Check cache first
    if enabled, found := ffm.cache.Get(flagName); found {
        return enabled.(bool), nil
    }

    // Get flag from backend
    flag, err := ffm.backend.GetFlag(ctx, flagName)
    if err != nil {
        return false, fmt.Errorf("failed to get feature flag: %v", err)
    }

    // Evaluate flag
    enabled := ffm.evaluateFlag(flag, userContext)

    // Cache result
    ffm.cache.Set(flagName, enabled, cache.DefaultExpiration)

    return enabled, nil
}

func (ffm *FeatureFlagManager) evaluateFlag(flag FeatureFlag, userContext UserContext) bool {
    // Check if flag is within time window
    if flag.StartTime != nil && time.Now().Before(*flag.StartTime) {
        return false
    }
    if flag.EndTime != nil && time.Now().After(*flag.EndTime) {
        return false
    }

    // Check flag type
    switch flag.Type {
    case "boolean":
        return flag.Enabled

    case "percentage":
        if !flag.Enabled {
            return false
        }
        // Use user ID for consistent rollout
        hash := hashString(userContext.ID) % 100
        return int(hash) < flag.Percentage

    case "user":
        if !flag.Enabled {
            return false
        }
        // Check specific users
        for _, user := range flag.Users {
            if user == userContext.Email || user == userContext.ID {
                return true
            }
        }
        // Check groups
        for _, group := range flag.Groups {
            for _, userGroup := range userContext.Groups {
                if group == userGroup {
                    return true
                }
            }
        }
        return false

    case "environment":
        env, ok := flag.Metadata["environment"].(string)
        return ok && flag.Enabled && userContext.Environment == env

    default:
        return false
    }
}

func hashString(s string) uint32 {
    h := fnv.New32a()
    h.Write([]byte(s))
    return h.Sum32()
}

// Feature flags for controlled rollout
var DefaultFeatureFlags = map[string]FeatureFlag{
    "attack-path-engine": {
        Name:        "attack-path-engine",
        Description: "Enable new attack path analysis engine",
        Enabled:     true,
        Type:        "percentage",
        Percentage:  10, // Roll out to 10% of requests
        Metadata: map[string]interface{}{
            "version": "v2",
            "owner":   "security-team",
        },
    },
    "remediation-auto-approval": {
        Name:        "remediation-auto-approval",
        Description: "Auto-approve low-risk remediations",
        Enabled:     false,
        Type:        "boolean",
        Metadata: map[string]interface{}{
            "risk_threshold": 30,
            "categories":     []string{"low", "medium"},
        },
    },
    "real-time-dashboard": {
        Name:        "real-time-dashboard",
        Description: "New real-time dashboard with WebSocket updates",
        Enabled:     true,
        Type:        "user",
        Users:       []string{"admin@company.com", "security-lead@company.com"},
        Groups:      []string{"beta-testers"},
        Metadata: map[string]interface{}{
            "ui_version": "v3",
        },
    },
    "ml-anomaly-detection": {
        Name:        "ml-anomaly-detection",
        Description: "Machine learning anomaly detection for configuration changes",
        Enabled:     true,
        Type:        "environment",
        Metadata: map[string]interface{}{
            "environment": "staging",
            "model":       "v1.2",
        },
    },
}
