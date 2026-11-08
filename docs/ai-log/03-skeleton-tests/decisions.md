# Decision record: skeleton, tests and first implementation (2026-11-08)

Scope: this session covered S-004, the skeleton and tests for S-007 (`iso`) and S-008 (`netclass`), their implementation, and S-005 and S-006 (test database README check and benchmark). It was not the design, plan-review or OpenAPI session. Everything below comes from this conversation only. Where the developer gave no reason, the table says so. Nothing was committed after the three commits made at the start (S-004, skeleton, red tests); all later work is uncommitted at the time of writing.

## 1. Decisions

| # | Decision | What the AI proposed | What I chose | My reason (quote) |
|---|---|---|---|---|
| 1 | Which steps to start with | The AI picked S-004 first (it was not done and S-007, S-008 depend on it), then skeleton and tests for S-007 and S-008. It said it was skipping S-005 and S-006, a departure from the plan order. | Accepted. S-005 and S-006 come after S-007 and S-008. | No reason given. "it is fine to skip S-005 and S-006 for now. Do them after S-007 and S-008." |
| 2 | Names in the skeleton | `Codes`, `Valid`, `Parse`, `Classify`, `Category*`, `MaskedAddr` (the plan names none) | Accepted. | No reason given. "Codes, Valid, Parse, Classify, Category* and MaskedAddr are fine." |
| 3 | ISO check-date comment | The plan test line said both "reads no file" and "check-date comment present". The AI left that check out and offered two ways: read the source in the test, or expose the date as a constant. | Neither. Keep the comment next to the list; the reviewer checks it. Record it in G (G49). | No reason given. "Please do not read the source in the test, and do not add a date constant. Keep the comment in iso.go next to the list, and leave it to the reviewer to check it. Record this in G." |
| 4 | How `MaskedAddr` is logged | The struct printed `{}` in JSON, so the implementation needed `LogValue()` or `MarshalText()`. | `slog.LogValuer` (`LogValue`), no `MarshalText`. Recorded in G34. | The developer said the notes already say it (G34 only said `String()`). "implement slog.LogValuer (LogValue method), as the notes say, so the log shows the prefix. Do not use MarshalText." |
| 5 | Commits | The AI had committed S-004, the skeleton and the red tests. | No more commits for now; repeated twice later. | No reason given. "Do not commit for the moment." / "Do not commit anything yet." |
| 6 | Library function name | The AI found that `maxminddb-golang/v2` v2.9.0 has no `FromBytes`; the in-memory constructor is `OpenBytes`. It stopped and asked, as the plan requires. | Use `OpenBytes`. Fix the notes (G) first, then the plan, in change order. Record that `OpenBytes` keeps the slice, so it is never modified (G50). | No reason given for the name. "Yes, carry on with S-005 and S-006 with OpenBytes. First fix the name in the notes (G), then the plan, in the change order." |
| 7 | S-007 plan test line | The AI pointed out that "check-date comment present" contradicts G49. | Remove it. | "to match G49". "Please remove "check-date comment present" from the S-007 test line in the plan, to match G49." |
| 8 | `Classify` of the zero address | The AI left it returning `public` (the plan was silent) and asked. | Return `reserved` (G51). | "Classify of the zero address: return reserved, so an invalid address can never count as public. Fail closed." |
| 9 | Wording of the ISO list comment | The AI's first comment said the list was checked against ISO's Online Browsing Platform and the glossary. | The comment must say the codes were copied from tzdata `iso3166.tab` and not checked code by code against ISO; the reviewer must do that (G52). | "Do not write that they were checked against ISO's Online Browsing Platform. I only checked IT and UK there." |
| 10 | Amount of code comments | The AI's first comments were long. | Keep only comments that explain the why. | "Minimize the comments since they should explain just the why when is not clear" |
| 11 | `go` line in `go.mod` | `go mod tidy` changed `go 1.26` to `go 1.26.0`, which conflicts with KD20. The AI asked. | Keep `go 1.26.0` and `toolchain go1.26.10`. Update KD20 and S-004, record in G (G53). | "keep go 1.26.0, as go mod tidy wrote it. Update KD20 and S-004 to match, and record it in G. Keep toolchain go1.26.10. Do not fight the tool." |
| 12 | Test for the zero-address branch | No committed test covered it (coverage fell to 96.2%). The AI asked before touching a committed test. | Add one case to the existing `TestClassify`; record in G (G54) and in `docs/ai-usage.md` that a committed test was extended after approval, and why. | No reason given in the message. "yes, add one test case to the existing Classify test and record it in G and in docs/ai-usage that a commited test was extended after approval and why. Change nothing else in tests." |
| 13 | Network use and versions for `go mod tidy` | Tidy downloaded test-only modules of the library (`testify`, `yaml/v3`); the AI flagged it. | Accepted. Pin the versions, add nothing more. | No reason given. "accepted. Pin the versions and do not add more." |
| 14 | Stray files in `/tmp` | The AI had copied `go.mod` and `go.sum` into `/tmp` by mistake, checked they were identical and removed them. | Fine. | No reason given. "Ceanup is fine." |
| 15 | Meaning of "numbers only" and the benchmark code | The AI had read it as a rule for the S-006 step. | Numbers are recorded, there is no pass/fail threshold, and the benchmark code stays in the repository. | "I misread "numbers only" earlier. It means the numbers are recorded and there is no pass/fail threshold. The benchmark code may stay in the repo." |
| 16 | Where the benchmark numbers live | The plan said G2, and the AI had written them there. The AI noted that D7 did not exist. | New D7 (storage choice: in-process `.mmdb`, not Redis, plus the four results, CPU, Go version, date, "one run on the synthetic database", no threshold). G2 only points to D7 (G55). | "Put the numbers there. G2 only points to D7. Do not keep them in two places." |
| 17 | D numbering | The AI reported that only D1 to D6 and D11 exist. | Keep the lettering; D8 to D10 stay unused. | The developer's rule was to keep D for side notes and D11 for the pod. "D8 to D10 staying unused is fine." |
| 18 | S-001 test line | The AI flagged that "G1 to G47" was stale. | Reword: G has no gaps in its numbering and every item is listed once; no fixed count. Record in G55. | "so the test does not break each time we add an item" (quoted from: "do not pin it to a fixed count. Write that G has no gaps in its numbering and that every item is listed, so the test does not break each time we add an item.") |
| 19 | Fixture for "no fallback to `registered_country`" | The README table had no `registered_country` column. | Add the column and a `5.83.124.1` row (RO, no country); `TestDatabaseMatchesExpectedFacts` checks it; nothing else in the tests changes (G56). | "Our key test for "no fallback to registered_country" needs an address in the test database that has no country but has a registered_country" |
| 20 | Extra reserved ranges and `address_category` text | The Classify test also covers `0.0.0.0/8`, `192.0.0.0/24`, `198.18.0.0/15`, `240.0.0.0/4`. | Record these in G (G57). Pin the text of the `address_category` values by a test when the logging step is written (G58). | No reason given. "record in G the extra reserved ranges in the Classify test, and that the text of the address_category values must be pinned when the logging step is written." |
| 21 | Approval of the tests | The AI had listed the tests of S-005 to S-008. | Approved, with the request in decision 19. | No reason given. "I approve the tests of S-005 to S-008, with one request." |

