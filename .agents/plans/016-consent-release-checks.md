# Plan 016: Check releases only with consent

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Users can opt into quiet asynchronous stable-release notifications or explicitly request a manual check.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- The owner confirmed `namra4122/terminal-typer` as release authority. Use HTTPS GET `https://api.github.com/repos/namra4122/terminal-typer/releases/latest`; no custom service or new infrastructure. Repository404/no stable release is an unavailable check, not permission to switch upstream.
- Timeout5 seconds including connection/response; max256 KiB body; reject redirects. Use standard-library HTTP/TLS with normal certificate validation, no credentials/cookies/install ID, no executable downloads. User-Agent `terminal-typer/<version> (<GOOS>; <GOARCH>)`, Accept application/vnd.github+json. Disclose IP/unavoidable network metadata.
- Auto attempts are at most once per24 hours, with attempt time committed before request. Manual check is explicit consent for that request and also updates attempt time. Network failure is silent automatically and visible manually.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- After the first completed Result, display a one-time consent choice while preserving that result. Enable/Not now commits enabled/declined. Escape/quit without a choice leaves unset.
- Add Settings Data choice and manual Check for updates palette action. Do not run on help/list/version or scripting invocations (-json/-csv/-oneshot/-noreport), including enabled saved consent.
- When enabled, enqueue a due check after the ready screen is rendered; no network gates launch/input. Show a quiet new stable version/link/install guidance notice with dismiss.
- Never download/install/execute/restart or send user settings/resource/text/result/task content.

## Explicitly deferred scope

- Beta/nightly channels, update installers, content downloads, telemetry and custom update servers are product non-goals.

## Requirement coverage

UPDATE-01 through UPDATE-09; CFG-05; UI-08 Data, UI-09 manual check; PERF-01, PERF-06; A-14.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Unset/declined consent yields0 automatic requests. Unset consent appears after the first result, not startup; dismiss leaves it unset, Not now keeps it declined across relaunch.
2. Enabled consent generates at most1 auto attempt per24h across two simultaneous launches, including failed requests. Attempt persistence failure yields0 requests and an actionable Data error.
3. Manual checks work while auto declined, time out within 5s, enforce body/redirect/response validation, and show concise failure; auto failures show no distracting notice.
4. Request capture contains only allowed version/OS/arch metadata and GitHub protocol fields, with no sentinel typing/settings/path/installation data.
5. Newer stable versions show notification/link only; dismissal persists for that version. Same/older/prerelease/draft versions show no automatic new-release notice. Scripting/help/list/version never request or display consent.

## Keyboard journey

1. Complete the first regular test and keep Results accessible behind consent.
2. Choose Enable or Not now, or Escape to defer. Change that choice under Data.
3. Request Check for updates manually and read status without blocking typing.
4. Dismiss a newer-version notice or copy its verified release link for external installation.

## Verified reusable implementation

