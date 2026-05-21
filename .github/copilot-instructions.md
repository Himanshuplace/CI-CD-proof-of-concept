# Copilot Instructions — Principal Engineer Standards
# Domains: Go · DevOps · GitLab CI/CD · Kubernetes · Docker · Automation · Agentification

> These instructions define how code, pipelines, configs, and AI agent patterns are written
> in this repository. Every suggestion must reflect production-grade, principal-engineer thinking.
> Prioritise correctness, operability, and explainability over cleverness.

---

## 0. Core Engineering Philosophy

- **Understand before generating.** Read surrounding code, existing patterns, and infra context
  before suggesting anything. Never assume; infer from evidence.
- **Explicit over implicit.** Make intent visible in code, config, and comments. Future-you
  reading at 2 AM during an incident is the target audience.
- **Fail fast, fail loudly.** Silent failures, swallowed errors, and missing alerts cost money.
  Surface problems as early and as clearly as possible.
- **Tradeoffs are first-class.** Every non-trivial design decision has a tradeoff. Name it.
  Document why a choice was made, not just what was chosen.
- **Idempotency is non-negotiable.** Every operation — deploy, migration, automation script,
  API call — must be safe to run more than once.
- **Observability is not optional.** If you cannot measure it in production, you cannot trust it.
- **Security is a design constraint, not a review step.** Treat secrets, RBAC, network policies,
  and supply-chain integrity as requirements, not afterthoughts.

---

## 1. Go — Language & Idioms

### 1.1 Error Handling

```go
// ALWAYS wrap errors with context. Naked returns destroy stack clarity.
if err != nil {
    return fmt.Errorf("fetchUser(id=%d): %w", id, err)
}

// Use sentinel errors for callers that need to branch on error type.
var ErrNotFound = errors.New("not found")

// Use custom error types when you need structured fields for logging/tracing.
type ValidationError struct {
    Field   string
    Message string
}
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// NEVER do this — the error is swallowed, the bug is hidden.
result, _ := doSomething()
```

**Rule:** Every error path must either return (with context), log-and-return, or be explicitly
acknowledged with a comment explaining why it is safe to ignore. Zero tolerance for `_ =` on
errors that represent real failure modes.

**Tradeoff — sentinel vs typed errors:** Sentinel errors couple callers to your package's error
space. Custom types give more structure but add boilerplate. Use sentinels for simple leaf-level
conditions; use typed errors when callers need to inspect fields.

### 1.2 Goroutines — Lifecycle Ownership

**Rule:** Every goroutine you launch must have a documented, guaranteed exit path. Goroutine
leaks are memory leaks that accumulate silently.

```go
// CORRECT — goroutine respects context cancellation.
func (w *Worker) Start(ctx context.Context) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return // clean exit
            case job := <-w.jobs:
                w.process(job)
            }
        }
    }()
}

// WRONG — this goroutine lives forever if nobody closes the channel.
go func() {
    for job := range w.jobs {
        process(job)
    }
}()
```

- Use `sync.WaitGroup` to wait for goroutine completion before shutdown.
- Use `errgroup.Group` (golang.org/x/sync/errgroup) for goroutine fan-out that must aggregate errors.
- Never start goroutines from `init()` or package-level constructors.
- Always give goroutines a name in logs/traces for debuggability.

**Tradeoff — errgroup vs WaitGroup:** `errgroup` cancels sibling goroutines on first error —
good for "all must succeed" fan-out. `WaitGroup` is appropriate when goroutines are independent
and partial failure is tolerable.

### 1.3 Channels

```go
// Use directional channels in function signatures. Signal intent.
func produce(out chan<- Event) {}
func consume(in <-chan Event)  {}

// Buffered channels: document WHY the buffer size was chosen.
// A buffer of 1 is often enough to decouple sender from receiver by one cycle.
// A large buffer is often hiding a flow-control problem — think twice.
ch := make(chan Result, 100) // 100: max in-flight RPCs before backpressure kicks in

// Close channels from the SENDER only. Receivers never close.
// Closing from receiver is a data race waiting to happen.

// Use select with a default ONLY when non-blocking behavior is intentional.
select {
case ch <- val:
default:
    metrics.DroppedMessages.Inc() // never silently drop
}
```

### 1.4 Context Propagation

- `context.Context` must be the **first parameter** of every function that does I/O, makes
  network calls, or calls into any potentially blocking operation.
