# Secrets Management: Two Approaches

## The Comparison

| Property | GitLab CI Variables | HashiCorp Vault |
|---|---|---|
| Setup time | 5 minutes | 2-4 hours (first time) |
| Infrastructure | None (built into GitLab) | Vault server (can run in-cluster) |
| Access control | GitLab project roles | Vault policies (fine-grained) |
| Audit log | No (who read what secret, when?) | Yes — every read is logged |
| Secret rotation | Manual (go to UI, change variable) | Automatic (Vault dynamic secrets) |
| Dynamic secrets | No | Yes (Vault generates DB passwords per-request) |
| Secret in K8s pod | Set as env var in Helm values | External Secrets Operator syncs to K8s Secret |
| Complexity | Low | High |
| Cost | Free (included in GitLab) | Open source but needs VM/cluster resources |

## When to use which

**Use GitLab CI variables for:**
- Non-sensitive config: STAGING_URL, COVERAGE_THRESHOLD
- Webhook URLs: SLACK_WEBHOOK_URL, DORA_WEBHOOK_URL
- API keys that change rarely and don't need per-access audit logs
- External tool tokens: ANTHROPIC_API_KEY, GITOPS_DEPLOY_TOKEN

**Use Vault for:**
- Database passwords (especially with dynamic secrets — Vault generates a new
  password per deployment, auto-expires after the pod dies)
- Service-to-service authentication tokens
- Cryptographic keys (NSDL/CDSL signing keys in a financial context)
- Anything where "who accessed this secret, when?" is a compliance question

## Approach 1: GitLab CI Variables

### What variables you need to add (Settings → CI/CD → Variables)

| Variable | Where used | Masked | Protected |
|---|---|---|---|
| `STAGING_URL` | smoke-test-staging | No | No |
| `GITOPS_DEPLOY_TOKEN` | deploy-staging, deploy-dev | Yes | Yes |
| `GITOPS_REPO_URL` | all deploy jobs | No | No |
| `DORA_WEBHOOK_URL` | track-dora | No | No |
| `SLACK_WEBHOOK_URL` | notify-deploy, notify-failure | Yes | No |
| `ANTHROPIC_API_KEY` | ai-review | Yes | No |
| `GITLAB_TOKEN` | ai-review (post MR comment) | Yes | Yes |

**Masked**: GitLab replaces the value with `[MASKED]` in CI logs. Use for any secret value.
**Protected**: Variable is only available in protected branches (main) and tags. Use for deploy tokens.

### How the pipeline uses them

```yaml
# In your job — reference the variable name directly
script:
  - git clone "https://deploy-bot:${GITOPS_DEPLOY_TOKEN}@${GITOPS_REPO_URL}" gitops-config

# GitLab injects these as environment variables before running the script.
# No additional configuration needed in the pipeline YAML.
```

### How to pass secrets to Kubernetes pods (GitLab CI approach)

```yaml
# Helm values file (environments/staging/services/example-go-service/values.yaml)
# This bakes the secret into Helm values — NOT recommended for sensitive secrets.
# Use this ONLY for non-sensitive config.
env:
  LOG_LEVEL: info

# For actual secrets: use Kubernetes Secrets created manually or via External Secrets.
# Example: create a K8s secret manually (one-time setup):
kubectl create secret generic example-go-service \
  --from-literal=DATABASE_URL="postgres://..." \
  --from-literal=REDIS_URL="redis://..." \
  -n staging
```

---

## Approach 2: HashiCorp Vault + External Secrets Operator

### Architecture

```
GitLab CI pipeline
    │ (1) JWT token from GitLab OIDC
    ▼
Vault server (JWT auth backend)
    │ (2) validates token → short-lived Vault token
    ▼
Vault secrets (secret/gnexlayer/staging/*)
    
External Secrets Operator (runs in K8s cluster)
    │ (3) reads from Vault using a service account bound Vault role
    ▼
Kubernetes Secret (created/synced automatically)
    │
    ▼
Pod environment variable (mounted by the Helm chart)
```

### Flow explanation

Step 1: Your CI pipeline authenticates to Vault using a JWT token that GitLab
generates for the pipeline run. The token proves "this is GitLab CI running
for project gnexlayer/paymentcraft on branch main."

