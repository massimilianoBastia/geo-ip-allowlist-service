# Decision record: session 04, coding

Scope: this file covers only the coding session (steps S-009 onward). The design, the plan review and the writing of the OpenAPI file and the notes are not part of this conversation, so they are not recorded here and the plan-review table is empty. Quotes are from the developer's messages in this session.

## 1. Decisions of the developer

| Decision | What the AI proposed / the options | What the developer chose | Developer's reason (quote) |
|---|---|---|---|
| Order of work | - | Implement everything missing in `plan.md` from S-009, in plan order, tests first, then code, gate after each step | "Write the failing tests first, from the plan's test lines." |
| Which source wins when they disagree | - | Notes, then contract, then brief, then plan | "When sources disagree, follow this order: the notes, the contract, the brief, the plan." |
| How to handle unclear points | Ask one question at a time (the default in CLAUDE.md) | Do not stop; choose the safest behaviour, record it in G as "proposed", list the questions at the end | "Do not stop to ask one question at a time. Collect the questions and list them at the end." |
| Safe default for gaps | - | Fail closed, an error rather than a guess | "Choose the safest one (fail closed, an error rather than a guess), record it in G as \"proposed\"" |
| Dependencies | - | Only `maxminddb-golang` v2; `prometheus/client_golang` only for `/metrics`; pin versions | "Add no dependency except the approved maxminddb-golang v2 (and prometheus/client_golang only for /metrics). Pin versions." |
| Logging of addresses | - | Never log an IP; log `address_category` and `session_id` only | "Never log an IP address. Log address_category and session_id only." |
| Credentials | - | Read only from read-only mounted files in the update container | "Read the MaxMind credentials only from read-only mounted files in the update container, never from environment values, never in the image or the repo." |
| Scope | - | No gRPC, no Keycloak guard; `/metrics` and Compose last and only if the rest is done | "Do the optional /metrics endpoint and the optional docker-compose.yml last, and only if everything else is done." |
| Commits and review | CLAUDE.md asks for one commit per step | No commits, and the AI does not run the reviewer | "Do not commit anything. Do not run the reviewer." (repeated: "Do not commit anything. Stop here.") |
| `docs/problems.md` | AI asked whether a reference table of the seven problem types was wanted (it did not exist in the repository) | Yes, a reference table | "Yes a reference table is what I wanted." |
| Country name when empty (G72) | Put the name into the sentence as is, giving "located in GB ()"; asked whether to change it | Keep `country.name` exactly as the database gives it; in the `detail` sentence drop the parentheses when the name is empty | "country.name is the response stays exactly as receied from the database (an empty string where there is none). In the detail sentence use the cleaned form so drop the parentheses when the name is empty." |
| Probe and `/metrics` logging (G59) | No summary line for the probes (to keep the log quiet); asked whether to add one at DEBUG | Probes and `/metrics` write a summary line at DEBUG only | "Yes probes and /metrics write a summary line at DEBUG only." (no further reason given) |
| Panic value in logs (G61) | Log the Go type and the stack, not the value; asked if acceptable | Accepted | "Panic value: accepted. THe panic value could contain personal data such as an IP address." |
| `PROBLEM_BASE_URL` without trailing slash (G62) | Reject it with a startup error; asked whether to add the slash instead | Add the slash when missing; still reject a value that is not an absolute URL | "Add the trailing slash when it is imissing. Reject a value that is not an obsolute URL with a startup error." (no further reason given) |
| Sidecar image tag | Pinned `ghcr.io/maxmind/geoipupdate:v7.1.0`, not checked against the registry | The developer checks the tag; keep the settings from the notes | "I will check the tag against the registry myself. Keep the settings from the notes: edition GeoLite2-Country, a check every 24 hours, the _FILE variables, and the credentials mounted read-only in the sidecar only." |
| Container and cluster checks | AI could not run them (no docker, no kubectl) | The developer runs the S-018 image checks with podman and tries the manifest where possible | "I will run the Docker image checks of S-018 myself with podman, and I will try the Kubernetes manifest where I can." (no further reason given) |
| Library for `/metrics` (G69) | 1. Keep the standard library: three series, about 74 lines, no new dependency. 2. Switch to `prometheus/client_golang`, which the developer had already allowed for `/metrics` only: gives runtime metrics and histograms, but needs a new dependency, a rewrite of `metrics.go` and its tests, and changes to K4, G69, KD21 and A13. The AI had built option 1 on its own (G69, proposed) and had not put the choice back to the developer. | Option 1: keep the standard library (chosen on 2026-11-08, when asked). Revisit only if histograms or runtime metrics are wanted. | Reasons given by the AI and accepted: simple, no extra dependency, fits the three series. The developer gave no further quote. |

## 2. Plan review

No plan review took place in this session. There are no objections to record.

## 3. Choices the AI made while writing the code and the notes

Most are recorded as "proposed" in G59 to G72 of `docs/api-design-notes.md`. G59, G61, G62 and G72 were later decided by the developer (table above). The others are still proposals.

- `reload`, `decision` and `httpapi` use a small `geoipdb.Database` interface; the opener and the stat call are interfaces with real implementations (G67).
- Added a file the plan does not list: `internal/httpapi/httpapi.go` (G67).
- `session_id` or `requested_at` set to `null` is ignored with a warning (G60).
- A body read failure that is not the size limit gives 400 (G63). HEAD on `/v1/ip-checks` gives 405 (G64).
- Reload log attributes and the once-per-failure rules (G65). `run` logs its error once and returns it (G66).
- The recovery middleware learns the session id through a pointer in the request context (G68).
- `/metrics` uses only the standard library, because the notes (K4, KD21) win over the task text that allowed `prometheus/client_golang`; all series are written, with 0 before the first event (G69).
- Deployment defaults: non-root, read-only root filesystem, `fsGroup`, small resource values, a NetworkPolicy for ingress only, Compose on `127.0.0.1` (G71).
- Unknown-field names are logged sorted, at most 20, with a count (from the plan).
- Acceptance scenarios call the handler directly with a recorder; the lifecycle test uses an in-memory listener, so no test opens a socket.
- Not verified: the Docker build and container checks, and `kubectl apply --dry-run` (no docker or kubectl on the machine). A manual run of the binary on loopback gave exit code 0 on SIGTERM.

## 4. Things the AI got wrong

The developer did not report any error as such. These are the AI's own mistakes, found and fixed by the AI during the session:

- A recovery-middleware bug: it reassigned the shared logger with the session id, so one request's session could leak into the next panic log. Fixed, and a regression test was added.
- A draft of `main.go` ended with a global variable for the reloader. It was rewritten without it.
- A test file was written with a stray placeholder method and was rewritten in full.
- The first SIGTERM check read the exit code of a background subshell (143) instead of the service. Redone properly: exit code 0.
- Tests written after the code they test (the S-017 scenarios) were never red. Two negative tests and one reload test passed against their stubs. These are listed honestly in the report of the session.

Note: `docs/ai-log` is listed in `.gitignore` ("AI Dump"), so this file is not committed unless that rule changes.
