# Use of AI

AI coding tools (Claude Code) were used, as the challenge allows. The design decisions were made by the developer, with the AI as an adviser that proposed options and checked claims against sources.

## Two runs

The design was first worked out in an earlier repository (the design phase). This repository is a second run of the same workflow, session by session: design, plan review, contract, skeleton and failing tests, implementation, review and coverage, operations and documentation. Sessions 02 to 08 belong to this second run.

## Developer replies

In the second run, the developer's replies to the workflow agents were drafted with a second AI assistant from the decisions the developer had already recorded in the first run. The developer reviewed each reply and sent it. Where a reply repeats an earlier decision, the reason and the source are given in the notes (docs/api-design-notes.md).

## Changes to approved tests

| Test | Change | Reason |
|---|---|---|
| internal/netclass/classify_test.go | Added a subtest: the zero address is classified as reserved | Fail closed: an invalid address must never count as public. Approved by the developer and recorded in G51 and G54. |

## Prompts

The raw prompt logs are not published. They can contain sensitive data, so they are excluded from git.

## Corrections made to AI output

(To be completed at submission: for each piece of AI output, whether it was accepted or rejected, and why.)