# NightOwl — Deployment & CI/CD

> This document covers building container images via GitHub Actions, pushing to GHCR, and deploying NightOwl to a Kubernetes cluster.

---

## 1. Container Images

NightOwl produces two container images from a single Dockerfile using multi-stage builds:

| Image | Contents | Purpose |
|-------|----------|---------|
| `ghcr.io/wisbric/nightowl` | Go binary (API + worker mode) | Backend: API server and background worker |
| `ghcr.io/wisbric/nightowl-web` | Nginx + React static build | Frontend: serves the SPA |

Both images are tagged with:
- `latest` — latest commit on `main`
- `v0.1.0` — semantic version on release tags
- `sha-abc1234` — short commit SHA for traceability

---

## 2. GitHub Actions CI/CD

### 2.1 CI Pipeline (`.github/workflows/ci.yml`)

Runs on every push and PR to `main`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: nightowl
          POSTGRES_PASSWORD: nightowl
          POSTGRES_DB: nightowl
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install sqlc
        run: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

      - name: Verify sqlc
        run: sqlc diff

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: Test
        run: make test
        env:
          DATABASE_URL: postgres://nightowl:nightowl@localhost:5432/nightowl?sslmode=disable
          REDIS_URL: redis://localhost:6379/0

      - name: Build
        run: make build

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Install
        run: cd web && npm ci

      - name: Lint
        run: cd web && npm run lint

      - name: Type check
        run: cd web && npx tsc --noEmit

      - name: Build
        run: cd web && npm run build
```

### 2.2 Release Pipeline (`.github/workflows/release.yml`)

Builds and pushes container images on tags and main branch pushes:

```yaml
name: Release

on:
  push:
    branches: [main]
    tags: ['v*']

env:
  REGISTRY: ghcr.io
  IMAGE_BACKEND: ghcr.io/wisbric/nightowl
  IMAGE_FRONTEND: ghcr.io/wisbric/nightowl-web

jobs:
  build-backend:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.IMAGE_BACKEND }}
          tags: |
            type=ref,event=branch
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha,prefix=sha-

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build and push backend
        uses: docker/build-push-action@v6
        with:
          context: .
          file: ./Dockerfile
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          platforms: linux/amd64,linux/arm64

  build-frontend:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.IMAGE_FRONTEND }}
          tags: |
            type=ref,event=branch
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha,prefix=sha-

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build and push frontend
        uses: docker/build-push-action@v6
        with:
          context: ./web
          file: ./web/Dockerfile
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          platforms: linux/amd64,linux/arm64
```

### 2.3 Frontend Dockerfile (`web/Dockerfile`)

The backend Dockerfile already exists. The frontend needs one:

```dockerfile
# Build stage
FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# Production stage
FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### 2.4 Frontend Nginx Config (`web/nginx.conf`)

```nginx
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA routing — all non-file requests go to index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API proxy — pass through to backend service
    location /api/ {
        proxy_pass http://nightowl-api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health check
    location /nginx-health {
        return 200 'ok';
        add_header Content-Type text/plain;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

---

## 3. Helm Deployment

### 3.1 Prerequisites

- Kubernetes 1.27+ cluster
- `kubectl` configured for target cluster
- `helm` 3.x installed
- GHCR image pull access (public images or imagePullSecret for private repo)
- PostgreSQL 16+ (CNPG recommended, or external)
- Redis 7+ (Bitnami Helm chart or external)

### 3.2 Quick Deploy

```bash
# Add NightOwl repo (if hosting chart in OCI/GHCR)
helm install nightowl oci://ghcr.io/wisbric/charts/nightowl \
  --namespace nightowl --create-namespace \
  -f values-production.yaml

# Or from local chart
helm install nightowl deploy/helm/nightowl/ \
  --namespace nightowl --create-namespace \
  -f values-production.yaml
```

### 3.3 Production `values-production.yaml`

```yaml
# -- Container images
image:
  repository: ghcr.io/wisbric/nightowl
  tag: "v0.1.0"  # Pin to release tag in production
  pullPolicy: IfNotPresent

frontend:
  enabled: true
  image:
    repository: ghcr.io/wisbric/nightowl-web
    tag: "v0.1.0"
  replicas: 2
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 128Mi

# -- Backend
api:
  replicas: 2
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi

worker:
  replicas: 1
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 300m
      memory: 256Mi

# -- Database (external PostgreSQL or CNPG)
database:
  # Option A: External PostgreSQL
  external:
    enabled: true
    url: "postgres://nightowl:CHANGEME@postgres-rw.nightowl.svc:5432/nightowl?sslmode=require"

  # Option B: CNPG (uncomment to use)
  # cnpg:
  #   enabled: true
  #   instances: 3
  #   storage:
  #     size: 20Gi
  #     storageClass: "longhorn"  # or your CSI driver
  #   backup:
  #     enabled: true
  #     s3:
  #       bucket: nightowl-backups
  #       endpoint: https://s3.eu-central-1.amazonaws.com

# -- Redis
redis:
  external:
    enabled: true
    url: "redis://redis-master.nightowl.svc:6379/0"
  # Or use Bitnami Redis subchart:
  # bitnami:
  #   enabled: true
  #   architecture: standalone
  #   auth:
  #     enabled: false

# -- Ingress
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
  hosts:
    - host: nightowl.example.com
      paths:
        - path: /
          pathType: Prefix
          service: frontend
        - path: /api
          pathType: Prefix
          service: api
        - path: /healthz
          pathType: Exact
          service: api
  tls:
    - secretName: nightowl-tls
      hosts:
        - nightowl.example.com

