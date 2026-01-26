# SecuRizon API Documentation

## Overview

The SecuRizon API provides a comprehensive REST interface for managing security assets, analyzing risk, and detecting attack paths in your cloud infrastructure.

## Base URL

```
Development: http://localhost:8080/api/v1
Production: https://api.securizon.com/api/v1
```

## Authentication

SecuRizon supports multiple authentication methods:

### API Key Authentication
```http
Authorization: Bearer <your-api-key>
```

### JWT Authentication
```http
Authorization: Bearer <your-jwt-token>
```

### OAuth2 Authentication
```http
Authorization: Bearer <oauth2-token>
```

## Rate Limiting

API requests are rate-limited to ensure fair usage:
- Default: 100 requests per minute
- Burst: 200 requests per minute

Rate limit headers are included in responses:
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640995200
```

## Response Format

All API responses follow a consistent format:

### Success Response
```json
{
  "success": true,
  "data": {
    // Response data
  },
  "meta": {
    "total": 100,
    "limit": 20,
    "offset": 0,
    "has_more": true
  }
}
```

### Error Response
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": "Asset ID is required"
  }
}
```

## HTTP Status Codes

- `200 OK` - Request successful
- `201 Created` - Resource created successfully
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

## API Endpoints

### Assets

#### List Assets
```http
GET /assets
```

Query Parameters:
- `type` - Asset type filter (identity, compute, network, data, saas)
- `provider` - Provider filter (aws, azure, gcp, github, jira)
- `environment` - Environment filter (prod, staging, dev, test)
- `min_risk_score` - Minimum risk score filter
- `max_risk_score` - Maximum risk score filter
- `limit` - Number of results to return (default: 50)
- `offset` - Number of results to skip (default: 0)

Example:
```http
GET /assets?type=compute&provider=aws&environment=prod&limit=20
```

#### Create Asset
```http
POST /assets
```

Request Body:
```json
{
  "asset": {
    "provider": "aws",
    "type": "compute",
    "environment": "prod",
    "name": "web-server-01",
    "description": "Production web server",
    "metadata": {
      "instance_type": "t3.large",
      "region": "us-east-1"
    }
  }
}
```

#### Get Asset
```http
GET /assets/{id}
```

#### Update Asset
```http
PUT /assets/{id}
```

#### Delete Asset
```http
DELETE /assets/{id}
```

#### Search Assets
```http
POST /assets/search
```

Request Body:
```json
{
  "query": "web-server",
  "types": ["compute"],
  "providers": ["aws"],
  "limit": 50
}
```

#### Get Asset Neighbors
```http
GET /assets/{id}/neighbors?direction=both&max_depth=3
```

#### Get Asset Risk
```http
GET /assets/{id}/risk
```

#### Get Asset Findings
```http
GET /assets/{id}/findings
```

### Relationships

#### List Relationships
```http
GET /relationships
```

Query Parameters:
- `asset_id` - Filter by asset ID
- `type` - Relationship type filter
- `min_strength` - Minimum relationship strength
- `max_strength` - Maximum relationship strength
- `limit` - Number of results to return

#### Create Relationship
```http
POST /relationships
```

Request Body:
```json
{
  "relationship": {
    "from_asset_id": "asset-123",
    "to_asset_id": "asset-456",
    "type": "HAS_ACCESS_TO",
    "strength": 0.8,
    "properties": {
      "access_type": "read",
      "protocol": "https"
    }
  }
}
```

#### Get Relationship
```http
GET /relationships/{id}
```

#### Update Relationship
```http
PUT /relationships/{id}
```

#### Delete Relationship
```http
DELETE /relationships/{id}
```

### Findings

#### List Findings
```http
GET /findings
```

Query Parameters:
- `status` - Finding status filter (open, resolved, suppressed)
- `severity` - Severity filter (0-10)
- `asset_id` - Filter by asset ID
- `limit` - Number of results to return

#### Create Finding
```http
POST /findings
```

Request Body:
```json
{
  "finding": {
    "policy_id": "CIS-1.2",
    "severity": 8.5,
    "risk_score": 72,
    "status": "open",
    "description": "S3 bucket is publicly accessible",
    "recommendation": "Enable bucket ACL restrictions",
    "asset_id": "asset-123"
  }
}
```

#### Get Finding
```http
GET /findings/{id}
```

#### Update Finding
```http
PUT /findings/{id}
```

#### Resolve Finding
```http
POST /findings/{id}/resolve
```

### Risk Management

#### Get Risk Summary
```http
GET /risk/summary
```

Response:
```json
{
  "success": true,
  "data": {
    "total_assets": 1250,
    "assets_by_type": {
      "compute": 450,
      "network": 200,
      "data": 300,
      "identity": 250,
      "saas": 50
    },
    "risk_distribution": {
      "critical": 15,
      "high": 45,
      "medium": 120,
      "low": 350,
      "info": 720
    },
    "average_risk": 23.5,
    "high_risk_assets": ["asset-123", "asset-456"],
    "critical_findings": 8
  }
}
```

