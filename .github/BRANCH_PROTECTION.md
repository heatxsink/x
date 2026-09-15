# Branch protection for `main`

`branch-protection.json` in this directory is the payload applied to the
protection API. It is a record and a rollback source, **not** something CI
applies — nothing reads it automatically, so a change here does nothing until
someone runs the command below.

## Apply

```sh
gh api -X PUT repos/heatxsink/x/branches/main/protection \
  --input .github/branch-protection.json
```

The `PUT` replaces the whole protection object, so the file has to carry every
setting, not just the one being changed. A `GET` of the same endpoint returns a
*different* shape — with `url` fields and nested `enabled` objects — and cannot
be fed back into `PUT` unedited.

## Required checks

Three fixed names, one per workflow:

| Check | Workflow | Covers |
|---|---|---|
| `CI Gate` | `test.yml` | tests, security scan, build, coverage, module discovery, every module leg |
| `Build and Verify` | `go.yml` | — |
| `golangci-lint` | `lint.yml` | — |

Only fixed names belong here. Most checks in this repo are generated —
`Run Tests (1.26.6)` carries the Go version, `Build (linux, amd64)` the
platform, and the `Test Individual Modules (...)` legs come from `go list`, so
their number changes whenever a package is added or removed. Requiring a
generated name leaves a required check that stops reporting the moment the name
moves, and every pull request then waits on it forever. `CI Gate` exists so one
stable name covers all of them; see `test.yml`.

## Settings worth understanding before changing them

- **`strict: false`** — a pull request does not have to be rebased onto the
  latest `main` before merging. With `strict: true` every pull request needs a
  rebase and a full re-run even when nothing conflicts, which serialises
  merges; dependency bumps land in batches here and already need rebasing
  whenever `go.sum` genuinely conflicts. The trade is that two independently
  green pull requests can merge into a broken combination. `main` runs the full
  suite on every push, so that surfaces after the merge rather than before.
- **`enforce_admins: true`** — admins are not exempt. Nobody merges past a red
  or pending check, which is what makes this real rather than advisory. A flaky
  check blocks the merge until it is re-run.
- **`required_approving_review_count: 0`** — no second reviewer is required,
  which suits a single-maintainer repository. Because
  `required_pull_request_reviews` is present at all, changes still have to go
  through a pull request; direct pushes to `main` are refused.

## Rollback

To drop the required checks and return to the pre-2026-09-14 state, delete the
`required_status_checks` key from the JSON and apply it again. Every other
setting here predates that change.
