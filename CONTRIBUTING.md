# Contributing to SecuRizon

Thank you for your interest in contributing to SecuRizon! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Git
- Make

### Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/SecuRizon.git
   cd SecuRizon
   ```

3. Set up development environment:
   ```bash
   make dev-setup
   ```

4. Start development services:
   ```bash
   make dev-start
   ```

## Development Workflow

### Branch Strategy

- `main`: Production-ready code
- `develop`: Integration branch for features
- `feature/*`: Feature branches
- `bugfix/*`: Bug fix branches
- `hotfix/*`: Critical fixes

### Commit Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `style:` Code formatting changes
- `refactor:` Code refactoring
- `test:` Test additions or changes
- `chore:` Maintenance tasks

### Code Quality

- Run linter: `make lint`
- Run tests: `make test`
- Format code: `make format`
- Check security: `make security-scan`

## Project Structure

```
SecuRizon/
├── cmd/                    # Application entry points
├── internal/               # Private application code
│   ├── api/               # API gateway
│   ├── events/            # Event processing
│   ├── graph/             # Graph database
│   └── risk/              # Risk engine
├── pkg/                   # Public library code
│   ├── models/            # Data models
│   └── schemas/           # JSON schemas
├── deployments/           # Deployment configurations
├── config/               # Configuration files
└── scripts/              # Utility scripts
```

## Contributing Areas

### Core Components

1. **Graph Database**: Neo4j integration and schema management
2. **Event Processing**: Kafka-based event handling
3. **Risk Engine**: Risk calculation and propagation
4. **API Gateway**: REST API and middleware
5. **Collectors**: Cloud provider integrations

### Cloud Collectors

We welcome contributions for:

- **AWS Collector**: EC2, S3, IAM, VPC, Lambda, etc.
- **Azure Collector**: VMs, Storage, AD, Network, etc.
- **GCP Collector**: Compute Engine, Cloud Storage, IAM, etc.
- **SaaS Collectors**: GitHub, Jira, Salesforce, etc.

### Features

- **Policy Engine**: Security policy evaluation
- **Threat Intelligence**: Integration with threat feeds
- **Dashboard**: React-based UI
- **Reporting**: Security reports and analytics
- **Automation**: Remediation workflows

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

```bash
make test-integration
```

### Coverage

```bash
make test-coverage
```

## Documentation

- Update README.md for significant changes
- Add code comments for complex logic
- Update API documentation
- Maintain CHANGELOG.md

## Security

- Follow secure coding practices
- Run security scans: `make security-scan`
- Report security vulnerabilities privately
- Never commit secrets or credentials

## Pull Request Process

1. Create a feature branch from `develop`
2. Make your changes with proper commits
3. Ensure all tests pass
4. Update documentation as needed
5. Submit a pull request to `develop`
6. Address review feedback
7. Merge after approval

## Code Review Guidelines

- Review for functionality and correctness
- Check for security vulnerabilities
- Ensure code follows project conventions
- Verify tests are adequate
- Check documentation updates

## Release Process

1. Update version numbers
2. Update CHANGELOG.md
3. Create release tag
4. Build and publish artifacts
5. Update documentation

## Community

- GitHub Issues: Bug reports and feature requests
- GitHub Discussions: General questions and ideas
- Code Reviews: Participate in PR reviews

## Getting Help

- Check existing issues and documentation
- Ask questions in GitHub Discussions
- Join our community channels (when available)

## License

By contributing to SecuRizon, you agree that your contributions will be licensed under the MIT License.