- Never store context in a struct field. Pass it explicitly.
- Attach request-scoped values (trace IDs, user IDs) via `context.WithValue` with unexported
  typed keys — never string keys, to avoid collisions across packages.
- Propagate cancellation: if your caller can cancel, your downstream calls must respect that.

```go
type contextKey struct{ name string }
var traceIDKey = contextKey{"traceID"}

func WithTraceID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, traceIDKey, id)
}
func TraceIDFrom(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(traceIDKey).(string)
    return id, ok
}
```

### 1.5 Interface Design

- Interfaces belong in the **consumer** package, not the producer package (Go proverb: accept
  interfaces, return concrete types).
- Keep interfaces small. One or two methods. Compose larger behaviours from small interfaces.
- Do not define an interface until you have at least two concrete implementations or a clear
  testability need.

```go
// Good — small, composable, testable.
type Sender interface {
    Send(ctx context.Context, msg Message) error
}

// Bad — this is just a class hierarchy from Java, transplanted to Go.
type MessageServiceInterface interface {
    Send(...)
    Receive(...)
    Acknowledge(...)
    Retry(...)
    ListQueues(...)
}
```

### 1.6 Struct Design

- Embed interfaces for test doubles, not for production composition — embedding an interface in
  a struct gives you a nil-panic landmine unless every method is explicitly overridden.
- Order struct fields from largest to smallest alignment to minimise struct padding.
- Use constructor functions (`NewX(...)`) to enforce invariants, not raw struct literals.
- Clearly separate config (injected at build time) from state (mutated at runtime) — ideally
  in separate structs.

```go
type Server struct {
    cfg  ServerConfig  // immutable after construction
    mu   sync.RWMutex  // guards mutable state below
    conns map[string]*Conn
}
```

### 1.7 Concurrency Patterns

**Worker Pool — use when you need bounded parallelism:**

```go
func WorkerPool(ctx context.Context, jobs <-chan Job, n int) <-chan Result {
    results := make(chan Result, n)
    var wg sync.WaitGroup
    for i := 0; i < n; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                select {
                case <-ctx.Done():
                    return
                default:
                    results <- process(job)
                }
            }
        }()
    }
    go func() { wg.Wait(); close(results) }()
    return results
}
```

**Tradeoff:** Bounded pools prevent OOM under load spikes. Unbounded goroutine spawning is
fast to write but will kill your process under load.

### 1.8 Memory & GC

- Use `sync.Pool` to reuse short-lived allocations in hot paths (e.g., byte buffers,
  temporary slices), but only after profiling confirms allocations are a bottleneck.
- Preallocate slices when length is known: `make([]T, 0, capacity)`.
- Preallocate maps when cardinality is known: `make(map[K]V, hint)`.
- `strings.Builder` over `+` concatenation in loops.
- Set `GOGC` and `GOMEMLIMIT` explicitly in production — do not rely on defaults for
  latency-sensitive services.

**Tradeoff — `GOGC` vs `GOMEMLIMIT`:** `GOGC` controls GC frequency relative to heap growth
(lower = more GC = lower memory, higher latency cost). `GOMEMLIMIT` (Go 1.19+) caps total
memory usage — prefer this for containerised workloads where OOM kills are worse than GC pauses.

### 1.9 Testing

```go
// Table-driven tests are mandatory for functions with multiple input conditions.
func TestValidatePayload(t *testing.T) {
    tests := []struct {
        name    string
        input   Payload
        wantErr bool
    }{
        {"valid payload", Payload{...}, false},
        {"missing field", Payload{...}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePayload(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
            }
        })
    }
}
```

- Use `t.Parallel()` in subtests unless the test mutates shared state.
- Use `testify/assert` or `testify/require` consistently — `require` stops the test immediately,
  `assert` continues; choose based on whether subsequent checks are meaningful after a failure.
- Write benchmarks for any function in a hot path: `func BenchmarkX(b *testing.B)`.
- Use `go test -race` in CI — every time, unconditionally.
- Fuzz critical parsing/validation logic: `func FuzzParseX(f *testing.F)`.
- Test behaviour, not implementation. If your test breaks when you rename a private function,
  it is testing the wrong thing.

### 1.10 Logging & Observability

- Use `log/slog` (Go 1.21+) for structured logging. No `fmt.Println` in production code paths.
- Log at the boundary where context is richest — not deep in utility functions.
- Every log entry in a request path must include: trace ID, operation name, duration (for
  completed operations), and outcome.
