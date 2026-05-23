# Architecture website

A self-contained visual reference for the CI/CD architecture in this repo.
Walks through the big picture, git workflow, pipeline stages, service design,
gateway layer, GitOps flow, secrets management, team norms, and migration plan.

## How to view

Three options:

### Option 1 — open the file directly

Double-click `website/index.html`. Opens in your default browser.
No server needed. All assets load from CDN or local relative paths.

### Option 2 — local web server (recommended for development)

```sh
# From the repo root:
cd website
python3 -m http.server 8000
# or: npx serve .

# Then open http://localhost:8000
```

### Option 3 — VS Code Live Server extension

If you have the "Live Server" extension installed:
right-click `website/index.html` → "Open with Live Server".

## What's inside

| File | Purpose |
|---|---|
| `index.html` | The full page — all content sections, embedded SVG diagrams |
| `assets/css/styles.css` | Custom styles beyond Tailwind utility classes |
| `assets/js/main.js` | Interactivity: pipeline tabs, sidebar active state, keyboard shortcuts |

## Keyboard shortcuts

While viewing the page:

- `1` through `8` — jump between pipeline stage details
- Sidebar links — click to jump to any section
- Browser back/forward — works as expected with the anchor links

## How the sections map to the repo

| Section | Files explained |
|---|---|
| 00 — The big picture | The whole repo, visualised |
| 01 — Git workflow | `CONTRIBUTING.md`, `.commitlintrc.yml`, `ci-templates/templates/go-service.yml` |
| 02 — The pipeline | `ci-templates/templates/go-service.yml` |
| 03 — The service | `service/` directory |
| 04 — Gateway layer | `gateway/`, `docker-compose.yml` |
| 05 — GitOps deploy | `gitops/argocd/`, `charts/`, `environments/` |
| 06 — Secrets | `gitops/external-secrets/`, `docs/secrets-comparison.md` |
| 07 — Team norms | `CONTRIBUTING.md`, `.gitlab/CODEOWNERS`, MR template |
| 08 — Migration plan | `docs/adr/0001-cicd-migration-plan.md` |
| 09 — File reference | Index of every file in the repo |

## Why a website instead of more markdown?

Markdown is good for text. Architecture is also flow, hierarchy, and connection.
SVG diagrams + clickable pipeline stages + side-by-side comparisons communicate
those better than monospace boxes in a README. The visual layer doesn't replace
the source files — it points at them and explains how they fit together.

## Tech choices

- **Tailwind CDN** — no build step, no Node deps to maintain. The whole site
  is one HTML file + small CSS + small JS.
- **Inline SVG** — diagrams are version-controlled like code (not image binaries)
  and scale crisply at any size.
- **Plain ES6 JS** — no framework. Two functions, ~80 lines total.

If this site grows beyond a single page, the right next step is a static site
generator (Hugo, Eleventy, Astro). Until then, one HTML file is the right
amount of complexity.

## Updating the site when the repo changes

The site is hand-maintained — it doesn't auto-generate from the YAML/code.
That's intentional: prose explanations of why something exists don't come
from the file structure. When the pipeline gains a stage or a major
architectural decision changes:

1. Update the relevant section in `index.html`
2. If a new file was added, add a row to section 09 (file reference)
3. If a new ADR was written, link it from the appropriate section
4. Commit alongside the change it documents (same MR, not separate)