#### Get Risk Trends
```http
GET /risk/trends/{assetId}?start_time=2024-01-01T00:00:00Z&end_time=2024-01-31T23:59:59Z
```

#### Recalculate Risk
```http
POST /risk/recalculate
```

Request Body:
```json
{
  "asset_ids": ["asset-123", "asset-456"]
}
```

#### Batch Recalculate Risk
```http
POST /risk/batch-recalculate
```

### Attack Path Analysis

#### Find Attack Paths
```http
POST /attack-paths/find
```

Request Body:
```json
{
  "entry_points": ["internet-exposed-vm"],
  "targets": ["sensitive-database"],
  "max_depth": 5,
  "min_risk_score": 50
}
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "edges": [
        {
          "relationship": {
            "id": "rel-123",
            "type": "CONNECTED_TO",
            "from_asset_id": "internet-vm",
            "to_asset_id": "internal-server",
            "strength": 0.9
          },
          "from_asset": {
            "id": "internet-vm",
            "type": "compute",
            "name": "web-server-01"
          },
          "to_asset": {
            "id": "internal-server",
            "type": "compute",
            "name": "app-server-01"
          }
        }
      ],
      "nodes": [
        {
          "id": "internet-vm",
          "type": "compute",
          "name": "web-server-01"
        }
      ],
      "total_weight": 75.5,
      "length": 3
    }
  ]
}
```

#### Find Path
```http
POST /attack-paths/path
```

Request Body:
```json
{
  "from_asset_id": "asset-123",
  "to_asset_id": "asset-456",
  "max_depth": 3
}
```

### Health and Monitoring

#### Health Check
```http
GET /health
```

Response:
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "timestamp": "2024-01-26T10:30:00Z",
    "version": "1.0.0",
    "graph_store": {
      "status": "ok"
    },
    "event_bus": {
      "status": "ok"
    }
  }
}
```

#### Metrics
```http
GET /metrics
```

## SDKs and Client Libraries

### Go SDK
```go
import "github.com/securizon/go-sdk"

client := securizon.NewClient("your-api-key")
assets, err := client.Assets.List(&securizon.AssetListOptions{
    Type: securizon.AssetTypeCompute,
    Provider: securizon.ProviderAWS,
})
```

### Python SDK
```python
from securizon import Client

client = Client(api_key="your-api-key")
assets = client.assets.list(
    type="compute",
    provider="aws"
)
```

### JavaScript SDK
```javascript
import { SecuRizonClient } from '@securizon/js-sdk';

const client = new SecuRizonClient({ apiKey: 'your-api-key' });
const assets = await client.assets.list({
    type: 'compute',
    provider: 'aws'
});
```

## Webhooks

SecuRizon supports webhooks for real-time event notifications:

### Configure Webhook
```http
POST /webhooks
```

Request Body:
```json
{
  "url": "https://your-webhook-endpoint.com/events",
  "events": ["asset.created", "finding.created", "risk.score_changed"],
  "secret": "your-webhook-secret"
}
```

### Webhook Payload
```json
{
  "event": "asset.created",
  "timestamp": "2024-01-26T10:30:00Z",
  "data": {
    "asset": {
      "id": "asset-123",
      "type": "compute",
      "name": "web-server-01"
    }
  },
  "signature": "sha256=..."
}
```

## Error Handling

### Common Error Codes

- `VALIDATION_ERROR` - Invalid request parameters
- `AUTHENTICATION_ERROR` - Authentication failed
- `AUTHORIZATION_ERROR` - Insufficient permissions
- `RESOURCE_NOT_FOUND` - Resource not found
- `RATE_LIMIT_EXCEEDED` - Rate limit exceeded
- `INTERNAL_ERROR` - Internal server error

### Retry Strategy

For transient errors (5xx status codes), implement exponential backoff:
- Initial delay: 1 second
- Maximum delay: 30 seconds
- Maximum retries: 5

## Pagination

List endpoints support pagination using `limit` and `offset` parameters:
- Maximum limit: 1000
- Default limit: 50

Response includes pagination metadata:
```json
{
  "meta": {
    "total": 1250,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

## Filtering and Sorting

### Filtering
Most list endpoints support filtering by various attributes using query parameters.

### Sorting
Sort results using the `sort` parameter:
```http
GET /assets?sort=risk_score:desc,name:asc
```

## API Versioning

SecuRizon uses URL path versioning:
- Current version: `/api/v1`
- Previous versions: `/api/v1.0`

Backward compatibility is maintained for at least 6 months after deprecation.

## Support

For API support:
- Documentation: https://docs.securizon.com
- Support: support@securizon.com
- Issues: https://github.com/prompt-general/SecuRizon/issues
