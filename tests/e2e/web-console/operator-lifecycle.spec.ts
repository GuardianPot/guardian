import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import process from 'node:process';

/**
 * The step-up gate in a real browser (WCX-09 sections 10.3.6-8).
 *
 * Separate from `onboarding.spec.ts` because none of this needs an enrolled
 * Edge, and a failure here should name the gate rather than be buried in a
 * twelve-step enrollment journey.
 *
 * **Chromium only, and the reason is the fixture.** Bootstrap issues exactly
 * ten recovery codes and the onboarding flow spends six of them — two per
 * engine, because a sign-in consumes one and cannot be repeated. Every
 * step-up consumes another. TOTP is not an alternative: the Control Plane
 * stores the last accepted counter, so two proofs inside one thirty-second
 * window are rejected by design, and the harness zeroes the seed at bootstrap
 * rather than exporting it.
 *
 * So four codes remain for three engines and this test needs two. Running it
 * on one engine leaves two spare; running it on three would need six. What is
 * lost is cross-engine coverage of one gate, and that is the cheapest thing
 * on the list to lose — the screens themselves are scanned by axe on every
 * engine through the shared routes, and `AccountPage.test.tsx` drives the
 * whole gate against a mocked Control Plane.
 */
const CODE_OFFSET = 6;

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

/** The two codes reserved for this test, after the onboarding flow's six. */
function spareCodes(): [string, string] {
  const all = JSON.parse(required('GUARDIAN_E2E_RECOVERY_CODES')) as string[];
  const [first, second] = all.slice(CODE_OFFSET, CODE_OFFSET + 2);
  if (!first || !second) throw new Error('bootstrap did not supply two spare recovery codes');
  return [first, second];
}

async function signIn(page: Page, recoveryCode: string) {
  await page.getByLabel('Username').fill(required('GUARDIAN_E2E_USERNAME'));
  await page.getByLabel('Password').fill(required('GUARDIAN_E2E_PASSWORD'));
  await page.getByRole('button', { name: 'Recovery code' }).click();
  await page.getByLabel('Recovery code', { exact: true }).fill(recoveryCode);
  await page.getByRole('button', { name: 'Continue securely' }).click();
  await expect(page.getByRole('heading', { name: 'Environments', exact: true })).toBeVisible();
}

async function expectNoSeriousAxeViolations(page: Page, where: string) {
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter((item) => ['critical', 'serious'].includes(item.impact ?? ''));
  expect(serious.map((item) => `${item.id}: ${item.help}`), `axe on ${where}`).toEqual([]);
}

test('an irreversible action is gated by step-up, and a refused one changes nothing', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium', 'one engine: recovery codes are a bounded fixture, see the file comment');

  const [signInCode, stepUpCode] = spareCodes();
  // Any dialog would mean something executed rather than rendered.
  const dialogs: string[] = [];
  page.on('dialog', (dialog) => { dialogs.push(dialog.message()); void dialog.dismiss(); });

  // Every request to the password endpoint, so "issued nothing" is measured
  // at the network rather than inferred from the screen.
  const passwordRequests: string[] = [];
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/v1/auth/password') passwordRequests.push(request.method());
  });

  await page.goto('/login');
  await signIn(page, signInCode);

  await page.getByRole('link', { name: 'Account' }).click();
  await expect(page.getByRole('heading', { name: 'Account', exact: true })).toBeVisible();
  await expect(page.getByRole('table', { name: /Guardian sessions for this owner/ })).toBeVisible();
  // Section 10.3.8.
  await expectNoSeriousAxeViolations(page, 'the account route');

  // Section 9.5.3: this session is labelled and has no revoke control. The id
  // comes from the Control Plane, so the assertion is about the real session
  // rather than about whichever row the console happened to mark.
  const sessionId = await page.evaluate(async () =>
    ((await (await fetch('/v1/auth/session', { credentials: 'include' })).json()) as {
      session: { session_id: string };
    }).session.session_id);
  await expect(page.getByText('This session')).toBeVisible();
  await expect(page.getByRole('button', { name: `Revoke session ${sessionId}` })).toHaveCount(0);
  await expect(page.getByText('Use Sign out to end this session.')).toBeVisible();

  // Section 10.3.6: a level 3 action opens the step-up prompt first. The
  // confirmation body — the typed object name — must not be reachable yet.
  await page.getByLabel('Current password').fill(required('GUARDIAN_E2E_PASSWORD'));
  await page.getByLabel('New password').fill('a replacement that is long enough');
  await page.getByRole('button', { name: 'Change password' }).click();

  const stepUp = page.getByRole('dialog', { name: 'Confirm it is you' });
  await expect(stepUp).toBeVisible();
  await expect(page.getByLabel('Object name')).toHaveCount(0);
  await expectNoSeriousAxeViolations(page, 'the step-up prompt');

  // Section 10.3.7: a refused step-up leaves the object unchanged and issues
  // nothing. A wrong recovery code is refused the same way a wrong password
  // is, and the console must not say which.
  await stepUp.getByLabel('Username').fill(required('GUARDIAN_E2E_USERNAME'));
  await stepUp.getByLabel('Password').fill(required('GUARDIAN_E2E_PASSWORD'));
  await stepUp.getByRole('button', { name: 'Recovery code' }).click();
  await stepUp.getByLabel('Recovery code', { exact: true }).fill('not-a-real-recovery-code');
  await stepUp.getByRole('button', { name: 'Reauthenticate' }).click();

  await expect(stepUp.getByText('Sign-in was denied. Check your credentials and MFA proof.')).toBeVisible();
  // The refusal reflects no submitted value.
  await expect(stepUp).not.toContainText('not-a-real-recovery-code');
  await expect(page.getByLabel('Object name')).toHaveCount(0);
  expect(passwordRequests, 'a refused step-up must issue no request').toEqual([]);

  // The real proof cancels cleanly, and the action is still not attempted.
  await stepUp.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByText('Reauthentication was not completed, so nothing was changed.')).toBeVisible();
  expect(passwordRequests).toEqual([]);

  // The password is unchanged, which is the only check that does not rely on
  // the console's own account of itself.
  await page.getByRole('button', { name: 'Sign out' }).click();
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
  await signIn(page, stepUpCode);

  expect(dialogs).toEqual([]);
  expect(await page.evaluate(async () => ({
    local: localStorage.length,
    session: sessionStorage.length,
    databases: typeof indexedDB.databases === 'function' ? (await indexedDB.databases()).length : 0,
  }))).toEqual({ local: 0, session: 0, databases: 0 });
});

/**
 * Two scenarios from section 10.3 are deliberately not automated here, and
 * both for the same reason: they would damage state the run shares.
 *
 * **10.3.4, revoking another session.** Three engines run against one owner
 * account, so "the other session" is another engine's. Revoking by a captured
 * id would be safe, but it costs two more recovery codes than the fixture has.
 * Covered by `AccountPage.test.tsx`, which asserts the request targets one id
 * and that the current session is untouched.
 *
 * **10.3.5, changing the password.** Success would invalidate
 * `GUARDIAN_E2E_PASSWORD` for the engines running beside this one and revoke
 * every session for the owner. A test that breaks the fixture it shares is
 * worse than no test. The gate above proves nothing is sent without a
 * completed step-up; `AccountPage.test.tsx` drives the success and rejection
 * paths and asserts the reported outcome claims nothing the response did not;
 * and `internal/storage/auth.go` covers what the backend actually revokes.
 */
