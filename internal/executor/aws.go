package executor

import (
    "context"
    "fmt"
    "log"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials/stscreds"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/iam"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/aws/aws-sdk-go-v2/service/sts"
)

type AWSRunner struct {
    clients map[string]*AWSClient // Keyed by region
    mu      sync.RWMutex
}

type AWSClient struct {
    S3  *s3.Client
    EC2 *ec2.Client
    IAM *iam.Client
    // ... other services
}

func NewAWSRunner() *AWSRunner {
    return &AWSRunner{
        clients: make(map[string]*AWSClient),
    }
}

func (r *AWSRunner) ExecuteStep(ctx context.Context, step Step, params map[string]interface{}) (map[string]interface{}, error) {
    // Parse inputs
    region, ok := params["region"].(string)
    if !ok {
        return nil, fmt.Errorf("region parameter missing or not a string")
    }

    // Get or create client for region
    client, err := r.getClient(ctx, region)
    if err != nil {
        return nil, fmt.Errorf("failed to get AWS client: %v", err)
    }

    // Route to appropriate handler based on action
    switch {
    case strings.HasPrefix(step.Action, "aws:s3:"):
        return r.executeS3Action(ctx, client.S3, step, params)
    case strings.HasPrefix(step.Action, "aws:ec2:"):
        return r.executeEC2Action(ctx, client.EC2, step, params)
    case strings.HasPrefix(step.Action, "aws:iam:"):
        return r.executeIAMAction(ctx, client.IAM, step, params)
    case strings.HasPrefix(step.Action, "verify:"):
        return r.executeVerification(ctx, step, params)
    default:
        return nil, fmt.Errorf("unsupported action: %s", step.Action)
    }
}

func (r *AWSRunner) executeS3Action(ctx context.Context, client *s3.Client, step Step, params map[string]interface{}) (map[string]interface{}, error) {
    bucketName, ok := params["bucket_name"].(string)
    if !ok {
        return nil, fmt.Errorf("bucket_name parameter missing or not a string")
    }

    switch step.Action {
    case "aws:s3:get-bucket-acl":
        resp, err := client.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: aws.String(bucketName)})
        if err != nil {
            return nil, fmt.Errorf("failed to get bucket ACL: %v", err)
        }
        grants := make([]map[string]interface{}, len(resp.Grants))
        for i, grant := range resp.Grants {
            grants[i] = map[string]interface{}{"grantee": grant.Grantee, "permission": grant.Permission}
        }
        return map[string]interface{}{"owner": resp.Owner, "grants": grants}, nil

    case "aws:s3:put-bucket-acl":
        acl, ok := params["acl"].(string)
        if !ok {
            return nil, fmt.Errorf("acl parameter missing or not a string")
        }
        _, err := client.PutBucketAcl(ctx, &s3.PutBucketAclInput{Bucket: aws.String(bucketName), ACL: types.BucketCannedACL(acl)})
        if err != nil {
            return nil, fmt.Errorf("failed to set bucket ACL: %v", err)
        }
        return map[string]interface{}{"bucket": bucketName, "acl": acl, "action": "updated"}, nil

    case "aws:s3:put-public-access-block":
        cfgMap, ok := params["public_access_block_configuration"].(map[string]interface{})
        if !ok {
            return nil, fmt.Errorf("public_access_block_configuration missing or invalid")
        }
        _, err := client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{Bucket: aws.String(bucketName), PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{BlockPublicAcls: cfgMap["BlockPublicAcls"].(bool), IgnorePublicAcls: cfgMap["IgnorePublicAcls"].(bool), BlockPublicPolicy: cfgMap["BlockPublicPolicy"].(bool), RestrictPublicBuckets: cfgMap["RestrictPublicBuckets"].(bool)}})
        if err != nil {
            return nil, fmt.Errorf("failed to set public access block: %v", err)
        }
        return map[string]interface{}{"bucket": bucketName, "public_access_block": "enabled"}, nil

    default:
        return nil, fmt.Errorf("unsupported S3 action: %s", step.Action)
    }
}

