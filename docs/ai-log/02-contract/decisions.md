# Decision record: contract review session

Source: the conversation in which `api/openapi.yaml` was reviewed against `docs/request.md`, `docs/design/plan.md` and `docs/api-design-notes.md`, linted with Redocly, and corrected. Only what was said in that conversation is recorded. The design session and the plan review are recorded separately in `docs/ai-log/01-design/decisions.md`; they are not repeated here. "The AI" means the main assistant of this session. Quotes are the developer's own words, typos included. Where no reason was given, the table says "no reason given". Entry IDs (A15, A16, CR1 to CR3, G48) match `docs/api-design-notes.md`.

Outcome: Redocly reports the API description as valid, with 0 errors and 4 warnings (accepted). Notes, plan, contract and brief agree on the decisions below. No code, no test code, nothing committed.

## 1. Decisions

| # | Decision | What the AI proposed or asked | What I chose | My reason (quote) |
|---|---|---|---|---|
| 1 | How to lint the contract | The AI tried `npx @redocly/cli lint`; the permission classifier denied it. The AI did not work around it, ran only a local structural check (YAML parse, `$ref`s, unique ids) and said it was not a real lint | I ran Redocly myself (`REDOCLY_TELEMETRY=off npx --yes @redocly/cli@2.76.0 lint`), saved the output in `docs/ai-log/04-contract/lint.txt` and pasted it back each time | no reason given (I pasted the command and its output) |
| 2 | Fix the 9 `security-defined` errors | Add a root-level `security: []`, which also states "no authentication" (notes J4) | Applied | "just apply small edits to make the specification compliant" |
| 3 | Licence | Redocly warns `info-license`; the AI would not pick a licence | First "Leave licence empty", later MIT (`name: MIT`, `identifier: MIT`) | "Use MIT license as license." (no reason given) |
| 4 | The 4 `operation-4xx-response` warnings on the `HEAD` operations | Four options: remove `HEAD` (notes first), add a Redocly config file, declare a 4xx on each `HEAD` (inaccurate), or keep the warnings. The AI recommended accepting them | Keep the 4 warnings. I had said so before ("keep 5 warnings") and confirmed it when asked | "Now the lint result is fine, keep 5 warnings" and "Ok fine with 4 warnings." (no further reason given) |
| 5 | How to close the content gaps from the review | The AI listed 15 findings and named four as important for the tests: error precedence, `ip_address` syntax, empty `Country.name`, the 413 detail | Draft three entries (precedence, `ip_address` syntax, `Country.name`), notes first, then the contract; anything beyond what I said is marked "proposed". The 413 detail and the other findings were not taken up | "yes draft the three entries before the tests so notes first then the contract. Mark anything beyond this as "proposed"." |
| 6 | Error precedence (notes A15, CR1) | The AI asked which error wins when several apply; the contract was silent | 405, then 415, then 413 and 400, each stops at the first failure; then one 422 with all problems, `ip_address` first, then the countries by ascending index; the lookup comes after validation, so an invalid request gets 422 even with no database; 503 only for a valid request | "follow the pipeline order: 405, then 415, then 413 and 400. Each stops at the first failure." and "The lookup comes after validation, so an invalid request gets 422 even with no database. 503 applies only to a valid request." (no further reason given) |
| 7 | `ip_address` syntax (notes A16, CR2) | The AI noted the contract only said "without a zone" and did not state the other rules (leading zeros, whitespace, empty string) | A string with an IPv4 or IPv6 text address, parsed with `net/netip`; whatever `netip.ParseAddr` rejects is a 422 with the right pointer; IPv4-mapped IPv6 is unmapped to IPv4; the reason text never repeats the IP | "A string with an Ipv4 or Ipv6 text address, parsed with net/netip. Anything netipParseAddr rejects is a 422 with correct pointer." and "the reason text NEVER repeats the ip" (no reason given for the parser choice) |
| 8 | `Country.name` (notes A5, CR3) | The AI pointed out that G22 (empty name) was not in the contract | The English name from the database, informational, never used for logic; `""` when the code exists but has no English name, so the object always has both members | "The english name from the database, informational, never used for logic. If the database has the code but no Englisn name return "" so the object alwasy has both members" (no further reason given) |
| 9 | Zone in `ip_address` (A16, A9, EC-08) | The AI tested `netip.ParseAddr("fe80::1%eth0")`: it succeeds and keeps the zone. This contradicted the approved design (A9, EC-08, S-008, S-009: a zone gives 422). The AI wrote the rest, marked the zone **U (open)**, kept 422 until I decided, and asked | A zone is not an error. Drop it with `WithZone("")`, then look up. `fe80::1%eth0` becomes `fe80::1`, link-local, so 200, `allowed:false`, `country_unknown`. This replaces the earlier 422 rule in the notes, the plan, the contract and the brief | "the zone only names a local network interface and has nothing to do with the country, and rejecting a valid address would turn a normal request into an error for no benefit" |
| 10 | Reason text for an invalid `ip_address` (G48) | Because a zone is no longer an error, remove "without a zone": "ip_address must be a valid IPv4 or IPv6 address." Marked **proposed**; the parser drops the zone first, then unmaps IPv4-mapped IPv6 | Approved the wording | "The wording for G48 is fine: "ip_address must be a valid IPv4 or IPv6 address."" (no further reason given) |
| 11 | Test for 503 versus 422 (plan S-014) | Add an assertion to S-014 that an invalid body returns 422 while the database is not loaded; S-009 does not need to change | Add it to S-014; keep S-009 as it is; add nothing else | "add to S-014 an assertion that an invalid body returns 422 while the database is not loaded. Keep S-009 as it is." and "Do not add anything else." (no reason given) |
| 12 | Status of G48 after approval | The AI left "(proposed)" on G48 and offered to update it | Nothing else to change | "Nothing else to change" (no reason given) |