## 2. Plan review

No plan-review objections were raised in this conversation. The plan was already approved before it started. Edits to the plan made here follow decisions 6, 7, 11, 16, 18 and 19 above.

## 3. Choices while writing the OpenAPI file and the notes

- `api/openapi.yaml` was not changed in this session.
- Notes items added or changed (all decided by the developer, 2026-11-08):
  - G15 and H3: `OpenBytes` instead of `FromBytes`.
  - G34: `slog.LogValuer`, no `MarshalText`.
  - G49: ISO check date is a comment only, no test, no constant.
  - G50: `OpenBytes` rename and slice rule.
  - G51: `Classify` of the zero address is `reserved`.
  - G52: the ISO list was copied from tzdata and is not fully checked.
  - G53: `go 1.26.0` and pinned versions; H6 and E10 updated.
  - G54: one case added to the committed `TestClassify`.
  - G55: D7 added; D8 to D10 unused.
  - G56: `Registered country` column and the `5.83.124.1` fixture.
  - G57: extra reserved ranges.
  - G58: `address_category` text to be pinned at S-014.
  - G2 is now a pointer to D7.
- Plan lines changed: KD20, S-004, the S-005 table, the S-006 recording place, the S-007 test line, the S-001 test line, and the `FromBytes` mentions.

## 4. Things the AI got wrong and I corrected

- **ISO comment overclaim (decision 9).** The AI's comment said the list had been checked against ISO sources. The developer had checked only `IT` and `UK`, and corrected it.
- **Stray copy in `/tmp`.** The AI put `go.mod` and `go.sum` into `/tmp` by mistake. It noticed, verified and removed them, and the developer accepted the cleanup (decision 14).
- **Stale items the AI had left or created, found and flagged by the AI:** `FromBytes` in the plan and notes (not a method in v2.9.0), G2 holding the numbers when the developer wanted D7, and "G1 to G47" in the S-001 test line.
- **Summary hid a leading space.** The AI's report showed the test input `" 1.2.3.4"` in backticks, and the leading space was lost, so the developer read it as the valid address `1.2.3.4`. The test and the implementation were correct, and no change was made. The AI should have written the string quoted.
- **Where the developer's premise did not match the files** (the AI pointed this out, nothing was wrongly changed): the notes did not mention `LogValuer` (decision 4), and D7 did not exist (decision 16).
