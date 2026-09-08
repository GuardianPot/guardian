# WCX-11 form stack and decoy console security review

- Review date: 2026-09-08
- Work package: WCX-11
- Decisions: WC-D21, WC-D03, WC-D16, WC-D15, UX-06, ON-05, DC-11, DC-12,
  OPS-02, SEC-06, SEC-08
- Scope: the console form and validation stack, the change-proposal-0004
  consumption path, the decoy list, the decoy detail screen, the decoy
  configuration form, and the deploy, enable, disable, and remove flows

The package is marked `requires_security_review`. Three things in it are
security behaviour rather than UX behaviour, and they are the reason.

## 1. Attacker-visible fields

A decoy's display name, address, and persona are shown to whoever probes it.
That is what a decoy is for, and it is exactly why those fields must never
carry anything real: an operator who names a decoy after a production host, or
gives it a real hostname, has handed an attacker a true fact about the network
Guardian is meant to be protecting.

The warning is therefore on the control, wired through `aria-describedby`, not
placed near it:

> Attackers can see this value. Never enter a real credential, a real hostname,
> or anything that identifies a real system.

It names all three of the things an operator is likely to reach for. A
page-level note would be read once and never again; an associated description
reaches a screen reader with the field and stays with it. `DecoysPage.test.tsx`
asserts the association for both the display name and the address by resolving
`aria-describedby` and matching the text, and the browser scenario repeats it
against a real Control Plane.

This wording is security-critical. It is in the `WCX-08` catalogue under
`decoys.attackerVisible.warning` so it is reviewable in one place, and it
should change through this review rather than as a copy edit.

## 2. Emulated personas (AC-SMB-002)

The console must never state or imply that a real Windows, database, or
application host exists. An operator who believes one does will reason — and
make decisions — about a machine that is not there.

Every persona therefore travels with an `Emulated` label in the list and in the
detail screen, and the configuration form carries the sentence "Every persona is
an emulation presented by Guardian. No real host of this kind is created."
Tests assert the label in the row and the sentence in the form.

The residual risk is wording drift: a future screen that renders
`decoy.persona` as a bare category would reintroduce exactly the implication
`AC-SMB-002` forbids. The catalogue keeps the label adjacent to the persona
entries so the pairing is visible to a reviewer.

## 3. Untrusted content (SEC-08)

Two fields cross the trust boundary and both are marked at it, in
`taintDecoyView`:

- **`display_name`** round-trips through the API. The console cannot tell an
  operator's typing from an attacker with a stolen session, so it does not try.
- **A condition `message`** is written by an Edge, which is the component
  sitting closest to whoever is probing the decoy.

`reason` is marked too. The contract bounds it to `^[a-z][a-z0-9_]{0,63}$`,
which is what makes it safe to display; marking it makes relying on that a
decision rather than an assumption.

Everything else in a decoy — family, persona, interaction level, desired and
observed state — is a closed token the console may render as a category.

Both hostile-content paths are tested end to end: a hostile display name in the
list, and a hostile condition message on the detail screen, each asserted to
produce no element and to appear as text.

## 4. The honesty controls

These are security controls in this product, because misreporting coverage is
the failure mode a deception product has to avoid.

| Control | Where it is enforced |
|---|---|
| A decoy nothing reported on reads `unknown`, never deployed or absent | `decoyObservedEncoding`, asserted in the list and detail tests |
| A missing interaction reads `Unknown`, never `Never` | `DecoyTable`, asserted directly |
| Convergence past the window is `overdue`, not failed | `convergenceOf`, asserted with an explicit "not a failure" assertion |
| `unmanaged` is asked no convergence question | `convergenceOf` returns early; asserted for a revoked device's decoy |
| A revoked device's decoy stays visible and marked | list test, SEC-06 |
| A failed transition changes no displayed state | lifecycle test, section 9.9.5 |
| No optimistic update anywhere | `useConsoleForm` submits pessimistically |

`overdue` deserves a note. It reports how long Guardian has been waiting and
says explicitly that it has not been told anything failed. The temptation is to
render a timeout, and section 9.5 forbids it: an absence of news is not news of
a failure, and a console that invents one teaches an operator to distrust the
states that are real.

## 5. Client validation is not a control

Section 8.1, and worth stating because it is easy to forget once a form
validates well. The resolver catches typos before a round trip. It is not an
authorization decision, not an input-sanitisation boundary, and not a guarantee
about anything: the Control Plane validates every field again, and a rejection
it returns is rendered truthfully even when the client believed the value good.

Two existing tests submitted malformed values expecting a backend rejection.
Once the validators were derived from the contract the console caught those
first, which is correct behaviour and made the tests stop testing section 8.1.
They now submit values the client accepts and the Control Plane refuses — a
zone CIDR that overlaps, an address outside its zone — because only the Control
Plane can know either.

The derived-constraint mechanism is itself a control against a subtler failure:
a hand-copied bound drifts from the contract, and a drifted bound accepts input
the backend refuses, which trains operators to distrust the form.

## 6. Nothing is persisted

Section 8.6. `UnsavedChangesGuard` warns on navigation and saves nothing; there
is deliberately no draft to restore. A half-configured decoy holds an
attacker-visible persona and address, and writing those to `localStorage` would
leave them on the machine after the tab closed, for a convenience nobody asked
for. Tests assert `localStorage`, `sessionStorage`, and `document.cookie` stay
empty while a form is dirty, on the stack and on both decoy screens.

The browser prompt on tab close cannot be worded or styled — `beforeunload` is
the only hook and browsers ignore custom text. That is the platform's limit,
recorded so nobody mistakes it for a choice.

## 7. What the migration nearly broke

Three regressions the form-stack migration introduced and this review is the
reason they were looked for.

**The sign-in proof.** The proof control is remounted on `key={method.value}`
so an authenticator code never travels into a recovery-code submission. React
Hook Form keeps a registered field's value across a remount, so the input would
have looked empty while the form state still held the code — and the form state
is what gets sent. The value the stack holds is now reset when the method
changes. This was a real credential-confusion path, introduced and closed
inside this package.

**Focus management.** `TextField` was letting the form library's ref replace
the screen's `inputRef`. The control still worked and the empty-state
affordance that sends focus to the first field silently stopped moving
anywhere. Both refs are composed now.

**A stripped field.** Valibot's `object()` drops keys the schema does not
declare, so the sign-in proof was being validated away before submission. It
surfaced as a failing test rather than as a silent credential loss, but it is
the same class: the validated object, not the DOM, is what is sent.

## Deliberately not covered

- **An Edge reporting `degraded` with its own reason.** The browser harness
  runs a Control Plane and no Edge, so every decoy in it reads `unknown` — the
  state section 8.7 cares most about, and the one asserted. The reported-failure
  half needs `P2-W3`'s runtime.
- **Decoy secrets and synthetic credentials.** `P2-W9` owns them; no decoy
  surface here displays or holds one, and the one-time handling rules for
  enrollment secrets are unchanged.
- **Backend authorization.** Capabilities gate whether a control renders as
  available and are never a security authority; the Control Plane refuses
  regardless, and the console renders that refusal.

## Conclusion

No new privilege, credential path, or network authority. The new surface is
what the console *says*: about coverage, and about which values an attacker
will read. Both are handled by construction rather than by care — closed
vocabularies and derived constraints for the second, and a rendering layer with
no path from "requested" to "observed" for the first.

The residual risk is wording. Every control in sections 1 and 2 is a sentence,
and a sentence can be edited by someone who does not know it is load-bearing.
That is why they live in the reviewed catalogue rather than in components.
