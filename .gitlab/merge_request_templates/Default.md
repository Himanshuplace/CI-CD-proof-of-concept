## What & why
<!-- One paragraph: what changed, and why now. Skip if the title is self-explanatory. -->


## How tested
<!-- One line for trivial MRs. For bigger changes: which scenarios you verified manually. -->


## Risk
<!-- Pick one and delete the others. The reviewer reads this first. -->
- Low — pure refactor / docs / config
- Medium — touches one service's behavior
- High — touches auth, payment, depository adapters, or DB schema

## Rollback
<!-- For Medium/High only. How would you undo this if it breaks production? -->


---
<!--
Pre-merge — you (the author) confirmed these before requesting review:
- tests pass locally (go test -race ./...)
- no secrets in the diff
- no debug logs / fmt.Println left in code
- if it's user-facing risky behavior: there's a feature flag wrapping it
-->
