import os
import subprocess

import anthropic
import gitlab


def env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"missing required env var: {name}")
    return value


base = env("CI_MERGE_REQUEST_DIFF_BASE_SHA")
diff = subprocess.check_output(["git", "diff", base, "HEAD"], text=True)
diff = diff[:50000]

client = anthropic.Anthropic(api_key=env("ANTHROPIC_API_KEY"))
review = client.messages.create(
    model=os.environ.get("ANTHROPIC_MODEL", "claude-sonnet-4-20250514"),
    max_tokens=2500,
    system=(
        "You are a senior Go and DevOps reviewer for a financial platform. "
        "Prioritize correctness, security, observability, idempotency, rollback safety, "
        "context propagation, SQL/tenant isolation, and production failure modes. "
        "Be concise. Separate blocking findings from advisory suggestions."
    ),
    messages=[{"role": "user", "content": f"Review this GitLab MR diff:\n\n{diff}"}],
)

gl = gitlab.Gitlab(env("CI_SERVER_URL"), private_token=env("GITLAB_TOKEN"))
project = gl.projects.get(env("CI_PROJECT_ID"))
mr = project.mergerequests.get(env("CI_MERGE_REQUEST_IID"))
mr.notes.create({"body": "## AI Code Review\n\n" + review.content[0].text})

