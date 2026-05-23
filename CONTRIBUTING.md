# Contributing — How we ship as a team

Five people, multiple deploys per day, financial domain. These are the rules
that make that possible.

## The three non-negotiables

1. **Main must always be deployable.** Every commit on main is a potential
   production release. If you wouldn't deploy it right now, don't merge it.
2. **You break main, you fix main.** Within 30 minutes — either fix forward
   or revert. If you can't, ping the team in `#engineering` and someone else
   reverts.
3. **Every risky user-facing change ships behind a feature flag.** "Risky"
   means: changes payment logic, depository adapter behavior, auth flow, or
   anything that touches a broker integration. The flag is OFF on merge.

## The branch model

Trunk-based development. One protected `main` branch.

```
main
  ├── feature/jwt-refresh         (you, 1-2 days max)
  ├── fix/nats-reconnect-timeout  (you, hours)
  └── hotfix/payment-nil-ptr      (production incident, < 4 hours)
```

Branch name regex (enforced by CI): `^(feature|fix|hotfix|chore|refactor|docs)/[a-z0-9-]+$`

**Branch lifetime: 2 days max.** If your feature needs longer, wrap the
incomplete work in a feature flag and merge what you have. Long-lived
branches cause merge hell that gets worse the longer you wait.

## The commit model

[Conventional Commits](https://www.conventionalcommits.org/). Enforced by
`commitlint` in CI.

```
feat(payment): add NSDL mandate flow
fix(nats): handle JetStream reconnect after timeout
chore(deps): bump grpc to v1.64.0
docs(adr): record ArgoCD migration decision
```

Type matters (drives semver). Scope is optional but useful.

## The review model

1 approver required (CODEOWNERS auto-assigns on sensitive paths).

**Review SLA: 4 working hours.** If you can't review in 4 hours, reassign
to someone else or say so in the MR.

**Author responsibilities before requesting review:**
- Self-review your own diff first. Catch typos, debug logs, commented-out
  code. Don't waste a teammate's time on stuff you can see yourself.
- Pipeline is green.
- MR description follows the template — risk level set, rollback path
  documented for Medium/High risk.

**Reviewer responsibilities:**
- Focus on: correctness, hidden side effects, missing error handling,
  rollback safety. NOT formatting (gofmt does that).
- Ask questions, don't make demands. "Should this be in a transaction?"
  beats "put this in a transaction."
- Approve when ready, even if you'd have written it differently. Diff
  reviews are about catching bugs, not enforcing your style.

**MR size guideline: under 400 lines.** Larger MRs miss 50% of bugs in
review. Split it. If splitting is hard, ship behind a feature flag in
multiple MRs.

## The deploy model

```
push to feature branch       → CI runs (no deploy)
merge to main                → CI runs → auto-deploy to staging
                               → smoke test must pass
                               → DORA event recorded
git tag v1.x.y               → CI runs → updates production GitOps repo
                               → ArgoCD requires manual sync
                               → smoke test on production
                               → on failure: auto-rollback
```

**You can deploy to staging any time.** Just merge to main.

**Production deploys require a tag.** Use semver. Run `git tag v1.4.0` after
the staging deploy is healthy. The pipeline does the rest.

## When main breaks

In priority order:

1. **Revert immediately** if the fix isn't obvious. `git revert <sha>`,
   push to main, done. We can debug the original change in a follow-up MR.
2. **Fix forward** if the fix is 5 lines and you understand it. Open
   the MR, get it reviewed within minutes (Slack-ping someone), merge.
3. **Page the team** if neither option is fast. Production is at risk.

Don't leave main broken overnight. Don't go home with a red pipeline.

## Hotfix procedure

Production incident. Tested-and-shipped within 1 hour.

```bash
git checkout main && git pull
git checkout -b hotfix/<short-description>
# make minimal fix
# add a test that fails without the fix
git push origin hotfix/<short-description>
# open MR — get 1 approval (any team member, doesn't need to be CODEOWNER)
# merge to main
git tag v1.x.(y+1)
git push origin v1.x.(y+1)
# ArgoCD manual sync to prod
# verify
# write incident postmortem within 24h (docs/incidents/YYYY-MM-DD-<title>.md)
```

Hotfixes skip the normal "2 days in staging" sanity. That's the tradeoff —
you trade safety for speed. To compensate, you write the postmortem.

## Decisions that affect future-you: write an ADR

When the team decides:
- to use a new tool (e.g. "we picked KrakenD over Kong")
- to change a structural pattern (e.g. "all services use slog now")
- to deviate from a documented best practice for a good reason

Write a one-page ADR in `docs/adr/`. Template at `docs/adr/0000-template.md`.
Three months from now, when someone asks "why did we do it this way?",
the ADR is the answer.

## Tools the team uses

- **GitLab** — code, CI, MRs, issues
- **Slack** — async coordination, deploy notifications, incident response
- **ArgoCD** — production deploy gate (manual sync for prod)
- **Flagsmith** — feature flags (when it's set up in Phase 2)

## What we don't do

- Direct push to `main`. Even tech lead. Even for typos.
- Force push to shared branches.
- Deploy bypassing the pipeline (no `kubectl apply` from your laptop).
- Skip the smoke test "just this once."
- Ship secrets in code. Vault path or GitLab CI variable, never both.
- Long-running branches. If it's been open a week, you're doing it wrong.
