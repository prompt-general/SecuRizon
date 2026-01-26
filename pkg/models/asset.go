package models

import (
	"time"
	"github.com/google/uuid"
)

// AssetType represents the type of asset
type AssetType string

const (
	AssetTypeIdentity   AssetType = "identity"
	AssetTypeCompute   AssetType = "compute"
	AssetTypeNetwork   AssetType = "network"
	AssetTypeData      AssetType = "data"
	AssetTypeSaaS      AssetType = "saas"
	AssetTypeFinding   AssetType = "finding"
)

// Provider represents the cloud provider
type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderAzure Provider = "azure"
	ProviderGCP   Provider = "gcp"
	ProviderGitHub Provider = "github"
	ProviderJira  Provider = "jira"
)

// Environment represents the deployment environment
type Environment string

const (
	EnvironmentProduction  Environment = "prod"
	EnvironmentStaging     Environment = "staging"
	EnvironmentDevelopment Environment = "dev"
	EnvironmentTesting     Environment = "test"
)

// PrivilegeLevel represents the privilege level for identities
type PrivilegeLevel string

const (
	PrivilegeLevelLow    PrivilegeLevel = "low"
	PrivilegeLevelMedium PrivilegeLevel = "medium"
	PrivilegeLevelHigh   PrivilegeLevel = "high"
	PrivilegeLevelAdmin  PrivilegeLevel = "admin"
)

// DataSensitivity represents data sensitivity levels
type DataSensitivity string

const (
	DataSensitivityPublic       DataSensitivity = "public"
	DataSensitivityInternal     DataSensitivity = "internal"
	DataSensitivityConfidential DataSensitivity = "confidential"
	DataSensitivityRestricted   DataSensitivity = "restricted"
)

// BaseAsset represents the base structure for all assets
type BaseAsset struct {
	ID           string     `json:"id" bson:"_id"`
	Provider     Provider   `json:"provider"`
	Type         AssetType  `json:"type"`
	Environment  Environment `json:"environment"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	FirstSeen    time.Time  `json:"first_seen"`
	LastSeen     time.Time  `json:"last_seen"`
	Tags         map[string]string `json:"tags,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Identity represents an identity asset (user, role, service account)
type Identity struct {
	BaseAsset
	Type           string         `json:"type"` // User, Role, ServiceAccount
	PrivilegeLevel PrivilegeLevel `json:"privilege_level"`
	IsHuman        bool           `json:"is_human"`
	Email          string         `json:"email,omitempty"`
	Username       string         `json:"username,omitempty"`
}

// Compute represents compute resources (VM, Container, Function)
type Compute struct {
	BaseAsset
	SubType         string    `json:"sub_type"` // VM, Container, Function
	OS              string    `json:"os"`
	ExposedPorts    []int     `json:"exposed_ports"`
	InternetExposed bool      `json:"internet_exposed"`
	PublicIP        string    `json:"public_ip,omitempty"`
	PrivateIP       string    `json:"private_ip,omitempty"`
	InstanceType    string    `json:"instance_type,omitempty"`
	Region          string    `json:"region,omitempty"`
}

// Network represents network resources (VPC, Subnet, Security Group)
type Network struct {
	BaseAsset
	SubType      string                 `json:"sub_type"` // VPC, Subnet, SecurityGroup, FirewallRule
	IngressRules []NetworkRule          `json:"ingress_rules,omitempty"`
	EgressRules  []NetworkRule          `json:"egress_rules,omitempty"`
	CIDRBlocks   []string               `json:"cidr_blocks,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// NetworkRule represents a network rule
type NetworkRule struct {
	Protocol string   `json:"protocol"` // tcp, udp, icmp, all
	FromPort int      `json:"from_port"`
	ToPort   int      `json:"to_port"`
	CIDRs    []string `json:"cidrs"`
	Action   string   `json:"action"` // allow, deny
}

// Data represents data resources (storage, database, file share)
type Data struct {
	BaseAsset
	SubType         DataSensitivity `json:"sub_type"` // Storage, Database, FileShare
	DataSensitivity DataSensitivity `json:"data_sensitivity"`
	ExternalSharing bool            `json:"external_sharing"`
	AdminAccess     bool            `json:"admin_access"`
	Encryption      bool            `json:"encryption"`
	SizeGB          int64           `json:"size_gb,omitempty"`
	Region          string          `json:"region,omitempty"`
}

// SaaS represents SaaS platform resources
type SaaS struct {
	BaseAsset
	Platform       string `json:"platform"` // GitHub, Jira, Salesforce, etc.
	ResourceType   string `json:"resource_type"` // Repo, Project, Workspace, etc.
	ExternalSharing bool  `json:"external_sharing"`
	AdminAccess    bool   `json:"admin_access"`
	Public         bool   `json:"public"`
	URL            string `json:"url,omitempty"`
}

// Finding represents security findings
type Finding struct {
	BaseAsset
	PolicyID      string    `json:"policy_id"`
	Severity      float64   `json:"severity"` // 0-10
	RiskScore     float64   `json:"risk_score"` // 0-100
	Status        string    `json:"status"` // open, resolved, suppressed
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	Description   string    `json:"description"`
	Recommendation string   `json:"recommendation"`
	AssetID       string    `json:"asset_id"`
	FalsePositive bool      `json:"false_positive"`
	Suppressed    bool      `json:"suppressed"`
	SuppressedReason string `json:"suppressed_reason,omitempty"`
}

// NewBaseAsset creates a new base asset
func NewBaseAsset(provider Provider, assetType AssetType, environment Environment, name string) BaseAsset {
	now := time.Now()
	return BaseAsset{
		ID:          uuid.New().String(),
		Provider:    provider,
		Type:        assetType,
		Environment: environment,
		Name:        name,
		CreatedAt:   now,
		UpdatedAt:   now,
		FirstSeen:   now,
		LastSeen:    now,
		Tags:        make(map[string]string),
		Metadata:    make(map[string]interface{}),
	}
}

// Asset interface for all asset types
type Asset interface {
	GetID() string
	GetType() AssetType
	GetProvider() Provider
	GetEnvironment() Environment
	GetName() string
	GetBaseAsset() BaseAsset
	UpdateLastSeen()
}

// Interface methods implementations
func (a BaseAsset) GetID() string { return a.ID }
func (a BaseAsset) GetType() AssetType { return a.Type }
func (a BaseAsset) GetProvider() Provider { return a.Provider }
func (a BaseAsset) GetEnvironment() Environment { return a.Environment }
func (a BaseAsset) GetName() string { return a.Name }
func (a BaseAsset) GetBaseAsset() BaseAsset { return a }
func (a *BaseAsset) UpdateLastSeen() {
	a.LastSeen = time.Now()
	a.UpdatedAt = time.Now()
}

func (i Identity) GetBaseAsset() BaseAsset { return i.BaseAsset }
func (c Compute) GetBaseAsset() BaseAsset { return c.BaseAsset }
func (n Network) GetBaseAsset() BaseAsset { return n.BaseAsset }
func (d Data) GetBaseAsset() BaseAsset { return d.BaseAsset }
func (s SaaS) GetBaseAsset() BaseAsset { return s.BaseAsset }
func (f Finding) GetBaseAsset() BaseAsset { return f.BaseAsset }