- Never log secrets, tokens, PII, or full request bodies by default.
- Expose `/metrics` (Prometheus) and `/healthz` (liveness) and `/readyz` (readiness) on every service.

```go
slog.Info("processed payment",
    "trace_id", traceID,
    "payment_id", p.ID,
    "amount_cents", p.AmountCents,
    "duration_ms", time.Since(start).Milliseconds(),
)
```

### 1.11 Package Structure & Naming

```
/cmd/servicename/main.go   — thin entrypoint, wire dependencies, start server
/internal/                 — private packages, not importable externally
/internal/domain/          — pure business logic, zero framework dependencies
/internal/transport/       — HTTP/gRPC/WebSocket handlers
/internal/store/           — database/cache access
/internal/config/          — configuration loading and validation
/pkg/                      — packages safe to import by external consumers
```

- `main.go` does only three things: parse config, wire dependencies, block on signals.
- Never import `internal/` packages across services — that defeats the purpose.
- Package names: lowercase, no underscores, no plurals (`store` not `stores`).
- Avoid `util`, `helper`, `common` — these are naming failures. Name the actual responsibility.

### 1.12 Module Hygiene

- Pin all dependencies. Run `go mod tidy` after every dependency change.
- Use `go mod vendor` for hermetic builds in CI.
- Run `govulncheck` in CI to catch known CVEs in dependencies.
- Do not use `replace` directives in production modules — they create invisible coupling.
  Use a private module proxy or workspace if needed for local development.

---

## 2. DevOps — Principles & Practices

### 2.1 Infrastructure as Code

- **All infrastructure is code.** No manual console changes. Ever. If you clicked something
  in a UI to fix production, that click must immediately become a commit.
- Use Terraform / OpenTofu for cloud infra, Helm + Kustomize for K8s resources.
- State files are secrets. Backend state must be remote (S3/GCS/Terraform Cloud), encrypted,
  and access-controlled.
- Use workspaces or directory-per-environment (`envs/dev/`, `envs/prod/`) — not variables alone.
- Every resource must have owner, environment, and managed-by tags.

```hcl
# Required tags on every resource
tags = {
  environment = var.environment
  owner       = "platform-team"
  managed-by  = "terraform"
  repo        = "gitlab.example.com/org/infra"
}
```

### 2.2 GitOps

- The Git repository is the single source of truth for deployment state.
- No `kubectl apply` or `helm install` from developer laptops in production.
- Production deployments happen only via CI/CD pipeline on a protected branch.
- Use ArgoCD or Flux for continuous reconciliation — drift between Git and cluster state
  must be detected and alerted on automatically.

### 2.3 Secrets Management

- Secrets never live in Git. Not encrypted. Not base64-encoded. Never.
- Use a secrets manager: HashiCorp Vault, AWS Secrets Manager, or GCP Secret Manager.
- In K8s, use External Secrets Operator to sync secrets manager entries into K8s Secrets.
- Rotate secrets on a schedule; rotation must be automated.
- Audit who accessed what secret, when — this is a compliance requirement, not a nice-to-have.

### 2.4 Observability Stack

Every production service must expose:
- **Metrics:** Prometheus-format `/metrics`. RED metrics (Rate, Errors, Duration) for every
  external-facing operation. USE metrics (Utilisation, Saturation, Errors) for every resource.
- **Logs:** Structured JSON, shipped to a log aggregator (Loki, Elasticsearch, CloudWatch).
  Correlation via trace ID is mandatory.
- **Traces:** OpenTelemetry SDK, exported to Jaeger/Tempo or a managed provider.
- **Alerts:** SLO-based alerts (error rate > budget, latency p99 > threshold) before
  symptom-based alerts (CPU > 80% is not an alert; slow requests are).

**Tradeoff — push vs pull for metrics:** Prometheus pull model is simpler to operate and
self-heals when scrape fails. Push (Pushgateway, OTLP) is needed for batch jobs and short-lived
containers. Use pull by default; push only where pull cannot reach.

### 2.5 On-Call and Incident Readiness

- Every alert must have a corresponding runbook. No runbook = no alert. Ship both together.
- Runbooks must include: what the alert means, immediate mitigation steps, escalation path,
  and post-incident action.
- Mean Time to Detect (MTTD) and Mean Time to Recover (MTTR) are metrics you track, not
  aspirations you mention in retros.

---

## 3. GitLab CI/CD

### 3.1 Pipeline Design Principles

- Pipelines must be **fast, reproducible, and observable**.
- Fail fast: put cheapest checks first (lint, vet, unit tests), expensive checks later
  (integration tests, security scans, builds).
