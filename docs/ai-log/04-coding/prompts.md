# Massimiliano's prompts in this session (04-coding)

> Edited disclosure copy: personal identifiers are replaced with placeholders, developer instructions are lightly paraphrased (so they are not verbatim), and AI replies and reports keep their wording apart from redactions.

3 prompts. Personal data is redacted.

### Massimiliano, prompt 1


Please implement the Go service with simple, idiomatic code and check your work. I will run a separate review after.

The service tells an API gateway whether an IP address is inside a list of allowed countries. The design is finished and approved. The decisions and their reasons are in docs/api-design-notes.md, the contract is api/openapi.yaml and docs/problems.md, the short rules are in docs/agent-design-brief.md, the plan is docs/design/plan.md, and the working rules are in CLAUDE.md. Steps S-004 to S-008 are done and their tests are approved. Please focus on clear code, the structure and the API design before adding extra features. After you finish, I run the reviewer myself, then we stop.

Implement everything that is still missing in docs/design/plan.md, from S-009 onward, in plan order. For each step:
1. Write the failing tests first, from the plan's test lines. Unit tests sit next to the code. Acceptance tests go in tests/scenarios/. Use only testdata/GeoIP2-Country-Test.mmdb. No test opens a network socket or uses the real GeoLite2 file.
2. Write the minimum code that makes them pass, following CLAUDE.md, Effective Go and Go Code Review Comments.
3. Run gofmt -l ., go vet ./... and go test -race -cover ./... and fix every failure before the next step.
The remaining work is: request validation, the decision logic, the HTTP handler and request pipeline, the RFC 9457 problem documents, slog logging, /livez, /readyz and /version, the atomic database reload with verification, the recovery middleware, graceful shutdown, configuration from environment variables, the Dockerfile, the Kubernetes manifest with the update sidecar, and docs/problems.md kept in step with the code. Do the optional /metrics endpoint and the optional docker-compose.yml last, and only if everything else is done.

- When sources disagree, follow this order: the notes, the contract, the brief, the plan. Please do not make up behaviour. Choose the safest one (fail closed, an error rather than a guess), record it in G as "proposed", and keep going. Keep the questions together and send them at the end, instead of stopping for each one.
- Do not change the approved tests of S-005 to S-008. If you must, record the reason in docs/ai-usage.md and in G.
- Add no dependency except the approved maxminddb-golang v2 (and prometheus/client_golang only for /metrics). Pin versions.
- Never log an IP address. Log address_category and session_id only. Use the masked address type if a line ever needs an address.
- Read the MaxMind credentials only from read-only mounted files in the update container, never from environment values, never in the image or the repo. The _FILE variables hold paths, not secrets.
- Follow the limits in the notes: a 16 KiB body cap, server timeouts, graceful shutdown on SIGTERM, errors without internals.
- Keep the service small. Do not build gRPC or the Keycloak guard.
- Do not commit anything. Do not run the reviewer.

When you finish, send me: (1) the steps finished and the result of gofmt, go vet and go test -race -cover; (2) a coverage table by package; (3) the G entries you added as "proposed"; (4) your open questions, in a short list. Then stop.

### Massimiliano, prompt 2

Yes a reference table is what I wanted. 2. Keep two things apart. country.name is the response stays exactly as receied from the database (an empty string where there is none). In the detail sentence use the cleaned form so drop the parentheses when the name is empty. 3. Yes probes and /metrics write a summary line at DEBUG only. 4. Panic value: accepted. THe panic value could contain personal data such as an IP address. 5. Add the trailing slash when it is imissing. Reject a value that is not an obsolute URL with a startup error. 6. I will check the tag against the registry myself. Keep the settings from the notes: edition GeoLite2-Country, a check every 24 hours, the _FILE variables, and the credentials mounted read-only in the sidecar only. I will run the Docker image checks of S-018 myself with podman, and I will try the Kubernetes manifest where I can. Do not commit anything. Stop here.

### Massimiliano, prompt 3

ok now 

Please help me write the notes for this session.

I need a short decision record for the AI-usage page of my project. The reviewer wants to see what the AI proposed, what I decided, and why. This session had three parts: the design, the plan review, and the writing of the OpenAPI file and the notes.

Read the conversation and list the decisions I made. For each one, say what you suggested, what I chose and why. Use my words where possible.

Use only this conversation. Please do not add decisions I did not make. If I gave no reason, write "no reason given". Add a short quote of my message for each decision.

1. A table: Decision | What the AI proposed | What I chose | My reason (quote).
2. A second table for the plan review: Objection | Accepted or dismissed | My reason (quote).
3. A short list of the choices you made while writing the code file and the notes.
4. A short list "Things the AI got wrong and I corrected", if there are any.
Save the result in docs/ai-log/04-coding/decisions.md.