- `src/tt.go:480` currently embeds executable version0.4.2; keep one version constant rather than scattering comparison strings. The verified origin remote is git@github.com:namra4122/terminal-typer.git.
- Plan 005 supplies atomic settings transactions; plan 006 supplies palette/action routing and pause contexts; plan 015 isolates scripting stdout/early exits. No current update network client exists.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/updates.go` (new) | Consent/due/version/HTTP validation and asynchronous commands. |
| `src/updates_test.go` (new) | Fake-clock/server transport, payload and simultaneous launch cases. |
| `src/config.go` (plan 005) | Add optional update state and atomic due-claim transaction. |
| `src/app.go` (plan 001) | Post-result consent/Data/manual/notice states. |
| `src/tt.go:480` | Single executable version and scripting early-exit predicates. |
| `src/app_test.go` (plan 001) | Consent/resume/scripting journeys. |
| `README.md`, `man.md` | Endpoint, transmitted fields, consent and manual install guidance. |

## Data model and constraints

Configuration adds optional `updates:{consent:string(unset/enabled/declined),lastAttemptUnixMS:int64,dismissedVersion:string,lastStableVersion:string,lastReleaseURL:string}`. Missing means unset,0,empty. Keep consent unset on Escape. Valid release URLs require scheme https, host github.com and path prefix /namra4122/terminal-typer/releases/tag/.
Response subset: `{tag_name:string,html_url:string,draft:bool,prerelease:bool}`. Reject missing fields, oversized body, invalid tag/URL. Tag grammar `v?MAJOR.MINOR.PATCH` with nonnegative numeric parts no leading zero except0. Reject prerelease/build tags for stable checks. Compare numeric tuples, not strings. Development/non-semver executable version yields a manual `development build; version comparison unavailable` and no automatic request.
A due claim commits lastAttempt under the configuration lock. If wall clock moves backwards, wait until current time >= saved attempt+24h for auto attempts; manual remains usable. Cache validated version/link in config; a saved newer undismissed notice can display offline without a request.

## Go and CLI contracts

```go
func DueUpdate(cfg Configuration, nowUnixMS int64, invocation ScriptingContext) bool
func ClaimUpdateAttempt(path string, nowUnixMS int64, manual bool) (bool,error)
func CheckStableRelease(ctx context.Context, client *http.Client, current string) (ReleaseInfo,error)
```
`ReleaseInfo` has `Version string; URL string; Newer bool`. Automatic claim uses24h predicate under lock; manual bypasses the predicate but still commits time. Single-flight checks per process; manual can observe an in-flight auto request rather than send a duplicate. Explicit test HTTP/transport injection is confined to tests; runtime endpoint is fixed HTTPS.401/403/429/5xx/404 and malformed/redirect/TLS failures become error states. Auto errors are silent after saved attempt. Manual errors are short and actionable. HTTP logs never include sensitive config/content.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 15

1. After first result becomes visible and outside scripting context, push consent attention overlay; preserve result and pause context. Save choice before enabling requests.
2. On enabled launch, render ready screen, atomically claim due attempt and run a cancellable HTTP command. Current test proceeds independently.
3. Validate stable response/version/link, save minimal cache and correlate late messages. Announce only newer undismissed version. Ignore results after consent revocation except preserving attempt timestamp.
4. Manual action is explicit network intent; show Checking/status and its error. Exit cancels request and closes bodies. Dismiss commits version; a newer distinct version may notify again.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Two launches both check | Atomic claim under config lock, skip second | +2 concurrency cases |
| Bad endpoint redirects/data | Reject redirects/oversized/malformed/foreign links, no install | +7 transport cases |
| Consent cancellation becomes approval | Only explicit committed choice enables request | +3 consent cases |
| Typing waits for network | Tea command/context5s and stale-message correlation | +2 slow-server journeys |
| Sensitive content in request | Allowlisted fixed headers/URL, capture sentinel test | +2 request fixtures |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Consent/due/semver/URL/dismissal/script predicates | 20 |
| Integration | Request allowlist, timeouts/redirects/limits/concurrent claims | 12 |
| E2E | First-result choice, manual failure and background typing | 3 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Add optional settings fields, defaulting to unset and no network. Revert removes checker/notice and retains recoverable state; no new server or release publication. Tests use injected transport and do not hit live release endpoints. Automatic checks require both valid executable version and stored consent; manual actions never authorize installation.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Unset/declined consent yields0 automatic requests. Unset consent appears after the first result, not startup; dismiss leaves it unset, Not now keeps it declined across relaunch.
- [ ] AC-2: Enabled consent generates at most1 auto attempt per24h across two simultaneous launches, including failed requests. Attempt persistence failure yields0 requests and an actionable Data error.
- [ ] AC-3: Manual checks work while auto declined, time out within 5s, enforce body/redirect/response validation, and show concise failure; auto failures show no distracting notice.
- [ ] AC-4: Request capture contains only allowed version/OS/arch metadata and GitHub protocol fields, with no sentinel typing/settings/path/installation data.
- [ ] AC-5: Newer stable versions show notification/link only; dismissal persists for that version. Same/older/prerelease/draft versions show no automatic new-release notice. Scripting/help/list/version never request or display consent.