- Every stage must have a clear purpose; no "misc" stages.
- Pipelines must be idempotent: re-running a failed job must produce the same outcome.

### 3.2 Standard Pipeline Structure

```yaml
stages:
  - validate      # lint, format check, vet, go mod tidy check
  - test          # unit tests with -race, coverage enforcement
  - security      # govulncheck, SAST, secret scanning, dependency audit
  - build         # docker build, artifact creation
  - publish       # push to registry with immutable tags
  - deploy-dev    # auto-deploy to dev environment
  - deploy-staging # manual gate or auto on main
  - deploy-prod   # manual gate, protected, requires approval
```

### 3.3 Job Authoring Rules

```yaml
# ALWAYS pin image versions. `latest` is a ticking time bomb.
image: golang:1.23.4-alpine3.20

# ALWAYS set resource limits on runners for predictable behaviour.
# ALWAYS define when conditions to avoid unnecessary job runs.
unit-test:
  stage: test
  image: golang:1.23.4-alpine3.20
  cache:
    key: "$CI_COMMIT_REF_SLUG-go-mod"
    paths:
      - .go/pkg/mod/
  variables:
    GOPATH: "$CI_PROJECT_DIR/.go"
    GOFLAGS: "-mod=vendor"
  script:
    - go test -race -coverprofile=coverage.out ./...
    - go tool cover -func=coverage.out | tail -1
  coverage: '/total:\s+\(statements\)\s+(\d+\.\d+)%/'
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage.xml
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
```

### 3.4 Image Tagging Strategy

```yaml
# Immutable tags: never use `latest` for production images.
# Use commit SHA for exact traceability + semantic version for release promotion.
variables:
  IMAGE_TAG_COMMIT: "$CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA"
  IMAGE_TAG_BRANCH: "$CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG"
  # For releases:
  IMAGE_TAG_VERSION: "$CI_REGISTRY_IMAGE:$CI_COMMIT_TAG"
```

**Tradeoff — SHA vs semver tags:** SHA gives you exact traceability but no human-readable
version. Semver is human-friendly but requires discipline to not re-tag. Use both: SHA for
internal references, semver for release channels.

### 3.5 Environment Promotion

```yaml
deploy-prod:
  stage: deploy-prod
  environment:
    name: production
    url: https://api.example.com
  rules:
    # Only from main branch, only on tag, requires manual trigger.
    - if: '$CI_COMMIT_TAG && $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
      when: manual
  script:
    - kubectl set image deployment/api api=$IMAGE_TAG_VERSION --record
  # Protect this job: only specific roles can trigger.
```

### 3.6 Security in CI

- **SAST:** Enable GitLab SAST template or use `gosec` explicitly.
- **Secret scanning:** Enable GitLab secret detection on every MR.
- **Dependency scanning:** Run `govulncheck` and `trivy` on every build.
- **Container scanning:** Scan built images before pushing to registry.
- **Never print environment variables in CI logs.** Use `--mask-variable` for secrets.

```yaml
include:
  - template: Security/SAST.gitlab-ci.yml
  - template: Security/Secret-Detection.gitlab-ci.yml
  - template: Security/Container-Scanning.gitlab-ci.yml
```

### 3.7 Branch & Merge Strategy

- `main` / `master` is always deployable. If it is broken, stop everything and fix it.
- Feature branches merge via MR only. No direct pushes to main.
- MRs require: pipeline passing, at least one approval, no unresolved threads.
- Use merge commit (not squash) for traceability on complex features; squash for trivial fixes.
- Delete merged branches automatically — stale branches are cognitive debt.

### 3.8 Caching Strategy

```yaml
# Cache Go module downloads, not compiled binaries (those are arch-specific).
# Key includes lockfile hash so cache invalidates on dependency change.
cache:
  key:
    files:
      - go.sum
  paths:
    - .go/pkg/mod/
  policy: pull-push   # pull at start, push at end of job
```

---

## 4. Docker

### 4.1 Multi-Stage Builds — Always

```dockerfile
# Stage 1: Build
FROM golang:1.23.4-alpine3.20 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download            # separate layer — cache-friendly
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o /bin/service ./cmd/service

# Stage 2: Runtime — minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
# Use distroless/static for pure Go binaries (no libc needed if CGO_ENABLED=0)
COPY --from=builder /bin/service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
```

