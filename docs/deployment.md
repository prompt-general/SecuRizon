# SecuRizon Deployment Guide

## Overview

This guide covers the deployment of SecuRizon across different environments, from local development to production clusters.

## Prerequisites

### System Requirements

- **CPU**: 4 cores minimum, 8 cores recommended
- **Memory**: 8GB minimum, 16GB recommended
- **Storage**: 50GB minimum, 100GB recommended
- **Network**: 1Gbps network connection

### Software Requirements

- **Docker**: 20.10+
- **Docker Compose**: 2.0+
- **Kubernetes**: 1.24+ (for cluster deployment)
- **Helm**: 3.8+ (optional)
- **Go**: 1.21+ (for building from source)

## Environment Types

### Development
- **Purpose**: Local development and testing
- **Infrastructure**: Docker Compose
- **Data**: Ephemeral, reset on restart
- **Monitoring**: Basic health checks

### Staging
- **Purpose**: Pre-production testing
- **Infrastructure**: Kubernetes cluster
- **Data**: Persistent, test data only
- **Monitoring**: Full monitoring stack

### Production
- **Purpose**: Production workloads
- **Infrastructure**: Kubernetes cluster with HA
- **Data**: Persistent, production data
- **Monitoring**: Comprehensive monitoring and alerting

## Quick Start (Development)

### 1. Clone Repository
```bash
git clone https://github.com/prompt-general/SecuRizon.git
cd SecuRizon
```

### 2. Setup Development Environment
```bash
chmod +x scripts/setup-dev.sh
./scripts/setup-dev.sh
```

### 3. Start Services
```bash
make dev-start
```

### 4. Verify Deployment
```bash
curl http://localhost:8080/api/v1/health
```

## Docker Compose Deployment

### Development Environment

#### Start Services
```bash
docker-compose -f deployments/docker/docker-compose.dev.yml up -d
```

#### Stop Services
```bash
docker-compose -f deployments/docker/docker-compose.dev.yml down
```

#### View Logs
```bash
docker-compose -f deployments/docker/docker-compose.dev.yml logs -f
```

#### Scale Services
```bash
docker-compose -f deployments/docker/docker-compose.dev.yml up -d --scale securizon=3
```

### Production Environment

#### Configuration
```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  securizon:
    image: securizon/securizon:latest
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/app/config/production.yaml
    volumes:
      - ./config:/app/config:ro
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '1.0'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

#### Deploy
```bash
docker-compose -f docker-compose.prod.yml up -d
```

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Persistent storage provisioner
- Ingress controller (optional)

### Namespace Setup

```bash
kubectl create namespace securizon
kubectl label namespace securizon name=securizon
```

### Secrets Management

#### Create Secrets
```bash
# Database credentials
kubectl create secret generic securizon-db-creds \
  --from-literal=username=neo4j \
  --from-literal=password=your-password \
  --namespace=securizon

# API keys
kubectl create secret generic securizon-api-keys \
  --from-literal=api-key=your-api-key \
  --namespace=securizon

# TLS certificates
kubectl create secret tls securizon-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  --namespace=securizon
```

### ConfigMaps

#### Create ConfigMap
```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: securizon-config
  namespace: securizon
data:
  config.yaml: |
    graph:
      uri: "bolt://neo4j:7687"
      database: "neo4j"
      username: "neo4j"
      password: "password"
    
    events:
      brokers:
        - "kafka:9092"
      client_id: "securizon-events"
    
    api:
      host: "0.0.0.0"
      port: 8080
      enable_auth: true
```

```bash
kubectl apply -f configmap.yaml
```

### Deployments

#### Main Application
```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: securizon
  namespace: securizon
  labels:
    app: securizon
    component: api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: securizon
      component: api
  template:
    metadata:
      labels:
        app: securizon
        component: api
    spec:
      containers:
      - name: securizon
        image: securizon/securizon:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_PATH
          value: "/app/config/config.yaml"
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: securizon-config
```

#### Collectors
```yaml
# collectors.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: collector-aws
  namespace: securizon
spec:
  replicas: 2
  selector:
    matchLabels:
      app: collector-aws
  template:
    metadata:
      labels:
        app: collector-aws
    spec:
      containers:
      - name: collector-aws
        image: securizon/collector-aws:latest
        env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: secret-access-key
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

### Services

#### API Service
```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: securizon-api
  namespace: securizon
spec:
  selector:
    app: securizon
    component: api
  ports:
  - name: http
    port: 80
    targetPort: 8080
  type: ClusterIP
```

#### LoadBalancer Service
```yaml
# loadbalancer.yaml
apiVersion: v1
kind: Service
metadata:
  name: securizon-lb
  namespace: securizon
spec:
  selector:
    app: securizon
    component: api
  ports:
  - name: http
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

### Ingress

#### HTTP Ingress
```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: securizon-ingress
  namespace: securizon
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - api.securizon.com
    secretName: securizon-tls
  rules:
  - host: api.securizon.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: securizon-api
            port:
              number: 80
```

### Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -n securizon
kubectl get services -n securizon

# Check logs
kubectl logs -f deployment/securizon -n securizon
```

## Helm Deployment

### Install Helm Chart

```bash
# Add repository
helm repo add securizon https://charts.securizon.com
helm repo update

# Install chart
helm install securizon securizon/securizon \
  --namespace securizon \
  --create-namespace \
  --set image.tag=latest \
  --set ingress.enabled=true \
  --set ingress.host=api.securizon.com
```

