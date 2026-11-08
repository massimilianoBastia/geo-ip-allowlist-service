# Massimiliano's prompts in this session (03-skeleton-tests)

> Edited disclosure copy: personal identifiers are replaced with placeholders, developer instructions are lightly paraphrased (so they are not verbatim), and AI replies and reports keep their wording apart from redactions.

10 prompts. Personal data is redacted.

### Massimiliano, prompt 1


Please work on the Go tests first, before the implementation.

Follow CLAUDE.md. The approved plan is docs/design/plan.md and the contract is api/openapi.yaml. Tests written first show that the plan is testable, and they must fail for the right reason. I have limited time, so keep each step small.

1. Create the empty skeleton: packages, types and signatures with no logic, using the layout in the plan. Bodies return a "not implemented" error or a deliberately wrong value, so a stub can never pass a test by accident. Show it to me and commit it.
2. Write the table-driven tests for the first plan steps. Tell me which steps you start with, and for each plan step say if its tests go next to the code or in tests/scenarios/.
3. Run the tests and show that each one fails on its assertion, not on a missing symbol.

Do not write the implementation. Do not change a test after it is committed. Ask me if a test looks wrong. Do not open a network socket in any test, and do not use the real GeoLite2 file; use testdata/GeoIP2-Country-Test.mmdb. Do not add a third-party package without asking me.

The skeleton commit, then the test run (red), and the list of test cases with the plan step each one covers.

### Massimiliano, prompt 2

it is fine to skip S-005 and S-006 for now. Do them after S-007 and S-008. Codes, Valid, Parse, Classify, Category* and MaskedAddr are fine. Please do not read the source in the test, and do not add a date constant. Keep the comment in iso.go next to the list, and leave it to the reviewer to check it. Record this in G. implement slog.LogValuer (LogValue method), as the notes say, so the log shows the prefix. Do not use MarshalText. Do not commit for the moment.

### Massimiliano, prompt 3

Ah you wrote that 1.2.3.4 is an error row. That is a valid IPv4 address. Which input did you mean (for example with surrounding whitespace, or a CIDR)? Please correct the case, so it matches what the tests assert.

### Massimiliano, prompt 4

Yes, carry on with S-005 and S-006 with OpenBytes. First fix the name in the notes (G), then the plan, in the change order. Remind yourself that OpenBytes keeps the slice, so the caller must not modify it. Please remove "check-date comment present" from the S-007 test line in the plan, to match G49. Classify of the zero address: return reserved, so an invalid address can never count as public. Fail closed. The comment next to the ISO list must be truthful. Say that the codes were copied from tzdata iso3166.tab, and that they have not been checked code by code against ISO. The reviewer must do that. Do not write that they were checked against ISO's Online Browsing Platform. I only checked IT and UK there. Minimize the comments since they should explain just the why when is not clear

### Massimiliano, prompt 5

1. go directive: keep go 1.26.0, as go mod tidy wrote it. Update KD20 and S-004 to match, and record it in G. Keep toolchain go1.26.10. Do not fight the tool. 2. yes, add one test case to the existing Classify test and record it in G and in docs/ai-usage that a commited test was extended after approval and why. Change nothing else in tests. 3. accepted. Pin the versions and do not add more. 4. Ceanup is fine.

### Massimiliano, prompt 6

Ah wait I misread "numbers only" earlier. It means the numbers are recorded and there is no pass/fail threshold. The benchmark code may stay in the repo. Record in D7 the four results with the CPU, the Go version and the date, and say they come from one run on the synthetic database.

### Massimiliano, prompt 7

add D7 (storage choice: an in-process .mmdb file, not Redis; the numbers of the standard lookup, with the CPU, Go version, date, and the note that they come from one run on the synthetic database, with no threshold). Put the numbers there. G2 only points to D7. Do not keep them in two places. If D7 to D11 already exist under other numbers, tell me and keep the lettering we fixed: D for side notes, D11 for how the database reaches each pod. For s-001 yes reword the test line. Benchmark code in the repo is allowed and "numbers only" means no pass/fail threshold. Record the cange in G and not commit anything yet

### Massimiliano, prompt 8

Yes, update the S-001 test line, but do not pin it to a fixed count. Write that G has no gaps in its numbering and that every item is listed, so the test does not break each time we add an item. If you prefer a number, use G55. D8 to D10 staying unused is fine. Do not commit anything yet.

### Massimiliano, prompt 9

I approve the tests of S-005 to S-008, with one request. Our key test for "no fallback to registered_country" needs an address in the test database that has no country but has a registered_country (in the MaxMind source data, 5.83.124.0/22 with RO). The README table has no registered_country column. Please add that column, show that case in the table, and make TestDatabaseMatchesExpectedFacts check it. Do not change anything else in the tests. Also record in G the extra reserved ranges in the Classify test, and that the text of the address_category values must be pinned when the logging step is written. Do not commit anything yet.

### Massimiliano, prompt 10

Ok now 

Please help me write the notes for this session.

I need a short decision record for the AI-usage page of my project. The reviewer wants to see what the AI proposed, what I decided, and why. This session had three parts: the design, the plan review, and the writing of the OpenAPI file and the notes.

Read the conversation and list the decisions I made. For each one, say what you suggested, what I chose and why. Use my words where possible.

Use only this conversation. Please do not add decisions I did not make. If I gave no reason, write "no reason given". Add a short quote of my message for each decision.

 1. A table: Decision | What the AI proposed | What I chose | My reason (quote).
2. A second table for the plan review: Objection | Accepted or dismissed | My reason (quote).
3. A short list of the choices you made while writing the OpenAPI file and the notes. A short list "Things the AI got wrong and I corrected", if there are any.
Save the result in docs/ai-log/03-skeleton-tests/decisions.md. 