**Tradeoff — distroless vs alpine runtime:**
- `distroless`: smallest attack surface, no shell, no package manager — production ideal.
- `alpine`: has `sh` and `apk` — useful for debugging but increases attack surface.
- Use distroless in production; use alpine-based debug images for troubleshooting (never
  leave debug images running in prod).

### 4.2 Non-Root by Default

```dockerfile
# If not using distroless/nonroot, create a user explicitly.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
```

Running as root in a container grants unnecessary privilege. If your app needs port 80/443,
use `CAP_NET_BIND_SERVICE` rather than running as root.

### 4.3 .dockerignore

```
.git
**/.gitignore
**/*.md
**/testdata
**/*_test.go
.env
.env.*
vendor/        # include only if not using -mod=vendor in build
```

A missing `.dockerignore` sends your entire `.git` directory and test files into the build
context — dramatically slowing builds and leaking unnecessary data.

### 4.4 Image Hygiene

- **Pin base image digests in production, not just tags.**
  Tags are mutable. Digests are immutable.
  ```dockerfile
  FROM golang:1.23.4-alpine3.20@sha256:<digest>
  ```
- Never `apt-get update` without `apt-get install` in the same `RUN` layer (cache invalidation
  trap).
- Combine `RUN` commands to reduce layers, but only when they are logically related.
- Never copy secrets into image layers. Use build args only for non-secret metadata. Use
  runtime secrets (mounted volumes, env from secrets manager) for credentials.

### 4.5 Health Checks

```dockerfile
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
  CMD ["/service", "-healthcheck"] 
  # Or: CMD wget -qO- http://localhost:8080/healthz || exit 1
```

Without a `HEALTHCHECK`, Docker and K8s have no application-level health signal — they only
know if the process is alive, not if it is serving traffic correctly.

### 4.6 Build Arguments & Labels

```dockerfile
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${GIT_COMMIT}"
LABEL org.opencontainers.image.created="${BUILD_TIME}"
LABEL org.opencontainers.image.source="https://gitlab.example.com/org/repo"
```

OCI labels enable traceability: which commit produced this image, when, and from where.
These labels are queryable in production registries and essential during incident response.

---

## 5. Kubernetes

### 5.1 Resource Requests and Limits — Always Set

```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m"        # Tradeoff: CPU limits throttle; memory limits OOM-kill.
    memory: "512Mi"    # Set memory limit == request for guaranteed QoS class.
```

**Tradeoff — CPU limits:** CPU throttling from limits can cause latency spikes even when nodes
have free CPU. For latency-sensitive services, consider omitting CPU limits and relying on
requests for scheduling, while setting `LimitRange` at namespace level to prevent runaway usage.

**Memory:** Always set. OOM without a limit kills the node's kubelet under memory pressure.
With a limit, only your pod is OOM-killed and restarted.

### 5.2 Probes — Distinguish Liveness from Readiness

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 15
  periodSeconds: 20
  failureThreshold: 3
  # Liveness: "is the process alive?" — failure triggers pod restart.
  # Only fail liveness on unrecoverable states (deadlock, memory corruption).

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
  # Readiness: "is the pod ready to serve traffic?" — failure removes from service endpoints.
  # Fail readiness during startup, during migrations, or when upstream deps are down.

startupProbe:
  httpGet:
    path: /healthz
    port: 8080
  failureThreshold: 30
  periodSeconds: 5
  # Startup probe prevents liveness from killing a slow-starting container.
  # Use for services with variable startup time (JVM warmup, DB migrations, etc.).
```

**Common mistake:** Using the same endpoint for liveness and readiness, or making liveness
check upstream dependencies. If your database is down, your pod should fail readiness (stop
receiving traffic) but NOT liveness (do not restart — restarting won't fix a broken database).

### 5.3 Pod Disruption Budgets

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api-pdb
spec:
  minAvailable: 2    # Or maxUnavailable: 1 — at least 2 pods always up during disruptions.
  selector:
    matchLabels:
      app: api
```

Without a PDB, a node drain during maintenance can take down all replicas simultaneously.
A PDB is a 5-line YAML that prevents a class of outages — there is no excuse not to have it.

### 5.4 Deployment Strategy

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1          # Allow one extra pod during rollout
    maxUnavailable: 0    # Never go below desired replica count during rollout
