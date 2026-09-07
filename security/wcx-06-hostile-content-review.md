# WCX-06 hostile content rendering security review

## Review state

Implementation review complete. Product Owner acceptance remains required.

Work package: `WCX-06`. Decisions: `WC-D24`, `WC-D25` option B, `WC-D26`,
`WC-D27`, `SEC-07`, `SEC-08`, `SA-11`, `RE-11`, `RE-12`, `DATA-02` through
`DATA-05`. Acceptance references: `SEC-08` hostile UI rendering release
blocker, `RE-12` permanent security regression fixtures.

## The threat this package addresses

Guardian is a deception product. A large share of what the console displays was
written by whoever attacked the network: decoy display names, SSH transcripts,
HTTP bodies, captured filenames, credential attempts. Phase 1 relied on React's
text escaping, which is necessary and not sufficient.

React escaping stops a captured string *executing*. It does nothing about the
attacks that forge what an operator sees, and all three of these survive
escaping intact:

- a right-to-left override reverses a filename, so `report<RLO>gnp.exe`
  displays as `reportexe.png` — an executable reading as an image;
- an ANSI sequence repaints a transcript, so a denial can be made to read as a
  grant, or a failure as `ALL CHECKS PASSED`;
- zero-width characters split a keyword, so `pass<ZWSP>word` is invisible to a
  reader scanning for it and to a search.

An operator who cannot trust what a transcript says cannot investigate with it.

## Security boundaries

- **Untrusted values are not strings.** `Untrusted` is an opaque type: at
  runtime it is the string it always was, but the compiler refuses to let it
  reach JSX, an attribute, a template literal, or a plain string. The only way
  to display one is `UntrustedText` or `UntrustedBlock`. Routing is a typecheck
  failure rather than a review comment, and `@ts-expect-error` assertions prove
  each refusal, so the brand cannot quietly stop working.
- **The boundary is applied once per feature API module**, on fields that are
  genuinely attacker-influenced: health condition reasons and messages, which a
  compromised or emulated Edge writes directly; device and source identifiers
  inside a projection; and display names, because the console cannot distinguish
  an operator's typing from an attacker with a stolen session. Values the
  backend validates into a shape it issued — UUIDs, CIDRs, timestamps, enum
  states — stay plain strings, so the brand keeps meaning "untrusted" rather
  than "string".
- **The renderer neutralises rather than sanitises.** Control characters,
  directional formatting, and invisible characters become visible escaped
  source. Nothing is normalised: normalising would change the evidence an
  operator is reading. Nothing is removed: a removed character is a character
  the operator never learns was there.
- **No component emits a URL-bearing attribute or an anchor from data.** A
  `javascript:` URL, a `data:` URL, and a filename all render identically: as
  characters. Copy is an explicit control that hands over the original
  untransformed value and says in its accessible name that the value is
  untrusted.
- **`test/check-unsafe-dom.mjs` runs on lint** and forbids
  `dangerouslySetInnerHTML`, `innerHTML`, `outerHTML`, `insertAdjacentHTML`,
  `document.write`, `eval`, `new Function`, and string-bodied timers across the
  whole console. Section 8.2 permits no suppression, so this is a repository
  check rather than an ESLint rule a directive could switch off line by line.
- **Two response headers were added** in the existing `securityHeaders`
  middleware and no existing header changed. `Permissions-Policy` denies 26
  features the console never uses, on every scheme.
  `Strict-Transport-Security` is emitted only when `request.TLS != nil`, so a
  plain-HTTP development listener cannot pin a browser to a scheme it does not
  serve. `preload` is deliberately absent: it is effectively irreversible for
  months and is therefore an owner decision, not a middleware default.
- **The component workbench cannot reach production.** It is mounted behind
  `import.meta.env.DEV`, which Vite replaces with `false` so Rollup drops the
  dynamic import; `check-bundle.mjs` fails on its marker or any corpus fixture
  id in a production chunk; and a browser scenario asks a production build for
  `/__components` and asserts it receives the ordinary SPA shell. Its styles
  live in a separate CSS module because CSS Modules does not tree-shake unused
  class rules, and the measured CSS size is byte-for-byte what it was before
  the workbench existed.
- **MSW is development-only.** Its postinstall is blocked by the repository
  script policy, which is correct: that script only fetches the browser service
  worker, and no service worker belongs in this repository.

## The permanent fixture corpus

24 inert fixtures under `@shared/hostile/corpus`, satisfying `RE-12`: script and
event-handler injection, nested markup that defeats a single-pass stripper,
`javascript:` and `data:` URLs, three ANSI sequence classes (colour, cursor
control, terminal title), a right-to-left override, a bidi isolate, zero-width
splitting, a 4 000-character unbreakable token, a null byte, CRLF injection,
Markdown link and image, path traversal, trailing-space and double-extension
filename hiding, a credential-shaped string, an unpaired surrogate, a template
expression, and style injection.