## 2. Plan review

Not part of this conversation. The plan review (two `grillz` reviews) is recorded in section 2 of `docs/ai-log/01-design/decisions.md`. In this session I only made one change to the plan: decision 11 (S-014 assertion), plus the zone edits that follow from decision 9 (EC-08, EC-17, S-008, S-009, S-017).

## 3. Choices the AI made while writing the contract and the notes

- Review: listed 15 findings, each marked verified fact [V] or opinion [O], with the file and line; changed no file during the review, as asked.
- Wrote the notes first, then the contract, then the brief (the change order from the design session). The plan was edited after the notes.
- Gave the new rules their own IDs: A15 (error precedence) and A16 (`ip_address` syntax) in section A, and CR1 to CR3 in the appendix, dated 2026-11-08, instead of rewriting approved entries. A5 and A9 were edited in place.
- Put the check order in the `POST` description of the contract, and added to the 503 description that it applies only to a valid request.
- Declared the licence as `name: MIT` and `identifier: MIT`. Did not create a `LICENSE` file, because the plan does not ask for one; said so.
- Extended the existing EC-17 row for the zone case instead of adding an EC-43, so the references "EC-01 to EC-42" stay valid.
- Added a scenario to S-017 (`fe80::1%eth0`, `["GB"]`) and changed the S-008 and S-009 test lists for the zone; no test code existed, so no test was changed or deleted.
- Updated R16 in `docs/agent-design-brief.md` after the contract, because it also said "parsed without a zone".
- Marked the new reason wording as proposed (G48) and added the parser order, because it was a consequence of my decision and not something I wrote.
- Did not retry the denied `npx` command, and did not add a Redocly config file, because the plan lists no such file.
- Left the YAML check to PyYAML after each edit and left the Redocly run to me.

## 4. Things the AI got wrong and I corrected

- Nothing in this conversation was corrected by me as an AI error.
- Two small slips by the AI that I did not have to correct: (a) in the review, the AI expected a linter to also flag "no tags"; the Redocly run did not. (b) The first attempt to edit the `POST` description failed because the quoted text did not match the file; the AI fixed it in the next call.
- One conflict was raised by the AI, not by me: my instruction "a zone is dropped, not an error" contradicted the approved design. The AI did not apply it silently; it asked, and I decided (decision 9).
