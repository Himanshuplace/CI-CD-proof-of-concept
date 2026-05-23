# ADR-0001: Migrate from helm-push CI to GitOps with ArgoCD, in 4 phases

- Status: Accepted
- Date: 2026-05-23
- Deciders: <fill in>

## Context

Today: 5 backend engineers (2 also do DevOps), multiple services already
running on Kubernetes, deployed via `helm upgrade` from GitLab CI. Multiple
production deploys per day. Financial domain with compliance requirements
for SBOM, image signing, audit trail.

The current setup works but has known weaknesses:
- CI runner needs cluster credentials → if CI is compromised, the cluster is
- Rollback requires re-running CI (slow, error-prone)
- No drift detection: if someone `kubectl apply`s outside CI, nobody knows
- Audit trail for "who deployed what when" lives in CI logs, not git

We want to fix these while keeping the multi-deploy-per-day cadence and
without introducing tools the team doesn't yet operate.

## Decision

Adopt GitOps with ArgoCD as the deploy mechanism. Roll out in 4 phases:

| Phase | When | What |
|---|---|---|
| 0 | Week 1 | Team norms: lighter review process, pre-commit hooks, ADRs |
| 1 | Week 2-4 | ArgoCD service-by-service migration with dual-deploy safety net |
| 2 | Week 5-6 | Feature flags (Flagsmith self-hosted) + auto-rollback on smoke fail |
| 3 | Month 2-3 | Vault + External Secrets for secret management |
| 4 | Month 3+ | Kyverno policies (audit mode first, enforce later) |

Each phase ships value independently. Stopping after Phase 2 leaves us in a
better place than today. Phases 3 and 4 are upgrades, not requirements.

## Alternatives considered

- **Stay on helm-push from CI** (rejected): leaves the audit trail and
  rollback problems unsolved. Compliance pressure will force this change
  eventually; doing it now while team is small is cheaper.

- **Adopt all 4 tools at once (ArgoCD + Vault + ESO + Kyverno) in one
  quarter** (rejected): too many concepts for a team that's "mostly new"
  to these tools. Risk of misconfiguring something and causing a deploy
  freeze.

- **Bazel + monorepo build system** (rejected): correct theoretical answer
  for 100+ services. For 17 with a 5-person team, the maintenance burden
  exceeds the win. Revisit if we grow past 30 services.

## Consequences

**Easier:**
- Rollback = `git revert` (seconds, audit trail automatic)
- Drift detection = ArgoCD shows out-of-sync resources
- Compliance evidence = git history of every production deploy
- CI doesn't need cluster credentials (only git push permission to gitops repo)

**Harder:**
- Two repos to navigate: service repo + gitops-config repo
- Debugging "why didn't my service deploy?" now requires checking ArgoCD UI,
  not just CI logs
- Phase 1 has a 2-3 week period of running dual deploys (helm push + git
  push to gitops). More moving parts during cutover.

**New risk accepted:**
- ArgoCD becomes a critical dependency. If ArgoCD has a bug or outage, no
  new deploys go out. Mitigation: keep the helm-push path runnable as
  emergency fallback for the first 3 months.

**Revisit if:**
- ArgoCD adoption causes more incidents than helm-push did → roll back
- Team grows past 15 engineers → policy enforcement (Kyverno) becomes
  more valuable, accelerate Phase 4
- Compliance asks for per-deploy approval traceability that ArgoCD UI
  doesn't provide → add a wrapper that requires GitLab MR approval to
  trigger ArgoCD sync
