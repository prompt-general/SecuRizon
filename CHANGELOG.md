# Changelog

All notable changes to SecuRizon will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial Phase 1 implementation
- Core architecture and data models
- Graph database integration with Neo4j
- Event-driven architecture with Kafka
- Advanced risk scoring engine
- REST API gateway with comprehensive endpoints
- Docker development environment
- Cloud collector frameworks for AWS, Azure, and GCP

### Security
- Comprehensive security event processing
- Real-time risk assessment capabilities
- Attack path analysis framework

## [0.1.0] - 2026-01-26

### Added
- Project foundation and documentation
- Go module initialization with core dependencies
- Comprehensive build and development system
- Core asset data models (Identity, Compute, Network, Data, SaaS, Finding)
- Relationship and graph traversal models
- Comprehensive event system models
- Advanced risk scoring engine models
- JSON schema validation for event system
- Graph database interface and schema definition
- Neo4j graph database integration
- Kafka-based event bus system
- Real-time event processing engine
- Advanced risk scoring engine with caching
- REST API gateway with middleware support
- Comprehensive API handlers for all endpoints
- Complete Docker development environment
- MIT license for open source distribution
- Main application entry point
- Cloud collector stubs (AWS, Azure, GCP)

### Infrastructure
- Neo4j graph database for asset relationships
- Apache Kafka for event streaming
- Redis for caching
- TimescaleDB for metrics storage
- MinIO for object storage
- Grafana for visualization
- Prometheus for monitoring
- Jaeger for distributed tracing

### Documentation
- Comprehensive README with architecture overview
- API documentation through code comments
- Development setup instructions
- Docker deployment guides

---

## Development Notes

### Phase 1 Complete
This release marks the completion of Phase 1 of SecuRizon development,
establishing the foundational architecture for a real-time security
posture management platform.

### Key Features Implemented
- **Graph-based Asset Modeling**: Comprehensive asset types with time-aware relationships
- **Event-driven Architecture**: Real-time processing with Kafka integration
- **Advanced Risk Scoring**: Multi-factor risk calculation with caching
- **REST API**: Complete CRUD operations for all entities
- **Attack Path Analysis**: Security vulnerability mapping capabilities
- **Cloud Collectors**: Framework for AWS, Azure, and GCP integration

### Next Steps
Phase 2 will focus on:
- Complete cloud collector implementations
- Web dashboard development
- Advanced analytics and reporting
- Policy engine implementation
- Automated remediation capabilities
