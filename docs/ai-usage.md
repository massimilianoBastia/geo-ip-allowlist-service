# Use of AI

Claude Code was the assistant for the design, the contract, the tests and the code, as the challenge allows. The design decisions were made by the developer, with the AI as an adviser that proposed options and checked claims against sources.

After commit `0080892`, Claude Code was also used to review the repository (`docs/testing-audit.md`). It wrote that report and the two test files `internal/httpapi/audit_test.go` and `internal/request/audit_test.go`. They were accepted because they test documented behaviour, add no dependency and pass `go vet` and the race detector. Its findings 2 and 3 were then fixed with Claude Code (G75, G76); findings 1, 4 and 5 were not fixed and are listed as known limitations in that report.

## Developer replies

A second Claude Code session was used to clean up and standardise the developer's replies, so that the main agent received clear, consistent prompts. The developer reviewed and sent each reply.

## Process and time

- **Time spent:** about 4 hours 30 minutes of work in total, which is above the 4 hours the challenge suggests. This is the developer's estimate from memory; no time was tracked. The GitHub repository was created at 11:44 on 2026-11-08, the first commit is at 15:49 and the last pushed work is at 19:56 the same day, so the elapsed time is longer than the hours of work, and the exact breaks were not recorded.
- **Order of work:** the design notes and the contract (`api/openapi.yaml`) were written first, before the code. They were committed late (16:45), after the first code commits, so the commit order does not show the real order of work.
- **Agents:** the first stage (design, `ai-log/01-design`) used two Claude Code subagents that the developer defined, in `.claude/agents/`: `designer` turns the request into a design and an ordered plan of small steps, and `grillz` challenges that plan for gaps and untestable steps before the developer approves it. They only read files and never write code. Every later stage was a plain Claude Code session without custom agents.

## Prompts and sessions

Edited copies of the Claude Code sessions, one set per phase. Personal identifiers are replaced with placeholders and the developer's instructions are lightly paraphrased, so they are not verbatim. Tool calls and results are left out. Raw system notifications are removed. The logs are the exported sessions of each phase, not necessarily every prompt. Final fixes made after the logs were exported are not included.

| Phase | Prompts | Session |
|---|---|---|
| Design | [Prompts](ai-log/01-design/prompts.md) | [Session](ai-log/01-design/session.md) |
| Contract review | [Prompts](ai-log/02-contract/prompts.md) | [Session](ai-log/02-contract/session.md) |
| Skeleton and tests | [Prompts](ai-log/03-skeleton-tests/prompts.md) | [Session](ai-log/03-skeleton-tests/session.md) |
| Implementation | [Prompts](ai-log/04-coding/prompts.md) | [Session](ai-log/04-coding/session.md) |
| Refinements | [Prompts](ai-log/05-refinements/prompts.md) | [Session](ai-log/05-refinements/session.md) |
