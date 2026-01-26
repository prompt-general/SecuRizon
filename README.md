# SecuRizon - Real-time Security Posture Management Platform

## Overview

SecuRizon is a cloud-native, real-time security posture and attack surface management platform that continuously discovers assets, ingests events, correlates them with business and threat context, and presents actionable risk with attack path analysis.

## Architecture

```
System Architecture:
├── Data Plane (Collectors - OSS)
│   ├── AWS Collector
│   ├── Azure Collector  
│   ├── GCP Collector
│   └── SaaS Collectors (GitHub, Jira, etc.)
│
├── Event Bus (Kafka/Redpanda)
│   ├── Topic: asset.upserts
│   ├── Topic: asset.relationships
│   ├── Topic: security.events
│   └── Topic: policy.violations
│
├── Control Plane (Core Services)
│   ├── Event Ingestion Service
│   ├── Event Processor (with embedded Risk Engine)
│   ├── Graph Service (Neo4j interface)
│   ├── Policy Service
│   └── API Gateway
│
├── Storage Layer
│   ├── Graph DB (Neo4j) - Primary
│   ├── Time-Series DB (TimescaleDB) - Metrics
│   └── Object Store (S3) - Raw events
│
└── Presentation Layer
    ├── Dashboard (React)
    ├── CLI Tool
    └── External API
```

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Node.js 18+
- Python 3.11+

### Development Setup
```bash
# Clone and setup
git clone <repository>
cd securizon
make dev-setup
make dev-start
```

## Project Structure

- `/internal/` - Core application code
- `/pkg/` - Shared libraries
- `/api/` - API definitions
- `/collectors/` - Cloud provider collectors
- `/web/` - React dashboard
- `/deployments/` - Docker and Kubernetes configs
