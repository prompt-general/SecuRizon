# Kubernetes Deployment Guide for Securizon

## Overview

This guide provides step-by-step instructions for deploying Securizon on a Kubernetes cluster using the manifests provided in this directory.

## Prerequisites

- Kubernetes cluster version 1.24 or higher
- `kubectl` configured with access to your cluster
- Storage classes configured (`fast-ssd` recommended for production)
- Sufficient compute resources:
  - **Production**: 8+ CPU cores, 32GB+ RAM
  - **Development**: 4+ CPU cores, 16GB+ RAM

## Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │           API Gateway (3 replicas)                       │   │
│  │  ├─ Service: LoadBalancer (:80)                          │   │
│  │  ├─ Metrics: :9090                                       │   │
│  │  └─ Health: :8081                                        │   │
│  └──────────────────────────────────────────────────────────┘   │
│                         ↓                                        │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │    Event Processor (3 replicas + HPA up to 10)           │   │
│  │  ├─ Kafka Consumer: Topics (asset, security, findings)   │   │
│  │  ├─ Neo4j: Graph operations                              │   │
│  │  ├─ Metrics: :9090                                       │   │
│  │  └─ Health: :8081                                        │   │
│  └──────────────────────────────────────────────────────────┘   │
│                         ↓                                        │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │      AWS Collector (2 replicas + HPA up to 5)            │   │
│  │  ├─ AWS API ingestion                                    │   │
│  │  ├─ Kafka Producer                                       │   │
│  │  ├─ Metrics: :9090                                       │   │
│  │  └─ Health: :8081                                        │   │
│  └──────────────────────────────────────────────────────────┘   │
│            ↙                    ↙                  ↙             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐    │
│  │  Kafka Cluster  │  │  Neo4j (1 node) │  │   Storage    │    │
│  │  (3 nodes)      │  │  StatefulSet    │  │   Classes    │    │
│  │  Headless SVC   │  │  Service        │  │  (fast-ssd)  │    │
│  └─────────────────┘  └─────────────────┘  └──────────────┘    │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## File Descriptions

- **aws-collector.yaml**: AWS collector deployment with scaling and monitoring
- **event-processor.yaml**: Event processing pipeline with policy engine
- **api-gateway.yaml**: REST API gateway with authentication and rate limiting
- **neo4j.yaml**: Graph database StatefulSet with persistence
- **kafka.yaml**: Event streaming cluster with 3 brokers
- **kustomization.yaml**: Kustomize configuration for unified deployments
- **DEPLOYMENT.md**: This file

## Step 1: Create Namespace and Storage Classes

```bash
# Create storage class for fast storage
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/aws-ebs
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
allowVolumeExpansion: true
EOF

# Verify namespace exists
kubectl create namespace securazion
```

## Step 2: Create Secrets

```bash
# AWS credentials
kubectl create secret generic aws-credentials \
  -n securazion \
  --from-literal=access_key=YOUR_AWS_ACCESS_KEY \
  --from-literal=secret_key=YOUR_AWS_SECRET_KEY

# Neo4j credentials
kubectl create secret generic neo4j-credentials \
  -n securazion \
  --from-literal=username=neo4j \
  --from-literal=password=YOUR_NEO4J_PASSWORD \
  --from-literal=neo4j_auth=neo4j/YOUR_NEO4J_PASSWORD

# Kafka credentials
kubectl create secret generic kafka-credentials \
  -n securazion \
  --from-literal=username=securazion \
  --from-literal=password=YOUR_KAFKA_PASSWORD \
  --from-literal=sasl_jaas_config='org.apache.kafka.common.security.scram.ScramLoginModule required username="securazion" password="YOUR_KAFKA_PASSWORD";'
```

## Step 3: Deploy Infrastructure (Kafka and Neo4j)

```bash
# Deploy Kafka cluster (wait for all pods to be ready)
kubectl apply -f kafka.yaml
kubectl wait --for=condition=ready pod -l component=event-broker -n securazion --timeout=600s

# Deploy Neo4j database
kubectl apply -f neo4j.yaml
kubectl wait --for=condition=ready pod -l component=graph-database -n securazion --timeout=600s
```

Verify readiness:
```bash
kubectl get statefulset -n securazion
kubectl get pvc -n securazion
```

## Step 4: Deploy Application Services

```bash
# Deploy collectors
kubectl apply -f aws-collector.yaml

# Deploy event processor
kubectl apply -f event-processor.yaml

# Deploy API gateway
kubectl apply -f api-gateway.yaml

# Wait for all deployments to be ready
kubectl wait --for=condition=available --timeout=600s deployment --all -n securazion
```

## Step 5: Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n securazion

# Check services
kubectl get svc -n securazion

# Check HPA status
kubectl get hpa -n securazion

# Check PVCs
kubectl get pvc -n securazion
```

## Step 6: Port Forwarding for Development

For local testing, forward ports:

```bash
# API Gateway
kubectl port-forward -n securazion svc/api-gateway 8080:8080