Step 2: Vault validates the JWT token against GitLab's JWKS endpoint, checks
that the claims (project_path, ref) match the policy. If valid, issues a
short-lived Vault token (TTL: 5 minutes). The pipeline uses this token to
read secrets — it expires before anyone could steal and reuse it.

Step 3: External Secrets Operator (ESO) runs as a Kubernetes controller. It
reads ExternalSecret objects, fetches the actual secret from Vault, and creates
or updates a Kubernetes Secret. The pod then reads from the Kubernetes Secret
as an environment variable — the pod never talks to Vault directly.

### Vault policy for CI (what the CI runner is allowed to read)

```hcl
# vault-policy-ci.hcl
# CI runner can read staging secrets but NOT production.
# Production secrets are only readable by ESO's service account (in-cluster).

path "secret/data/gnexlayer/staging/*" {
  capabilities = ["read"]
}

# CI runner can write SBOM attestations but NOT read them.
path "secret/data/gnexlayer/staging/sbom-*" {
  capabilities = ["create", "update"]
}
```

### Pipeline job: reading from Vault in CI

```yaml
# Add to your pipeline for secrets that must be available DURING the build
# (e.g., a registry token, an external API key for integration tests).

fetch-secrets:
  stage: validate
  image: vault:1.16
  script:
    - |
      # Authenticate using GitLab's OIDC token
      VAULT_TOKEN=$(vault write -field=token auth/jwt/login \
        role=gitlab-ci-staging \
        jwt="$CI_JOB_JWT_V2")
      
      export VAULT_TOKEN
      
      # Read the secret and export as env var for subsequent jobs
      export DATABASE_URL=$(vault kv get -field=url secret/gnexlayer/staging/postgres)
      
      # Write to a dotenv artifact so other jobs can use it
      echo "DATABASE_URL=$DATABASE_URL" >> staging.env
  artifacts:
    reports:
      dotenv: staging.env  # GitLab injects these as env vars in downstream jobs
  rules:
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
```

### The existing External Secrets files in this repo

#### gitops/external-secrets/vault-cluster-secret-store.yaml

This tells ESO "here is how to talk to Vault." It's cluster-scoped (one
per cluster, not per namespace). ESO uses a Kubernetes ServiceAccount that
has a Vault role bound to it.

```yaml
# Already exists — explains the fields:
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: vault-cluster-store
spec:
  provider:
    vault:
      server: "https://vault.gnexlayer.internal"  # your Vault address
      path: "secret"                               # KV v2 mount path
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"                  # Vault's K8s auth mount
          role: "external-secrets"                 # Vault role for ESO
          serviceAccountRef:
            name: "external-secrets-sa"            # K8s SA bound to Vault role
```

#### gitops/external-secrets/example-go-service.yaml

This tells ESO "read these specific keys from Vault and create a K8s Secret."
One ExternalSecret per service per environment.

```yaml
# Already exists — explains the fields:
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: example-go-service
  namespace: staging
spec:
  refreshInterval: 1h           # ESO re-reads from Vault every hour
  secretStoreRef:
    name: vault-cluster-store
    kind: ClusterSecretStore
  target:
    name: example-go-service    # name of the K8s Secret to create
    creationPolicy: Owner       # ESO owns this Secret, deletes it if ES is deleted
  data:
    - secretKey: DATABASE_URL   # key in the K8s Secret
      remoteRef:
        key: gnexlayer/staging/example-go-service   # Vault path
        property: database_url                       # field within that Vault secret
```

### How the Go pod reads the secret

```yaml
# charts/go-service/templates/deployment.yaml
# The Helm template references the K8s Secret created by ESO:
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: example-go-service   # matches ExternalSecret.spec.target.name
        key: DATABASE_URL
```

The pod never knows Vault exists. It just reads `$DATABASE_URL` like any env var.

---

## Recommendation for this project

**Start with GitLab CI variables.** Wire up SLACK_WEBHOOK_URL, GITOPS_DEPLOY_TOKEN,
and STAGING_URL. Get the pipeline running end-to-end.

**Add Vault when you have:**
- A database the pipeline needs to connect to (integration tests)
- Multiple services that share secrets (one Vault entry → all services read it)
- Compliance requirement for secret access audit logs
- Dynamic secrets needed (database passwords that auto-expire)

The files in gitops/external-secrets/ are ready to use — they just need
a Vault server at `https://vault.gnexlayer.internal` and ESO installed
in the cluster (helm install external-secrets external-secrets/external-secrets).