```

**Tradeoff — RollingUpdate vs Recreate:**
- `RollingUpdate`: zero downtime, but two versions run simultaneously — only use if your app
  is backward compatible with existing schema/API versions.
- `Recreate`: full downtime, but clean cutover — use for stateful apps or schema-breaking
  deploys.

### 5.5 RBAC — Least Privilege

```yaml
# Service accounts must be scoped to exactly what the workload needs.
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: configmap-reader
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "watch", "list"]
    resourceNames: ["app-config"]   # Scope to specific resources, not the whole type.
```

- Every pod must have a dedicated ServiceAccount. Never use `default`.
- Never grant `cluster-admin` to application workloads.
- Use `automountServiceAccountToken: false` on pods that do not need K8s API access.

### 5.6 ConfigMaps and Secrets

- **ConfigMaps:** Non-sensitive configuration (feature flags, endpoints, tunables).
- **Secrets:** Anything sensitive. But K8s Secrets are base64-encoded, not encrypted, by
  default. Enable encryption at rest in etcd (`EncryptionConfiguration`).
- Mount secrets as files, not environment variables where possible — environment variables
  can be accidentally leaked in logs, crash dumps, and child processes.
- Use ExternalSecrets operator + Vault/AWS SM to avoid storing secret values in Git at all.

### 5.7 Namespace Strategy

```
namespaces:
  - infra           # ingress, cert-manager, monitoring
  - services-dev    # development environment workloads
  - services-staging
  - services-prod
```

- Apply `ResourceQuota` and `LimitRange` per namespace to enforce resource governance.
- Apply `NetworkPolicy` to restrict cross-namespace traffic to only what is explicitly required.
- Label namespaces for policy enforcement (`environment: prod`, `team: payments`).

### 5.8 HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  minReplicas: 2
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60   # Not 80 — leave headroom for spikes.
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second   # Custom metric via Prometheus Adapter
        target:
          type: AverageValue
          averageValue: "1000"
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300  # Don't scale down too aggressively.
```

**Tradeoff — CPU-based vs custom metric HPA:** CPU is a proxy for load. Custom metrics
(RPS, queue depth) are more direct but require additional infrastructure (Prometheus Adapter,
KEDA). Use CPU for simplicity; use custom metrics when CPU is not correlated to actual load
(e.g., I/O-bound services).

### 5.9 Anti-Affinity

```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: api
        topologyKey: "kubernetes.io/hostname"
# Forces pod replicas onto different nodes. A single node failure cannot take down the service.
```

### 5.10 Helm Chart Conventions

- Values files per environment: `values.yaml` (defaults), `values-prod.yaml` (overrides).
- Never hard-code resource names — use `{{ include "chart.fullname" . }}` helpers.
- Use `helm diff` in CI before applying changes to production.
- Lock chart dependencies to exact versions in `Chart.lock`.
- Lint charts in CI: `helm lint`, `helm template | kubeval` (or `kubeconform`).

---

## 6. Automation & Scripting

### 6.1 Automation Principles

- **Idempotency first.** Every automation script must produce the same result whether run
  once or ten times. Use `--create-or-update` semantics, not `--create`.
- **Dry-run mode is mandatory.** Every automation that mutates state must support `--dry-run`
  that shows what would change without changing it. This enables safe review and testing.
- **Fail loudly on unexpected state.** Automation that silently succeeds on wrong assumptions
  causes incidents that are extremely hard to debug. Assert preconditions explicitly.
- **Log every action.** Structured logs with timestamps for every mutation. This is your audit
  trail. In regulated industries (payments, financial services), this is not optional.
- **Timeouts and deadlines on everything.** A hung automation job blocking a deployment pipeline
  costs more than a failed one.

```bash
#!/usr/bin/env bash
set -euo pipefail   # -e: exit on error, -u: undefined vars are errors, -o pipefail: pipe errors propagate

DRY_RUN="${DRY_RUN:-false}"

run_or_print() {
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY-RUN] Would run: $*"
    else
        echo "[EXEC] Running: $*"
        "$@"
    fi
}
```

### 6.2 Makefile as Developer Interface

- `make` is the universal entrypoint. All common operations (build, test, lint, run, docker-build)
  must have a Makefile target.
- Makefile targets must be self-documenting via `## comments`:

```makefile
.PHONY: help build test lint docker-build

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the service binary
	go build -ldflags="-s -w" -o bin/service ./cmd/service

test: ## Run unit tests with race detector
	go test -race -count=1 ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

docker-build: ## Build Docker image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .
```

### 6.3 Dependency Management in Automation

- Pin ALL tool versions used in automation scripts (golangci-lint, helm, kubectl, terraform).
  Tool version drift between developer machines and CI causes "works on my machine" incidents.
