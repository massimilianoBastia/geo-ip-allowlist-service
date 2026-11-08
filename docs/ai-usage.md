# Use of AI

Claude Code (Claude Sonnet 5.5) was the assistant for the design, the contract, the tests and the code, as the challenge allows. The design decisions were made by the developer, with the AI as an adviser that proposed options and checked claims against sources. This page says what was done, how, and where the commit history does not show it.

## Time

- **Roughly 4.3 hours of work in this repository, estimated from memory and not tracked.** This is slightly above the 4 hours the challenge suggests. The earlier-repository design work is not counted.
- The GitHub repository was created at 11:44 on 2026-11-08. The first commit is at 15:49 and the last code commit at 20:54. Documentation commits continue until 21:29. The elapsed time is longer than the hours of work, and the breaks were not recorded.
- The design was first worked out in an **earlier repository** (the design phase), before the first commit here. This repository is a second run of the same workflow. That earlier work is not counted in the 4.3 hours or in the first commit's timestamp, and its session is not published here.

## Order of work and what the commits show

- The design notes and the contract (`api/openapi.yaml`) were written first. They were committed late (`a42aa2d`, 16:45), after the first code commits, so the commit order does not show the real order of work.
- **Tests first was followed only for S-007 and S-008** (`bb32bfc` skeleton, `11fba8f` failing tests, `cbfee78` implementation). The three commits were made within a few minutes of each other, not as a live red-green record. S-005 and S-006 tests were committed with the test data (`fb3d8af`), without their own skeleton commit.
- **From S-009 onward, tests and code were written in the same Claude Code session** (`ai-log/04-coding`): the prompt asked the AI to write the failing tests first for each step, then the code, and not to commit. The developer committed the result afterwards as one commit, `0080892` (18:16), which holds the tests and the implementation of config, decision, geoipdb, httpapi, reload, request and main, plus the Dockerfile and the Kubernetes manifest. The history cannot show that the tests were red before the code existed; that rests on the session log.
- The rule "skeleton, tests and implementation as separate commits" in `CLAUDE.md` was therefore not met for those steps.

## Changes to approved tests

CLAUDE.md says an existing test is never changed to make it pass. These are all the changes made to committed tests after they were committed:

| Commit | Test | Change | Reason |
|---|---|---|---|
| `cbfee78` | `internal/netclass/classify_test.go`, `TestClassify` | Added one subtest: the zero address is classified as reserved | Fail closed: an invalid address must never count as public. G51, G54. |
| `96dd759` | `internal/httpapi/checks_test.go`, `TestUnknownFieldsLoggedAtDebug` | The assertion changed from "names are `[bar foo]`" to "no names are logged" | Unknown field names are caller-controlled and can hold an IP address. Found by the review; G75. |
| `96dd759` | `internal/httpapi/checks_test.go`, `TestUnknownFieldNamesAreCapped` | Deleted and replaced by `TestUnknownFieldNamesAreNotLogged` | The 20-name cap no longer exists after G75. |

Other changes in `96dd759` only add tests. Fixing the code is in `1aa6a5a`.

## Review after the build

After commit `0080892`, Claude Code was also used to review the repository. It wrote the two test files `internal/httpapi/audit_test.go` and `internal/request/audit_test.go`. They were accepted because they test documented behaviour, add no dependency and pass `go vet` and the race detector. Its findings 2 and 3 were fixed with Claude Code (G75, G76); three other findings were not fixed and are listed as known limitations (E11 to E13 in `docs/design-notes.md`).

## Developer replies

A second Claude Code session was used to clean up and standardise the developer's replies, so that the main agent received clear, consistent prompts. The developer reviewed and sent each reply.

## Agents

The first stage (design, `ai-log/01-design`) used two Claude Code subagents that the developer defined, in `.claude/agents/`: `designer` turns the request into a design and an ordered plan of small steps, and `grillz` challenges that plan for gaps and untestable steps before the developer approves it. They only read files and never write code. Every later stage was a plain Claude Code session without custom agents.

## Accepted and rejected AI output

Each phase has a `decisions.md` with a table: what the AI proposed, what the developer chose, and the reason (quoted, or "no reason given"). They were written by Claude Code from the session of that phase and are listed below. Examples of rejected or corrected AI output are in those tables, for example the `/metrics` library choice (`04-coding`) and the manifest defects found by running the image (`05-refinements`).

## Prompts and sessions

Edited copies of the Claude Code sessions, one set per phase. Personal identifiers are replaced with placeholders and the developer's instructions are lightly paraphrased, so they are not verbatim. Tool calls and results are left out. Raw system notifications are removed. The logs are the exported sessions of each phase, not necessarily every prompt. The timestamps in the session files are placeholders and cannot be matched to commits. Final refinements made after the logs were exported are not included.

| Phase | Decisions | Prompts | Session |
|---|---|---|---|
| Design | [Decisions](ai-log/01-design/decisions.md) | [Prompts](ai-log/01-design/prompts.md) | [Session](ai-log/01-design/session.md) |
| Contract review | [Decisions](ai-log/02-contract/decisions.md) | [Prompts](ai-log/02-contract/prompts.md) | [Session](ai-log/02-contract/session.md) |
| Skeleton and tests | [Decisions](ai-log/03-skeleton-tests/decisions.md) | [Prompts](ai-log/03-skeleton-tests/prompts.md) | [Session](ai-log/03-skeleton-tests/session.md) |
| Implementation | [Decisions](ai-log/04-coding/decisions.md) | [Prompts](ai-log/04-coding/prompts.md) | [Session](ai-log/04-coding/session.md) |
| Refinements | [Decisions](ai-log/05-refinements/decisions.md) | [Prompts](ai-log/05-refinements/prompts.md) | [Session](ai-log/05-refinements/session.md) |