# -- OIDC Authentication
oidc:
  issuerUrl: "https://keycloak.example.com/realms/nightowl"
  clientId: "nightowl"

# -- Slack
slack:
  # Store in external secret or sealed secret
  botToken: ""
  signingSecret: ""

# -- Twilio (optional)
twilio:
  accountSid: ""
  authToken: ""
  fromNumber: ""

# -- Monitoring
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
  prometheusRule:
    enabled: true

# -- Pod Disruption Budget
pdb:
  enabled: true
  minAvailable: 1
```

### 3.4 Secrets Management

Never put secrets in `values.yaml`. Options in order of preference:

1. **External Secrets Operator** — syncs from Vault/AWS SSM/Azure KeyVault
   ```yaml
   apiVersion: external-secrets.io/v1beta1
   kind: ExternalSecret
   metadata:
     name: nightowl-secrets
   spec:
     secretStoreRef:
       name: vault
       kind: ClusterSecretStore
     target:
       name: nightowl-secrets
     data:
       - secretKey: DATABASE_URL
         remoteRef:
           key: nightowl/database-url
       - secretKey: SLACK_BOT_TOKEN
         remoteRef:
           key: nightowl/slack-bot-token
   ```

2. **Sealed Secrets** — encrypted in git, decrypted by controller
   ```bash
   kubeseal --format yaml < secret.yaml > sealed-secret.yaml
   ```

3. **Helm `--set`** — for quick deployments (not recommended for production)
   ```bash
   helm install nightowl deploy/helm/nightowl/ \
     --set database.external.url="postgres://..." \
     --set slack.botToken="xoxb-..."
   ```

---

## 4. CNPG PostgreSQL Setup

Recommended for production. Create before deploying NightOwl:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: nightowl-db
  namespace: nightowl
spec:
  instances: 3
  postgresql:
    parameters:
      shared_buffers: "256MB"
      effective_cache_size: "768MB"
      maintenance_work_mem: "128MB"
      max_connections: "100"
  storage:
    size: 20Gi
    storageClass: longhorn
  backup:
    barmanObjectStore:
      destinationPath: "s3://nightowl-backups/"
      s3Credentials:
        accessKeyId:
          name: nightowl-s3-creds
          key: ACCESS_KEY_ID
        secretAccessKey:
          name: nightowl-s3-creds
          key: SECRET_ACCESS_KEY
    retentionPolicy: "30d"
  monitoring:
    enablePodMonitor: true
```

---

## 5. Deployment Checklist

### Pre-deployment

- [ ] Kubernetes cluster accessible via `kubectl`
- [ ] Namespace created: `kubectl create namespace nightowl`
- [ ] Container images built and pushed to GHCR
- [ ] PostgreSQL running (CNPG or external) and connection string available
- [ ] Redis running and connection string available
- [ ] DNS record pointed to ingress IP/LB
- [ ] TLS certificate ready (cert-manager or manual)
- [ ] OIDC provider configured (Keycloak/Dex) with NightOwl client
- [ ] Secrets created (database URL, Slack tokens, OIDC config)

### Deploy

- [ ] `helm install` or `helm upgrade` with production values
- [ ] Verify pods running: `kubectl get pods -n nightowl`
- [ ] Verify health: `curl https://nightowl.example.com/healthz`
- [ ] Run migrations: migrations run automatically on API startup
- [ ] Seed initial tenant: `kubectl exec` into API pod, run seed command
- [ ] Verify frontend loads: `https://nightowl.example.com`
- [ ] Test OIDC login flow

### Post-deployment

- [ ] Configure Alertmanager webhook to `https://nightowl.example.com/api/v1/webhooks/alertmanager`
- [ ] Verify alert ingestion
- [ ] Set up Slack app and connect to workspace
- [ ] Create rosters for NZ and DE teams
- [ ] Configure escalation policies
- [ ] Import Grafana dashboard from `deploy/grafana/`
- [ ] Verify ServiceMonitor is scraped by Prometheus

---

## 6. Upgrading

```bash
# Update image tag
helm upgrade nightowl deploy/helm/nightowl/ \
  --namespace nightowl \
  --set image.tag=v0.2.0 \
  --set frontend.image.tag=v0.2.0 \
  -f values-production.yaml

# Migrations run automatically on API pod startup
# Rolling update ensures zero downtime (PDB enforces minAvailable)
```

### Rollback

```bash
helm rollback nightowl 1 --namespace nightowl
```

---

## 7. Multi-architecture Builds

The GitHub Actions pipeline builds for both `linux/amd64` and `linux/arm64`. This supports:
- Standard cloud nodes (amd64)
- ARM-based nodes (Graviton, Ampere, Apple Silicon dev)
- Mixed clusters with node affinity

---

## 8. Implementation Tasks for Claude Code

Add these to the existing codebase:

1. **Create `web/Dockerfile`** — multi-stage Node build + Nginx production image
2. **Create `web/nginx.conf`** — SPA routing + API proxy + static asset caching
3. **Create `.github/workflows/release.yml`** — build and push both images to GHCR on tag/push
4. **Update `.github/workflows/ci.yml`** — add frontend lint, typecheck, build jobs
5. **Add frontend deployment to Helm chart** — new deployment, service, and ingress path for `nightowl-web`
6. **Update `deploy/helm/nightowl/values.yaml`** — add frontend image config, replicas, resources
7. **Test**: `docker build .` and `docker build web/` both succeed locally