Every fixture is inert: no working exploit, no real credential, no third
party's hostname. The credential fixture is deliberately not shaped like any
real provider's key — one that tripped the repository secret scan would teach
people to ignore the scanner.

Every invisible character is built with `String.fromCodePoint` rather than
typed, and a test asserts the corpus file's own source contains no literal
invisible byte. A corpus nobody can review is not evidence of anything.

**The file only grows.** Every rendering defect found from here on adds a
fixture, so the same defect cannot return unnoticed.

## Abuse and failure cases reviewed

| Case | Control and evidence |
|---|---|
| Script or event-handler injection through a display name | Rendered as characters through `UntrustedText`; 24 fixtures × 2 components assert no forbidden element and no `on*` attribute is created. |
| ANSI sequence forging a transcript outcome | Escape characters render as `\x1b`; a test asserts both `denied` and `granted` survive a cursor-overwrite sequence and that no element acquires a `style` attribute. |
| Right-to-left override reversing a filename | The override renders as the escaped source `\u202e` and the value still ends in `gnp.exe`; asserted in component tests and end to end through the real API. |
| Zero-width characters hiding a keyword | Rendered as the escaped source `\u200b`, with an accessible description naming the character. |
| A value becoming a link or a request | No untrusted component emits an anchor or any of `href`, `src`, `srcset`, `action`, `formaction`, `poster`, `data`, `download`, `ping`, `background`. |
| A filename used as a path or a download target | No `download` attribute anywhere; filenames render as text only. Path traversal and double-extension fixtures are permanent. |
| An untrusted value reaching a surface that does not neutralise it | Typecheck failure. Four `@ts-expect-error` assertions prove JSX, attribute, and string assignment are all refused; string coercion is refused by type-aware lint rules, which the same test asserts are configured as errors. |
| Unbounded captured payload | 512 code points for a single-line value, 65 536 for a block, each stating the original length. Truncation is never silent, and the bound is in code points so a character is never split. |
| Malformed input crashing the renderer | An unpaired surrogate is escaped rather than thrown on; a permanent fixture covers it. |
| Clipboard silently failing | A failed write reports failure inline; silence would look like success. Copy hands over the original value and says so in its name. |
| A test passing against a console that sent no CSRF header | Found and fixed during this package: under jsdom's `Request` class MSW dropped every header the console set, so the `X-CSRF-Token` and `If-Match` assertions would have passed against a console that sent neither. The test environment now uses Node's `Request`, `Response`, and `Headers`. |
| An unmocked call mistaken for a healthy outcome | `onUnhandledRequest: 'error'` throws rather than returning something plausible. |
| Workbench or fixtures shipped to an operator | Three independent exclusion proofs; see boundaries above. |
| Downgrade to plain HTTP | `Strict-Transport-Security` for a year including subdomains, over TLS only. |
| Powerful browser feature reached from an injected frame | `Permissions-Policy` denies 26 features on every scheme. |

## Evidence commands

```text
task check
task web:e2e
go -C apps/control-plane test ./internal/api/
npm audit --omit=dev
```

## Known limitations and residual risk

- **Escaping is a rendering control, not a storage control.** The captured
  bytes remain in the database as they arrived, which is correct — they are
  evidence — but any future consumer that renders them outside this contract
  reintroduces the risk. The branded type is what prevents that inside the
  console; it cannot reach an export, a report, or a downstream tool.
- **The transformation is a denylist of character classes**, not an allowlist.
  A Unicode class that forges display and is not in the list would pass
  through. The corpus is the mitigation: every defect found adds a permanent
  fixture. An allowlist was considered and rejected because it would mangle
  legitimate international text an operator needs to read.
- **Length bounds are applied in code points, not bytes.** Section 9.1.5 states
  the block bound as 64 KiB; it is implemented as 65 536 code points so a
  truncation can never split a character and invent a replacement glyph that
  was not in the evidence. A worst-case all-astral payload is therefore larger
  in bytes than the stated bound.
- **`Permissions-Policy` feature names are not fully standardised.** Unknown
  names are ignored by browsers, so the header is a best-effort deny list that
  will need revisiting as the specification settles.
- **The browser scenarios were not run locally.** They require Docker, three
  browser engines, and generation tooling, and run in `full.yml`. The component
  and Control Plane suites were run.
- **`eslint-plugin-jsx-a11y` and MSW both declare stale peer ranges** against
  ESLint 10 and were installed with `--legacy-peer-deps`. `npm ci` reproduces
  the locked tree without the flag, so CI is unaffected.
