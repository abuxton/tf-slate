# Release policy

Only the repository owner (`abuxton`) may create, update, merge or release from a `release/**` branch.

| Control | Where | Effect |
|---------|-------|--------|
| `release/**` ruleset | Repo rulesets | Creation, update, deletion and force-push restricted to the repository admin role (bypass); PR required; `release-security` check required |
| `release` environment | Repo environments | Required reviewer `abuxton`; deployments limited to `main` |
| Workflow guard | `.github/workflows/release.yml` | Runs only if merged by the repo owner, from a same-repo branch named `release/vX.Y.Z` |
| CODEOWNERS | `.github/CODEOWNERS` | Owner review for workflows, GoReleaser, formula and scripts |
| Security scan | `.github/workflows/release-security.yml` | govulncheck, gitleaks, actionlint and SHA-pin check on every PR to `main` |

Notes:

- Personal repositories cannot restrict merging to a named user. The workflow guard and the environment approval enforce it: a release PR merged by anyone else publishes nothing.
- Actions are pinned to commit SHAs (Dependabot keeps them current) and GoReleaser to an exact version.
- Enable Settings → Actions → General → "Allow GitHub Actions to create and approve pull requests" for the formula PR job.
