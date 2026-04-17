# Kubernetes SRE Agent

An autonomous Kubernetes Site Reliability Engineering (SRE) agent that monitors cluster health, detects issues, performs auto-remediation, and can automatically roll back problematic deployments.

## Features

- **Real-time Monitoring**: Continuously watches pods, events, nodes, and deployments
- **Issue Detection**: Identifies CrashLoopBackOff, OOMKilled, ImagePullBackOff, and more
- **Auto-Remediation**: Automatically restarts pods, increases resources, and rolls back bad deployments
- **Automatic Rollback**: Rollback to previous version if a deployment causes instability
- **PostgreSQL Audit Log**: All actions are logged with evidence for compliance
- **Multi-Cloud Support**: Works with EKS, GKE, AKS, and standard Kubernetes
- **REST API + WebSocket**: Real-time streaming of cluster health and events
- **CLI Tool**: Full-featured CLI for manual operations

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    React Dashboard                          │
│     (Health, Pods, Nodes, Events, Actions, Rollback)       │
└─────────────────────────────────────────────────────────────┘
                              │
                     REST API + WebSocket
                              │
┌─────────────────────────────────────────────────────────────┐
│                       Go Agent                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Monitor  │→ │Diagnoser │→ │Remediator│→ │ Rollback │  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                          │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Kubernetes cluster (EKS/GKE/AKS/Standard)
- PostgreSQL (optional, for audit logging)

### Build

```bash
cd k8s-sre

# Build the agent
go build -o bin/agent ./cmd/agent

# Build the CLI
go build -o bin/k8s-sre ./cmd/cli
```

### Run Locally

```bash
# Without database (logs to stdout only)
./bin/agent --port 8080

# With PostgreSQL
./bin/agent --port 8080 --enable-db \
  --db-host localhost \
  --db-port 5432 \
  --db-name k8s_sre \
  --db-user postgres \
  --db-password yourpassword
```

### Deploy to Kubernetes

```bash
kubectl apply -f deployments/agent.yaml

# Port-forward to access the dashboard
kubectl port-forward -n k8s-sre svc/k8s-sre-agent 8080:80
```

Open http://localhost:8080 in your browser.

## CLI Usage

```bash
# Check cluster health
./bin/k8s-sre health --server http://localhost:8080

# List pods
./bin/k8s-sre pods
./bin/k8s-sre pods default

# List nodes
./bin/k8s-sre nodes

# View issues
./bin/k8s-sre issues

# View events
./bin/k8s-sre events

# Diagnose a pod
./bin/k8s-sre diagnose default myapp-pod

# Rollback a deployment
./bin/k8s-sre rollback default myapp --reason "Fixing crash loop"

# Restart a deployment
./bin/k8s-sre restart default myapp

# View audit logs
./bin/k8s-sre audit
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Cluster health status |
| `/api/pods` | GET | List all pods |
| `/api/nodes` | GET | List all nodes |
| `/api/deployments` | GET | List all deployments |
| `/api/events` | GET | List warning events |
| `/api/issues` | GET | List detected issues |
| `/api/actions` | GET | List remediation actions |
| `/api/audit` | GET | List audit logs |
| `/api/diagnose` | POST | Diagnose a pod |
| `/api/remediate` | POST | Trigger remediation |
| `/api/deployments/{ns}/{name}/rollback` | POST | Rollback deployment |
| `/api/deployments/{ns}/{name}/restart` | POST | Restart deployment |
| `/ws` | WS | WebSocket for real-time updates |

## Auto-Remediation Rules

| Issue Type | Action |
|------------|--------|
| CrashLoopBackOff | Restart pod, then rollback if persists |
| OOMKilled | Increase memory limit by 25% |
| ImagePullBackOff | Rollback to previous image |
| Node NotReady | Cordon node, drain workloads |
| Node Pressure | Drain node |
| Pending Pod | Restart pod |

## Rollback Policy

Rollback is triggered only if ALL conditions are met:

1. Issue started within 15 minutes of deployment
2. Error rate increased after deployment
3. Pods unhealthy for > 3 minutes
4. Previous version was stable for > 24 hours

## Safety Rules

- Never delete PVC or data volumes
- Never scale to zero replicas
- Never modify secrets directly
- Always verify rollback option before changes
- Require manual confirmation for destructive actions

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `POLL_INTERVAL` | 10s | Monitoring poll interval |
| `ENABLE_AUTO_REMEDIATION` | true | Enable automatic fixes |
| `ENABLE_AUTO_ROLLBACK` | true | Enable automatic rollback |
| `MEMORY_INCREASE_PERCENT` | 25 | Memory increase on OOM |
| `ROLLBACK_THRESHOLD` | 15m | Min time since deploy for rollback |

## Project Structure

```
k8s-sre/
├── cmd/
│   ├── agent/          # Main agent binary
│   └── cli/            # CLI tool
├── internal/
│   ├── agent/          # Core agent logic
│   │   ├── monitor/    # Kubernetes monitoring
│   │   ├── diagnose/   # Root cause analysis
│   │   ├── remediate/  # Auto-remediation
│   │   └── rollback/   # Rollback manager
│   ├── api/            # REST + WebSocket
│   ├── store/          # PostgreSQL storage
│   ├── k8s/            # Kubernetes client
│   └── models/         # Data models
├── frontend/           # React dashboard
└── deployments/        # K8s manifests
```

## License

MIT
