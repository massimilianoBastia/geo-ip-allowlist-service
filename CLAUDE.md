# geo-ip-allowlist-service

Go service that answers one question: is this IP address inside one of the
allowed countries? Used by the API gateway to restrict UI logins by country.
The design is in `docs/design/` and the contracts in `api/`. If code and design disagree, the design wins: stop and ask.

## Working rules

- Don't make things up. If something is unclear while coding, stop and ask me
  before going on. Discuss design and library choices with me first.
- Do one plan step at a time, in the order of `docs/design/plan.md`.
- Never change or delete an existing test to make it pass. If a test looks
  wrong, tell me.
- Don't add features or files that the plan doesn't ask for.
- If you test the component follow strictly the workflow defined in the README.md either by building the binary or by installing the local cluster

## Go style

- Follow Effective Go (https://go.dev/doc/effective_go), the language spec
  (https://go.dev/ref/spec) and the code layout guide (https://go.dev/doc/code).
  Keep one style across the whole codebase.
- Standard library first. No third-party package without my approval; say why
  it is needed and what the alternative is.
- Small functions with a clear name that each do one short thing. Don't repeat
  yourself.
- Parts are independent and well encapsulated: depend on small interfaces and
  inject dependencies (constructors take them), so each part is testable alone.
- Errors: return them, wrap with `fmt.Errorf("...: %w", err)`, handle each once.
  No panics for expected failures.
- Pass `context.Context` as the first argument where work can block or cancel.
- Logging uses the standard `log/slog`, structured, with no secrets or full
  customer data in log lines.
- Comment exported names and anything non-obvious; no noise comments.
- For implementing the API-server for code organizations and project structure follow this layout https://go.dev/doc/modules/layout#server-project 

## Tests

- Tests are committed after the skeleton and before the implementation.
- Table-driven tests, with fakes for injected dependencies. No network in any
  test, and never the real GeoLite2 file.
- The committed synthetic database `testdata/GeoIP2-Country-Test.mmdb` (MaxMind
  test data, see `testdata/README.md`) is allowed and expected in the lookup
  and acceptance tests. Do not fake the lookup where the acceptance table
  needs that file; use fakes only for pure logic tests.
- Every edge case in the plan has a test.
- Before the tests, write the empty skeleton they need, in a separate commit:
  the packages, types and function signatures, with no logic. Bodies return a
  "not implemented" error or panic; where a value must be returned, return a
  deliberately wrong one, so a stub can never pass a test by accident. The
  tests must compile and then fail on their assertions (red), never on a
  missing symbol. Run them and confirm each one fails before implementing.
- Use a small interface only for a dependency that crosses a boundary (the
  database lookup, the clock, the logger) so tests can inject fakes. Pure
  functions stay plain functions.
- Order for each step: skeleton, then failing tests, then the implementation,
  each as its own commit. The implementation commit does not change tests,
  unless the developer approves it and the reason is recorded in
  `docs/ai-usage.md`.

## Before finishing any step

```
gofmt -l .
go vet ./...
go test -race -cover ./...
```

Report what passed and what didn't.

## Data and secrets

- `GeoLite2-*.mmdb` is never committed (MaxMind licence). The service never
  downloads it and never touches the licence key: a separate update container
  (a sidecar) downloads it into a shared volume, and the service only reads
  the file from there, checks it, and swaps it in (`docs/api-design-notes.md`,
  D11 and J7).
- The MaxMind account id and licence key are mounted as read-only files into
  the update container only: Docker Compose `secrets:` (at
  `/run/secrets/<name>`) and, in Kubernetes, a `Secret` used as files with
  `readOnly: true`. They are read through the update tool's `_FILE` variables
  (`GEOIPUPDATE_ACCOUNT_ID_FILE`, `GEOIPUPDATE_LICENSE_KEY_FILE`). These
  variables hold file paths, not secrets: the secret is inside the file.
- Never put them in environment values, in the image (no `COPY`, `ADD`, `ARG`
  or `ENV` for credentials), or in the repository. If a build ever needs a
  secret, use `docker buildx build --secret` with `RUN --mount=type=secret`.
  Only `secrets/*.example` placeholders are committed.
- No keys, tokens or personal paths in code, config or logs.

## Git

- One commit per finished step, with a short subject line. Commit tests before
  the implementation that satisfies them.