func (r *AWSRunner) executeVerification(ctx context.Context, step Step, params map[string]interface{}) (map[string]interface{}, error) {
    switch step.Action {
    case "verify:s3-bucket-private":
        return r.verifyS3BucketPrivate(ctx, params)
    case "verify:security-group-closed":
        return r.verifySecurityGroupClosed(ctx, params)
    default:
        return nil, fmt.Errorf("unsupported verification: %s", step.Action)
    }
}

func (r *AWSRunner) verifyS3BucketPrivate(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
    bucketName, ok := params["bucket_name"].(string)
    if !ok {
        return nil, fmt.Errorf("bucket_name missing or not a string")
    }
    region, ok := params["region"].(string)
    if !ok {
        return nil, fmt.Errorf("region missing or not a string")
    }
    client, err := r.getClient(ctx, region)
    if err != nil {
        return nil, err
    }
    // Check bucket ACL
    acl, err := client.S3.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: aws.String(bucketName)})
    if err != nil {
        return nil, fmt.Errorf("failed to get bucket ACL: %v", err)
    }
    for _, grant := range acl.Grants {
        if grant.Grantee.Type == types.TypeGroup && strings.Contains(*grant.Grantee.URI, "AllUsers") {
            return nil, fmt.Errorf("bucket still has public access")
        }
    }
    // Check public access block
    block, err := client.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucketName)})
    if err != nil {
        return nil, fmt.Errorf("failed to get public access block: %v", err)
    }
    if block.PublicAccessBlockConfiguration == nil ||
        !block.PublicAccessBlockConfiguration.BlockPublicAcls ||
        !block.PublicAccessBlockConfiguration.IgnorePublicAcls ||
        !block.PublicAccessBlockConfiguration.BlockPublicPolicy ||
        !block.PublicAccessBlockConfiguration.RestrictPublicBuckets {
        return nil, fmt.Errorf("public access block not fully enabled")
    }
    return map[string]interface{}{"verified": true, "message": "Bucket is properly secured"}, nil
}

func (r *AWSRunner) getClient(ctx context.Context, region string) (*AWSClient, error) {
    r.mu.RLock()
    client, exists := r.clients[region]
    r.mu.RUnlock()
    if exists {
        return client, nil
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    // Double-check after acquiring write lock
    if client, exists = r.clients[region]; exists {
        return client, nil
    }
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %v", err)
    }
    if roleArn := os.Getenv("AWS_REMEDIATION_ROLE_ARN"); roleArn != "" {
        stsClient := sts.NewFromConfig(cfg)
        creds := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
        cfg.Credentials = aws.NewCredentialsCache(creds)
    }
    client = &AWSClient{S3: s3.NewFromConfig(cfg), EC2: ec2.NewFromConfig(cfg), IAM: iam.NewFromConfig(cfg)}
    r.clients[region] = client
    return client, nil
}

// Placeholder methods for EC2, IAM, and additional verification steps – to be implemented as needed.
func (r *AWSRunner) executeEC2Action(ctx context.Context, client *ec2.Client, step Step, params map[string]interface{}) (map[string]interface{}, error) {
    log.Printf("EC2 action %s not yet implemented", step.Action)
    return nil, fmt.Errorf("EC2 actions not implemented")
}

func (r *AWSRunner) executeIAMAction(ctx context.Context, client *iam.Client, step Step, params map[string]interface{}) (map[string]interface{}, error) {
    log.Printf("IAM action %s not yet implemented", step.Action)
    return nil, fmt.Errorf("IAM actions not implemented")
}

func (r *AWSRunner) verifySecurityGroupClosed(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
    log.Printf("Security group verification not yet implemented")
    return nil, fmt.Errorf("security group verification not implemented")
}