### Custom Values

```yaml
# values.yaml
image:
  repository: securizon/securizon
  tag: latest
  pullPolicy: IfNotPresent

replicaCount: 3

resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: api.securizon.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: securizon-tls
      hosts:
        - api.securizon.com

config:
  graph:
    uri: "bolt://neo4j:7687"
  events:
    brokers:
      - "kafka:9092"
```

```bash
helm install securizon securizon/securizon \
  --namespace securizon \
  --create-namespace \
  --values values.yaml
```

## Infrastructure Deployment

### AWS

#### EKS Cluster
```bash
# Create EKS cluster
eksctl create cluster \
  --name securizon \
  --region us-east-1 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 1 \
  --nodes-max 4 \
  --managed
```

#### RDS for Neo4j
```bash
# Create RDS instance
aws rds create-db-instance \
  --db-instance-identifier securizon-neo4j \
  --db-instance-class db.t3.medium \
  --engine neo4j \
  --master-username neo4j \
  --master-user-password your-password \
  --allocated-storage 100
```

#### MSK for Kafka
```bash
# Create MSK cluster
aws kafka create-cluster \
  --cluster-name securizon-kafka \
  --kafka-version 2.8.1 \
  --number-of-broker-nodes 3 \
  --broker-node-group-info file://broker-config.json
```

### Azure

#### AKS Cluster
```bash
# Create resource group
az group create --name securizon-rg --location eastus

# Create AKS cluster
az aks create \
  --resource-group securizon-rg \
  --name securizon-cluster \
  --node-count 3 \
  --enable-addons monitoring \
  --generate-ssh-keys
```

### GCP

#### GKE Cluster
```bash
# Create GKE cluster
gcloud container clusters create securizon \
  --zone us-central1-a \
  --num-nodes 3 \
  --enable-autoscaling \
  --min-nodes 1 \
  --max-nodes 5 \
  --enable-autorepair
```

## Monitoring and Logging

### Prometheus Monitoring

```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: securizon-metrics
  namespace: securizon
spec:
  selector:
    matchLabels:
      app: securizon
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

### Grafana Dashboard

```bash
# Import dashboard
kubectl create configmap grafana-dashboard \
  --from-file=deployments/grafana/dashboard.json \
  --namespace=monitoring
```

### ELK Stack Logging

```yaml
# fluentd.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentd
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: fluentd
  template:
    metadata:
      labels:
        name: fluentd
    spec:
      containers:
      - name: fluentd
        image: fluent/fluentd-kubernetes-daemonset:v1-debian-elasticsearch
        env:
        - name: FLUENT_ELASTICSEARCH_HOST
          value: "elasticsearch.logging.svc.cluster.local"
```

## Security Considerations

### Network Security

- Use network policies to restrict traffic
- Enable TLS encryption for all communications
- Implement firewall rules for external access

### Secrets Management

- Use Kubernetes secrets or external secret managers
- Rotate secrets regularly
- Audit secret access

### Pod Security

- Use non-root containers
- Implement pod security policies
- Enable security contexts

## Backup and Recovery

### Database Backups

```bash
# Neo4j backup
kubectl exec -it neo4j-0 -- neo4j-admin dump --database=neo4j --to=/backup/neo4j.dump

# Restore backup
kubectl exec -it neo4j-0 -- neo4j-admin load --from=/backup/neo4j.dump --database=neo4j
```

### Configuration Backups

```bash
# Backup Kubernetes manifests
kubectl get all -n securizon -o yaml > backup.yaml

# Backup Helm releases
helm get values securizon -n securizon > securizon-values.yaml
```

## Troubleshooting

### Common Issues

#### Pod Not Starting
```bash
# Check pod status
kubectl describe pod <pod-name> -n securizon

# Check logs
kubectl logs <pod-name> -n securizon

# Check events
kubectl get events -n securizon --sort-by=.metadata.creationTimestamp
```

#### Service Not Accessible
```bash
# Check service endpoints
kubectl get endpoints -n securizon

# Check network policies
kubectl get networkpolicies -n securizon

# Test connectivity
kubectl exec -it <pod-name> -n securizon -- curl http://service-name
```

#### Database Connection Issues
```bash
# Check database connectivity
kubectl exec -it securizon-xxx -n securizon -- nc -zv neo4j 7687

# Check database logs
kubectl logs neo4j-0 -n securizon
```

### Performance Tuning

#### Resource Allocation
- Monitor resource usage with Prometheus
- Adjust resource requests and limits
- Enable horizontal pod autoscaling

#### Database Optimization
- Tune Neo4j memory settings
- Optimize query performance
- Implement connection pooling

## Maintenance

### Rolling Updates

```bash
# Update deployment
kubectl set image deployment/securizon securizon=securizon:latest -n securizon

# Monitor rollout
kubectl rollout status deployment/securizon -n securizon

# Rollback if needed
kubectl rollout undo deployment/securizon -n securizon
```

### Scaling

```bash
# Scale deployment
kubectl scale deployment securizon --replicas=5 -n securizon

# Enable autoscaling
kubectl autoscale deployment securizon \
  --cpu-percent=70 \
  --min=2 \
  --max=10 \
  -n securizon
```

## Support

For deployment support:
- Documentation: https://docs.securizon.com
- Support: support@securizon.com
- Issues: https://github.com/prompt-general/SecuRizon/issues