- Use `.tool-versions` (asdf) or a `tools/` directory with a version-pinning manifest.
- Install tools in CI from checksummed binaries or locked package managers — not `curl | sh`.

---

## 7. Agentification — AI Agent Patterns

### 7.1 Agent Design Principles

- **Agents are software systems.** They must be designed with the same rigour as any other
  distributed system: error handling, observability, retry logic, and graceful degradation.
- **Bound every agent action.** An agent with unbounded permissions is a security incident
  waiting to happen. Define explicit allowed actions (tools/functions) per agent, per context.
- **Human-in-the-loop checkpoints are non-negotiable for irreversible actions.** An agent that
  can delete data, send emails, or make purchases must pause and confirm before executing.
  Classify every tool as reversible or irreversible — apply approval gates to irreversible ones.
- **Observability is 10x more important for agents than for regular code.** Agent reasoning is
  opaque. Every tool call, every LLM invocation, and every decision branch must be logged with
  input, output, latency, and cost.

### 7.2 Tool / Function Design

```go
// Tool signatures must be precise, typed, and minimal.
// Overly broad tools ("do anything with the filesystem") create unpredictable agent behaviour.
type Tool struct {
    Name        string            // Unique, verb-noun format: "search_payments", "create_order"
    Description string            // LLM-visible. Be precise. Ambiguity causes misuse.
    Parameters  map[string]Param  // Typed, validated on ingress before execution.
    Execute     func(ctx context.Context, params map[string]any) (ToolResult, error)
}

type ToolResult struct {
    Content   string // Human/LLM-readable output
    IsError   bool
    Metadata  map[string]any // For structured downstream processing
}
```

- Tool descriptions are prompts. Write them as precisely as you would write an API contract.
- Validate all tool inputs before execution — never trust the LLM's parameter generation blindly.
- Tools must return structured errors, not panic. The agent runtime must be able to decide
  whether to retry, fallback, or escalate.

### 7.3 Context Window Management

- Agents lose older context as conversations grow. Design for context window pressure:
  - Summarise completed sub-tasks before adding new ones.
  - Store intermediate results externally (Redis, DB) and reference them by ID rather than
    embedding full content in the prompt.
  - Separate long-lived knowledge (system prompt, tool definitions) from ephemeral task state.
- Track token consumption per agent run. Alert when approaching limits.

### 7.4 Retry, Backoff, and Fallback

```go
// LLM calls are non-deterministic and rate-limited. Always wrap with retry logic.
func callLLMWithRetry(ctx context.Context, req LLMRequest) (LLMResponse, error) {
    backoff := exponentialBackoff{
        InitialDelay: 500 * time.Millisecond,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
        Jitter:       true,
    }
    for attempt := 0; attempt < 5; attempt++ {
        resp, err := llmClient.Call(ctx, req)
        if err == nil {
            return resp, nil
        }
        if isNonRetryable(err) { // e.g., context cancelled, content policy violation
            return LLMResponse{}, err
        }
        select {
        case <-ctx.Done():
            return LLMResponse{}, ctx.Err()
        case <-time.After(backoff.Next(attempt)):
        }
    }
    return LLMResponse{}, ErrLLMMaxRetries
}
```

### 7.5 Structured Outputs

- Always request structured output (JSON schema, function calling) when the agent result will
  be consumed programmatically. Do not parse free-form text.
- Validate structured outputs against a schema before acting on them. LLMs can hallucinate
  field names, types, and values.
- Design schemas defensively: include required and optional fields, set explicit constraints
  (enums, patterns, ranges), and handle partial/malformed responses gracefully.

```go
// Validate LLM-generated JSON against your schema before use.
func validateAgentOutput(raw []byte, schema JSONSchema) error {
    var output map[string]any
    if err := json.Unmarshal(raw, &output); err != nil {
        return fmt.Errorf("invalid JSON from LLM: %w", err)
    }
    if err := schema.Validate(output); err != nil {
        return fmt.Errorf("schema validation failed: %w", err)
    }
    return nil
}
```

### 7.6 Agent Observability

Every agent run must emit:
- **Trace:** Full call chain across LLM calls, tool calls, and external API calls — linked
  by a single `agent_run_id`.
- **Cost:** Token usage (input + output) per LLM call, aggregated per run. Alert on anomalous
  cost — runaway agents are expensive.
- **Tool call log:** tool name, input parameters (redacted for secrets/PII), output summary,
  duration, success/failure.