# Neo4j browser
kubectl port-forward -n securazion svc/neo4j 7474:7474 7687:7687

# Prometheus scrape
kubectl port-forward -n securazion svc/event-processor 9090:9090
```

## Monitoring and Health Checks

### Liveness and Readiness Probes

Each deployment includes:
- **Liveness probe**: Checks pod health every 10 seconds
- **Readiness probe**: Checks pod is ready to receive traffic every 5 seconds
- **Startup probe**: Allows extra time for startup (30 retries × 10s = 300s max)

### Metrics Collection

All components export Prometheus metrics on port 9090:

```bash
# Scrape metrics from event processor
curl http://localhost:9090/metrics | grep securizon

# Example metrics:
# securizon_events_processed_total
# securizon_findings_created_total
# securizon_api_requests_total
# securizon_kafka_lag
```

### Logs Collection

View logs for troubleshooting:

```bash
# AWS Collector logs
kubectl logs -n securazion -l component=collector --tail=50 -f

# Event Processor logs
kubectl logs -n securazion -l component=processor --tail=50 -f

# API Gateway logs
kubectl logs -n securazion -l component=api-gateway --tail=50 -f

# Neo4j logs
kubectl logs -n securazion -l component=graph-database --tail=50 -f

# Kafka logs
kubectl logs -n securazion -l component=event-broker --tail=50 -f
```

## Scaling

### Manual Scaling

```bash
# Scale AWS Collector
kubectl scale deployment aws-collector -n securazion --replicas=3

# Scale Event Processor
kubectl scale deployment event-processor -n securazion --replicas=5

# Scale API Gateway
kubectl scale deployment api-gateway -n securazion --replicas=4
```

### Automatic Scaling (HPA)

HPA is automatically configured and will scale based on CPU and memory utilization:

```bash
# Check HPA status
kubectl get hpa -n securazion -w

# View HPA details
kubectl describe hpa aws-collector-hpa -n securazion
```

## Configuration Updates

To update configuration:

```bash
# Edit ConfigMap
kubectl edit configmap securazion-collector-config -n securazion

# Changes apply upon pod restart:
kubectl rollout restart deployment aws-collector -n securazion

# Or update config and redeploy:
kubectl apply -f aws-collector.yaml
```

## Persistence and Data

### Neo4j Data

Neo4j data is persisted in a StatefulSet with PVC:
- **Size**: 50Gi (configurable in neo4j.yaml)
- **Storage Class**: fast-ssd
- **Backup**: Configure regular backups of `/var/lib/neo4j/data`

### Kafka Data

Kafka messages are persisted:
- **Size**: 100Gi per broker (configurable in kafka.yaml)
- **Retention**: 7 days (configurable via KAFKA_LOG_RETENTION_HOURS)
- **Replication**: 3 replicas per topic (configurable)

## Troubleshooting

### Pod not starting

```bash
kubectl describe pod <pod-name> -n securazion
kubectl logs <pod-name> -n securazion --previous  # Previous crash logs
```

### Connection issues

```bash
# Test connectivity between pods
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n securazion -- sh

# Inside debug pod:
curl http://api-gateway:8080/health
nc -zv kafka 9092
nc -zv neo4j 7687
```

### Storage issues

```bash
# Check PVC status
kubectl get pvc -n securazion
kubectl describe pvc neo4j-data-neo4j-0 -n securazion

# Check storage class
kubectl get storageclass
```

### Resource constraints

```bash
# Check node resources
kubectl top nodes

# Check pod resource usage
kubectl top pods -n securazion
```

## Cleanup

```bash
# Delete all resources in namespace
kubectl delete namespace securazion

# Or delete specific deployments
kubectl delete deployment aws-collector -n securazion
kubectl delete deployment event-processor -n securazion
kubectl delete deployment api-gateway -n securazion
kubectl delete statefulset kafka -n securazion
kubectl delete statefulset neo4j -n securazion
```

## Production Recommendations

1. **High Availability**:
   - Deploy across multiple availability zones
   - Use anti-affinity rules (included in manifests)
   - Set minAvailable in PodDisruptionBudgets

2. **Security**:
   - Enable TLS for API gateway (update api-gateway.yaml)
   - Use network policies (NetworkPolicy included for API gateway)
   - Implement RBAC (ServiceAccounts and roles included)
   - Scan container images for vulnerabilities

3. **Monitoring**:
   - Deploy Prometheus and Grafana
   - Configure alerts for critical metrics
   - Enable distributed tracing (Jaeger)

4. **Backup and Recovery**:
   - Implement automated Neo4j backups
   - Use ConfigMap versioning
   - Document disaster recovery procedures

5. **Updates**:
   - Use blue-green deployments for updates
   - Test in staging environment first
   - Monitor rollout progress with `kubectl rollout status`

## Next Steps

1. Deploy Prometheus and Grafana for monitoring
2. Configure Ingress for external access
3. Set up log aggregation (ELK stack or similar)
4. Implement backup and disaster recovery procedures
5. Configure autoscaling policies based on your workload
