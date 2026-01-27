package policy

import (
	"regexp"
	"sync"
	"time"

	"github.com/securazion/event-processor/internal/models"
)

type PolicyEngine struct {
	mu       sync.RWMutex
	policies []Policy
	enabled  map[string]bool
	compiled map[string]*CompiledPolicy
}

type Policy struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Severity    float64                `yaml:"severity"` // 0-10
	Category    string                 `yaml:"category"`
	Provider    string                 `yaml:"provider"` // aws, azure, gcp, all
	Resource    string                 `yaml:"resource"` // ec2, s3, iam, etc.
	Conditions  []Condition            `yaml:"conditions"`
	Remediation Remediation            `yaml:"remediation"`
	References  []string               `yaml:"references"` // CIS, NIST, etc.
	Metadata    map[string]interface{} `yaml:"metadata"`
}

type Condition struct {
	Field     string      `yaml:"field"`
	Operator  string      `yaml:"operator"` // eq, ne, gt, lt, contains, regex, in
	Value     interface{} `yaml:"value"`
}

type Remediation struct {
	Steps     []RemediationStep `yaml:"steps"`
	Automated bool              `yaml:"automated"`
	Playbook  string            `yaml:"playbook,omitempty"`
}

type RemediationStep struct {
	Action string `yaml:"action"`
	Script string `yaml:"script,omitempty"`
}

func NewEngine(policyConfig PolicyConfig) *PolicyEngine {
	engine := &PolicyEngine{
		policies: make([]Policy, 0),
		enabled:  make(map[string]bool),
		compiled: make(map[string]*CompiledPolicy),
	}
	
	// Load built-in policies
	engine.loadBuiltInPolicies()
	
	// Load custom policies from config
	for _, path := range policyConfig.PolicyPaths {
		engine.loadPoliciesFromPath(path)
	}
	
	// Compile all policies
	engine.compilePolicies()
	
	return engine
}

func (pe *PolicyEngine) EvaluateAsset(asset models.Asset) []models.Finding {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	
	var findings []models.Finding
	
	for _, policy := range pe.policies {
		// Skip if policy disabled
		if !pe.enabled[policy.ID] {
			continue
		}
		
		// Check if policy applies to this asset
		if !pe.appliesToAsset(policy, asset) {
			continue
		}
		
		// Evaluate conditions
		if pe.evaluateConditions(policy, asset) {
			finding := pe.createFinding(policy, asset)
			findings = append(findings, finding)
		}
	}
	
	return findings
}

func (pe *PolicyEngine) evaluateConditions(policy Policy, asset models.Asset) bool {
	compiled := pe.compiled[policy.ID]
	if compiled == nil {
		return false
	}
	
	for _, condition := range compiled.Conditions {
		fieldValue := pe.getFieldValue(asset, condition.Field)
		if !condition.Evaluate(fieldValue) {
			return false
		}
	}
	
	return true
}

func (pe *PolicyEngine) createFinding(policy Policy, asset models.Asset) models.Finding {
	return models.Finding{
		ID:          generateUUID(),
		AssetID:     asset.ID,
		PolicyID:    policy.ID,
		Title:       policy.Name,
		Description: policy.Description,
		Category:    policy.Category,
		BaseSeverity: policy.Severity,
		Status:      "open",
		FirstSeen:   time.Now().Unix(),
		LastSeen:    time.Now().Unix(),
		Evidence: map[string]string{
			"policy_id": policy.ID,
			"asset_id":  asset.ID,
			"provider":  asset.Provider,
			"type":      asset.Type,
		},
		Remediation: models.RemediationInfo{
			Steps:     policy.Remediation.Steps,
			Automated: policy.Remediation.Automated,
			Playbook:  policy.Remediation.Playbook,
		},
		References: policy.References,
	}
}

// Example built-in AWS policies
func (pe *PolicyEngine) loadBuiltInPolicies() {
	pe.policies = append(pe.policies, Policy{
		ID:          "AWS-S3-001",
		Name:        "S3 Bucket with Public Read Access",
		Description: "S3 bucket allows public read access without appropriate restrictions",
		Severity:    8.5,
		Category:    "misconfiguration",
		Provider:    "aws",
		Resource:    "s3_bucket",
		Conditions: []Condition{
			{
				Field:    "access_level",
				Operator: "in",
				Value:    []string{"public-read", "public-read-write"},
			},
		},
		Remediation: Remediation{
			Steps: []RemediationStep{
				{
					Action: "Update bucket ACL",
					Script: `aws s3api put-bucket-acl --bucket ${bucket} --acl private`,
				},
			},
			Automated: true,
		},
		References: []string{"CIS AWS 1.2", "AWS Foundational Security Best Practices"},
	})
	
	pe.policies = append(pe.policies, Policy{
		ID:          "AWS-EC2-001",
		Name:        "EC2 Instance with SSH Port Open to Internet",
		Description: "Security group allows SSH (port 22) from 0.0.0.0/0",
		Severity:    9.0,
		Category:    "misconfiguration",
		Provider:    "aws",
		Resource:    "ec2_instance",
		Conditions: []Condition{
			{
				Field:    "internet_exposed",
				Operator: "eq",
				Value:    true,
			},
			{
				Field:    "exposed_ports",
				Operator: "contains",
				Value:    22,
			},
		},
		Remediation: Remediation{
			Steps: []RemediationStep{
				{
					Action: "Update security group rules",
					Script: `aws ec2 revoke-security-group-ingress --group-id ${sg_id} --protocol tcp --port 22 --cidr 0.0.0.0/0`,
				},
			},
			Automated: false, // Requires approval
		},
	})
}