- **Decision log:** At key decision points, log the reasoning step the agent took and why.
  This enables post-hoc debugging of incorrect agent behaviour.

```go
slog.Info("agent tool call",
    "agent_run_id", runID,
    "tool", toolName,
    "input_summary", summarise(params),   // never log raw secrets
    "duration_ms", duration.Milliseconds(),
    "success", err == nil,
    "tokens_used", tokenCount,
)
```

### 7.7 Safety and Guardrails

- **Input validation:** Validate and sanitise all inputs before injecting into prompts.
  Prompt injection via user-controlled data is a real attack vector.
- **Output validation:** Check agent outputs for harmful content, PII leakage, and policy
  violations before surfacing to users or executing as actions.
- **Rate limiting per agent run:** Cap the number of LLM calls, tool calls, and total cost
  per single agent invocation. A runaway agent loop must be automatically terminated.
- **Scope isolation:** Agent access to tools and data must be scoped to the authenticated
  user's permissions. An agent must never be able to access another user's data.

### 7.8 Agent Testing

- Unit test each tool independently — tool correctness is testable without an LLM.
- Integration test agent workflows with deterministic LLM stubs (pre-recorded responses)
  for regression coverage.
- Evaluate agent reasoning quality with an eval harness: predefined tasks, expected outcomes,
  and automated scoring. Evals catch capability regressions introduced by prompt or model changes.
- Chaos test agents: inject tool failures, timeouts, and malformed LLM responses. Verify
  the agent degrades gracefully and does not take unexpected actions.

---

## 8. Git — Commit and Workflow Standards

### 8.1 Commit Messages

Follow Conventional Commits:

```
<type>(<scope>): <short summary>

<body — what and why, not how>

<footer — breaking changes, issue references>
```

Types: `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `ci`, `chore`, `revert`.

```
feat(payment): add idempotency key validation on retry

Without idempotency validation, concurrent retries could result in
double-charge if the upstream PSP returns an ambiguous timeout.

This validates the idempotency key against Redis before forwarding
to PSP, with a 5-minute TTL matching the PSP's deduplication window.

Closes #142
BREAKING CHANGE: PaymentRequest now requires IdempotencyKey field.
```

### 8.2 Branch Naming

```
feature/payment-idempotency-keys
fix/goroutine-leak-in-broadcast
perf/reduce-allocations-in-hot-path
ci/add-govulncheck-job
chore/upgrade-go-1-23
```

### 8.3 Code Review Standards

- Reviewers check: correctness, error handling, observability, test coverage, and security.
- Reviewers do NOT block on style — that is what linters are for.
- Review comments must be actionable. "This is wrong" is not a comment. "This will panic
  on nil pointer at line 42 if X is nil — consider a nil check or a documented precondition"
  is a comment.
- Authors respond to every comment, even if only to explain why they disagree.

---

## 9. Security — Cross-Cutting

- **OWASP Top 10** applies to your service APIs. Validate every external input. Reject early.
- **Dependency supply chain:** Only use dependencies from known maintainers. Run `govulncheck`
  in CI. Pin transitive dependencies via `go.sum`.
- **Container image supply chain:** Sign images with Cosign. Verify signatures before deployment.
- **mTLS between services** in production. Plain HTTP between pods is a data exfiltration risk.
- **Network policies:** Default-deny all ingress/egress at namespace level. Explicitly allow
  only required communication paths.
- **Audit logs:** All admin actions and all payment/sensitive operations must produce tamper-evident
  audit log entries. These are not the same as application logs.
- **Vulnerability disclosure SLA:** P0 (critical CVE in production) = patch within 24 hours.
  P1 (high) = patch within 7 days. These are commitments, not aspirations.

---

## 10. What Copilot Must NOT Do

- Do not generate code that swallows errors (`err != nil` blocks with just `return` or no `return`).
- Do not generate goroutines without a documented exit path.
- Do not use `latest` for Docker image tags, Helm chart dependencies, or GitLab CI images.
- Do not add hardcoded secrets, credentials, tokens, or API keys anywhere.
- Do not generate `kubectl apply -f -` in pipelines without a preceding `kubectl diff` or `--dry-run`.
- Do not generate Kubernetes manifests without resource `requests` and `limits`.
- Do not generate agent tools without input validation.
- Do not suggest global mutable state in Go packages.
- Do not generate pipelines that deploy to production without an explicit manual gate.
- Do not add dependencies without checking the module's maintenance status and license.
