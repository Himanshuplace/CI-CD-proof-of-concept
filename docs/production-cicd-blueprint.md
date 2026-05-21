# Production CI/CD Blueprint for 17 Go Services

Audience: 7 backend engineers, financial application, GitLab, Docker, Kubernetes, Helm, ArgoCD, Vault.

This is the recommended target, not a buzzword stack. Keep the operational surface small, automate the repeated work, and make production changes traceable.

## Final Architecture

Use a shared CI template repo plus one GitOps config repo.

```text
service repo                         ci-templates repo                  gitops-config repo                 k8s cluster
-----------                          -----------------                  ------------------                 -----------
Go code
Dockerfile
.gitlab-ci.yml  --include--------->  templates/go-service.yml
                                      configs/.golangci.yml
                                      configs/.gitleaks.toml

CI:
validate -> test -> scan -> build image -> sign -> SBOM -> update GitOps tag
                                                                             ArgoCD watches Git
                                                                             Helm values updated  ------>  dev/staging/prod
```

CI must never run `kubectl apply` against production. CI builds and records desired state in Git. ArgoCD deploys.

## Tool Choices

| Area | Use | Why |
| --- | --- | --- |
| Source control | GitLab | One system for repo, MR, registry, CI, approvals, audit |
| Branching | Trunk-based development | Best fit for 7 people and 17 services; avoids GitFlow ceremony |
| CI | GitLab CI shared templates | One pipeline standard for all services |
| Build | Docker BuildKit | Repeatable OCI images; works with GitLab registry |
| Deploy | Helm + ArgoCD | GitOps, drift detection, easy rollback |
| Cluster | k3s first, managed K8s later | Good entry point if coming from Docker Compose |
| Secrets | Vault + External Secrets Operator | Secrets stay outside Git and CI logs |
| Security | Gitleaks, gosec, Trivy, Syft, Cosign | Covers secret, source, dependency, image, SBOM, signing |
| Policy | Kyverno first | Easier than raw OPA for small teams |
| Flags | Flagsmith self-hosted | Decouple deploy from release per broker/tenant |
| Observability | Prometheus, Grafana, Loki, Tempo, Alertmanager | Standard, open stack |
| Agentic automation | CodeRabbit or GitLab AI first; custom agent later | Advisory first, action later after trust is built |

## Branch And Release Model

Use one protected `main` branch per service.

```text
feature/GNEX-123-nsdl-mandate-flow -> MR -> main -> staging
tag v1.8.2                         -> production release candidate -> ArgoCD manual sync
```

Rules:

- Feature branches live less than 2 days.
- Every MR needs a green pipeline and at least 1 approval.
- Auth, payment, CI, Docker, Helm, Vault, and production config need 2 approvals through CODEOWNERS.
- Main auto-deploys to staging.
- Production deploys only from semver tags and requires manual approval in ArgoCD or GitLab environment approval.
- Deploy is not release. Use Flagsmith for customer/broker rollout.

## Pipeline Stages

Fast path for every MR:

```text
validate:
  commitlint, branch naming, gofmt, go vet, golangci-lint, gitleaks

test:
  unit tests with race detector, coverage gate

security:
  gosec, govulncheck, Trivy filesystem scan

build:
  Docker image build only when tests pass

package:
  image scan, SBOM, cosign signature, provenance attestation

deploy:
  update Helm value in GitOps repo for dev/staging/prod

observe:
  smoke test, DORA event, Slack notification

agentic:
  AI MR review is advisory, not a merge blocker
```

## Environment Strategy

| Environment | Trigger | Deploy | ArgoCD sync | Replicas | Secrets | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Dev | feature branch | automatic | auto + self-heal | 1 | `secret/gnexlayer/dev/*` | Optional per-service preview namespace |
| Staging | merge to `main` | automatic | auto + self-heal | 2 | `secret/gnexlayer/staging/*` | Must mirror prod config shape |
| Production | semver tag | manual approval | manual | 3+HPA | `secret/gnexlayer/prod/*` | No debug, no direct kubectl |

Do not share databases, NATS accounts, Redis databases, Vault paths, or signing credentials between environments.

## Minimum GitLab Settings

For every service repo:

- Protected `main`: no direct push, no force push, no deletion.
- Merge method: squash commits required.
- Pipelines must succeed.
- All discussions resolved.
- Reset approvals on push.
- Author self-approval disabled.
- CODEOWNERS approval required.
- Branch regex:

```regex
^(main|feature|fix|hotfix|chore|refactor|docs)/[A-Z]+-[0-9]+-[a-z0-9-]+$
```

- Commit regex:

```regex
^(feat|fix|chore|refactor|test|perf|ci|docs|build|revert)(\([a-z0-9\/-]+\))?!?: .{1,72}$
```

## Roadmap

Phase 1, this week:

- Add shared GitLab CI template.
- Enforce branch naming, commitlint, CODEOWNERS, MR template.
- Add DORA deployment webhook table or endpoint.
- Add advisory AI MR review.

Phase 2, weeks 2-4:

- Add Trivy, Gitleaks, gosec, govulncheck.
- Build and push Docker images with immutable SHA tags.
- Generate SBOM with Syft.
- Sign images with Cosign.

Phase 3, month 2:

- Stand up k3s or managed Kubernetes.
- Install ArgoCD, External Secrets Operator, cert-manager, ingress controller.
- Move staging deploy to GitOps.

Phase 4, month 3:

- Move production deploy to GitOps with manual sync.
- Add Kyverno policies for non-root, signed images, resource limits, trusted registries.
- Add Flagsmith for tenant-level rollout.

Phase 5, months 4-6:

- Add Argo Rollouts canary for high-risk services.
- Add contract tests for service APIs.
- Add Dagger after the pipeline stabilizes, so developers can run the same pipeline locally.
- Allow agentic fix MRs only for low-risk dependency bumps and generated tests at first.

## Non-Negotiables

- Never deploy by SSH.
- Never deploy `latest`.
- Never rebuild per environment. Promote the same image digest.
- Never store production secrets in GitLab CI variables. Use Vault.
- Never run production deploys from feature branches.
- Never allow CI to mutate the cluster directly.
- Never make AI fixes auto-merge in a financial system. Agent opens MR; human approves.

