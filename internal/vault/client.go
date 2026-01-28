package vault

import (
    "context"
    "fmt"
    "io/ioutil"
    "time"

    vault "github.com/hashicorp/vault/api"
)

type VaultClient struct {
    client *vault.Client
    config VaultConfig
}

type VaultConfig struct {
    Address       string        `yaml:"address"`
    Token         string        `yaml:"token"`
    Role          string        `yaml:"role"`
    Namespace     string        `yaml:"namespace,omitempty"`
    RenewInterval time.Duration `yaml:"renew_interval"`
}

type AWSCredentials struct {
    AccessKeyID     string
    SecretAccessKey string
    SecurityToken   string
    LeaseID         string
    LeaseDuration   int
}

type DatabaseCredentials struct {
    Username       string
    Password       string
    LeaseID        string
    LeaseDuration  int
}

func NewVaultClient(config VaultConfig) (*VaultClient, error) {
    // Create Vault client
    vaultConfig := vault.DefaultConfig()
    vaultConfig.Address = config.Address

    client, err := vault.NewClient(vaultConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create Vault client: %v", err)
    }

    // Set token
    client.SetToken(config.Token)

    // Set namespace if provided
    if config.Namespace != "" {
        client.SetNamespace(config.Namespace)
    }

    vc := &VaultClient{client: client, config: config}

    // Start token renewal if configured
    if config.RenewInterval > 0 {
        go vc.renewToken()
    }

    return vc, nil
}

func (vc *VaultClient) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
    secret, err := vc.client.Logical().ReadWithContext(ctx, path)
    if err != nil {
        return nil, fmt.Errorf("failed to read secret: %v", err)
    }
    if secret == nil {
        return nil, fmt.Errorf("secret not found at path: %s", path)
    }
    return secret.Data, nil
}

func (vc *VaultClient) GetAWSCredentials(ctx context.Context, role string) (*AWSCredentials, error) {
    path := fmt.Sprintf("aws/creds/%s", role)
    data, err := vc.GetSecret(ctx, path)
    if err != nil {
        return nil, err
    }
    creds := &AWSCredentials{
        AccessKeyID:     data["access_key"].(string),
        SecretAccessKey: data["secret_key"].(string),
        SecurityToken:   data["security_token"].(string),
        LeaseID:         data["lease_id"].(string),
        LeaseDuration:   int(data["lease_duration"].(float64)),
    }
    // Schedule lease renewal
    go vc.renewLease(creds.LeaseID, creds.LeaseDuration)
    return creds, nil
}

func (vc *VaultClient) GetDatabaseCredentials(ctx context.Context, role string) (*DatabaseCredentials, error) {
    path := fmt.Sprintf("database/creds/%s", role)
    data, err := vc.GetSecret(ctx, path)
    if err != nil {
        return nil, err
    }
    return &DatabaseCredentials{
        Username:       data["username"].(string),
        Password:       data["password"].(string),
        LeaseID:        data["lease_id"].(string),
        LeaseDuration:  int(data["lease_duration"].(float64)),
    }, nil
}

func (vc *VaultClient) renewToken() {
    ticker := time.NewTicker(vc.config.RenewInterval)
    defer ticker.Stop()
    for range ticker.C {
        secret, err := vc.client.Auth().Token().RenewSelf(0)
        if err != nil {
            // Try Kubernetes auth if role is set
            if vc.config.Role != "" {
                vc.kubernetesAuth()
            }
            continue
        }
        if secret != nil && secret.Auth != nil {
            vc.client.SetToken(secret.Auth.ClientToken)
        }
    }
}

func (vc *VaultClient) kubernetesAuth() {
    jwt, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
    if err != nil {
        return
    }
    secret, err := vc.client.Logical().Write("auth/kubernetes/login", map[string]interface{}{
        "role": vc.config.Role,
        "jwt":  string(jwt),
    })
    if err == nil && secret != nil && secret.Auth != nil {
        vc.client.SetToken(secret.Auth.ClientToken)
    }
}

func (vc *VaultClient) renewLease(leaseID string, leaseDuration int) {
    // Simple lease renewal loop – can be enhanced with backoff and error handling
    ticker := time.NewTicker(time.Duration(leaseDuration) * time.Second / 2)
    defer ticker.Stop()
    for range ticker.C {
        _, err := vc.client.Sys().Renew(leaseID, 0)
        if err != nil {
            // If renewal fails, attempt to re-fetch credentials
            // This is a placeholder – implement retry logic as needed
            return
        }
    }
}
