import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import process from 'node:process';
import { spawn } from 'node:child_process';
import { currentHealthTransitionTime, hostileHealthMessage, publishHealth } from './device-health';

/**
 * A hostile display name that the Control Plane accepts (WCX-06 section 10.3).
 *
 * `NormalizeName` rejects control characters, so an ANSI sequence never
 * reaches a name. It does *not* reject markup, a right-to-left override, or a
 * zero-width space — those are format characters, not control characters — so
 * these are the classes that genuinely travel the real API into the console
 * and the ones the browser has to render inertly.
 *
 * The override reverses everything after it, so an unprotected renderer shows
 * `gnp.exe` as `exe.png`. Built from code points rather than typed, so this
 * file stays reviewable.
 */
const RLO = String.fromCodePoint(0x202e);
const ZWSP = String.fromCodePoint(0x200b);
const hostileDisplayName = `Lab <img src=x onerror=alert(1)> ${RLO}gnp.exe pass${ZWSP}word`;

/** Fails on any serious or critical finding, the WCX-05 threshold. */
async function expectNoSeriousAxeViolations(page: Page, where: string) {
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter((item) => ['critical', 'serious'].includes(item.impact ?? ''));
  expect(serious.map((item) => `${item.id}: ${item.help}`), `axe on ${where}`).toEqual([]);
}

/**
 * Walks the tab order from the top of the document and returns what it reached.
 *
 * Bounded so a broken focus trap fails the test instead of hanging it.
 */
async function tabOrder(page: Page, steps = 60): Promise<string[]> {
  await page.evaluate(() => { (document.activeElement as HTMLElement | null)?.blur(); });
  const reached: string[] = [];
  for (let step = 0; step < steps; step += 1) {
    await page.keyboard.press('Tab');
    const name = await page.evaluate(() => {
      const active = document.activeElement;
      if (!active || active === document.body) return null;
      return (active.textContent ?? '').trim() || active.getAttribute('aria-label') || active.tagName;
    });
    if (name === null) break;
    if (reached.includes(name) && reached[0] === name) break;
    reached.push(name);
  }
  return reached;
}

test('real owner onboarding, Edge enrollment, health degradation, and recovery', async ({ page, context }, testInfo) => {
  const projectIndex = ['chromium', 'firefox', 'webkit'].indexOf(testInfo.project.name);
  expect(projectIndex).toBeGreaterThanOrEqual(0);
  // Any dialog would mean injected backend text executed rather than rendered.
  const dialogs: string[] = [];
  page.on('dialog', (dialog) => { dialogs.push(dialog.message()); void dialog.dismiss(); });
  const recoveryCodes = JSON.parse(required('GUARDIAN_E2E_RECOVERY_CODES')) as string[];
  const fixtureDirectory = required('GUARDIAN_E2E_FIXTURE_DIR');
  const identityDirectory = join(fixtureDirectory, `identity-${testInfo.project.name}`);
  const edgeDirectory = join(fixtureDirectory, `edge-${testInfo.project.name}`);
  const containerRuntime = required('GUARDIAN_E2E_EDGE_RUNTIME') === 'container';
  const runtimeIdentityDirectory = containerRuntime ? `/fixture/identity-${testInfo.project.name}` : identityDirectory;
  const runtimeEdgeDirectory = containerRuntime ? `/fixture/edge-${testInfo.project.name}` : edgeDirectory;
  const runtimeConfig = join(runtimeEdgeDirectory, 'edge.json').replaceAll('\\', '/');
  await mkdir(identityDirectory, { recursive: true, mode: 0o700 });
  await mkdir(join(edgeDirectory, 'spool'), { recursive: true, mode: 0o700 });
  const edgeConfig = join(edgeDirectory, 'edge.json');
  await writeFile(edgeConfig, JSON.stringify({
    control_plane_endpoint: required('GUARDIAN_E2E_EDGE_CONTROL_PLANE'),
    device_channel_endpoint: required('GUARDIAN_E2E_EDGE_DEVICE_CHANNEL'),
    database_path: join(runtimeEdgeDirectory, 'edge.db').replaceAll('\\', '/'),
    spool_directory: join(runtimeEdgeDirectory, 'spool').replaceAll('\\', '/'),
    spool_capacity_bytes: 67_108_864,
    identity_certificate_path: join(runtimeIdentityDirectory, 'device.crt').replaceAll('\\', '/'),
    identity_private_key_path: join(runtimeIdentityDirectory, 'device.key').replaceAll('\\', '/'),
    shutdown_timeout_seconds: 5,
    log_level: 'error',
  }), { mode: 0o600 });

  await page.goto('/login');
  // WCX-06 section 10.3.3: the axe scan covers every route, not only the last
  // screen the flow happens to end on.
  await expectNoSeriousAxeViolations(page, 'the sign-in route');
  await signIn(page, recoveryCodes[projectIndex * 2]);
  await expectNoSeriousAxeViolations(page, 'the environments route');
  const environmentName = `Browser ${testInfo.project.name}`;
  await page.getByLabel('Display name').fill(environmentName);
  await page.getByRole('button', { name: 'Create environment' }).click();
  await page.getByRole('link', { name: new RegExp(environmentName) }).click();
  await expectNoSeriousAxeViolations(page, 'the environment route');

  await page.reload();
  await expect(page.getByText('Read-only session restored.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Create one-time secret' })).toBeDisabled();
  await page.getByRole('link', { name: 'Re-authenticate' }).first().click();
  await signIn(page, recoveryCodes[projectIndex * 2 + 1]);
  await page.getByRole('link', { name: new RegExp(environmentName) }).click();
  // Captured now, because the display name is replaced with a hostile
  // fixture later in this flow and the link can no longer be found by name.
  const environmentPath = new URL(page.url()).pathname;
  expect(environmentPath).toMatch(/^\/environments\/[0-9a-f-]{36}$/);

  const csrfFailure = await page.evaluate(async () => (await fetch('/v1/environments', {
    method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'invalid' }, body: '{"display_name":"denied"}',
  })).status);
  expect(csrfFailure).toBe(401);

  await page.getByLabel('Zone name').fill(`Zone ${testInfo.project.name}`);
  await page.getByLabel('Private CIDR').fill(`10.${20 + projectIndex}.0.0/24`);
  const zoneResponsePromise = page.waitForResponse((response) =>
    response.request().method() === 'POST' && /\/v1\/environments\/[^/]+\/zones$/.test(new URL(response.url()).pathname));
  await page.getByRole('button', { name: 'Add zone' }).click();
  const zoneResponse = await zoneResponsePromise;
  expect(zoneResponse.status(), await zoneResponse.text()).toBe(201);
  await expect(page.getByText('Private network zone added.')).toBeVisible();

  expect(await runEdgeEnrollment(edgeConfig, runtimeConfig, Buffer.from('invalid\n'))).not.toBe(0);
  await page.getByLabel('Device name').fill(`edge-${testInfo.project.name}`);
  await page.getByRole('button', { name: 'Create one-time secret' }).click();
  const secretLocator = page.getByTestId('one-time-secret').locator('code');
  const secretText = await secretLocator.textContent();
  expect(secretText).toMatch(/^[A-Za-z0-9_-]{43}$/);
  const enrollmentInput = Buffer.from(`${secretText}\n`);
  expect(await runEdgeEnrollment(edgeConfig, runtimeConfig, enrollmentInput)).toBe(0);
  enrollmentInput.fill(0);
  await page.getByRole('button', { name: 'I have stored it securely' }).click();
  await expect(page.getByTestId('one-time-secret')).toHaveCount(0);
  await expect(page).not.toHaveURL(new RegExp(secretText!));

  // WCX-06 section 10.3.2 and WCX-10 sections 10.2.1, 10.2.2, and 10.2.6:
  // every control is reachable by keyboard alone at every supported width.
  //
  // Sign-out is the one that matters — an operator who cannot reach it cannot
  // end a session they suspect is stolen — and it was unreachable below 900
  // pixels until `P1-W11` GAP-1 was fixed. `WCX-10` replaced that fix with a
  // disclosure, so below the breakpoint "reachable" now means reachable
  // *through* the disclosure, by keyboard, without a pointer.
  for (const viewport of [
    { width: 1440, height: 900 },
    { width: 375, height: 812 },
    { width: 320, height: 568 },
  ]) {
    const at = `${viewport.width}px`;
    await page.setViewportSize(viewport);

    const disclosure = page.getByRole('button', { name: 'Open navigation' });
    if (viewport.width <= 900) {
      await expect(disclosure, `a disclosure at ${at}`).toBeVisible();
      await expect(disclosure).toHaveAttribute('aria-expanded', 'false');
      await disclosure.click();
      await expect(disclosure).toHaveAttribute('aria-expanded', 'true');
    } else {
      await expect(disclosure, `no disclosure at ${at}`).toHaveCount(0);
    }

    const nav = page.getByRole('navigation', { name: 'Primary navigation' });
    for (const entry of ['Home', 'Environments', 'Account']) {
      await expect(nav.getByRole('link', { name: entry }), `${entry} at ${at}`).toBeVisible();
    }
    await expect(page.getByRole('button', { name: 'Sign out' }), `sign-out at ${at}`).toBeVisible();
    await expect(page.getByLabel('Environment scope'), `scope at ${at}`).toBeVisible();

    const reached = await tabOrder(page);
    expect(reached[0], `first tab stop at ${at}`).toBe('Skip to content');
    expect(reached, `sign-out in the tab order at ${at}`).toContain('Sign out');

    // Section 9.3.6: no horizontal page scroll at any supported width. Wide
    // content scrolls inside its own container; the page never does.
    const overflow = await page.evaluate(() =>
      document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow, `horizontal page overflow at ${at}`).toBeLessThanOrEqual(0);

    await expectNoSeriousAxeViolations(page, `the environment route at ${at}`);
  }
  await page.setViewportSize({ width: 1280, height: 800 });

  // WCX-06 section 10.3.1: an attacker-shaped display name, written through
  // the real API and read back through the real console. Markup, a
  // right-to-left override, and a zero-width space all survive the backend —
  // it rejects control characters, not these — so this is the class that
  // genuinely arrives.
  await page.getByLabel('Display name').fill(hostileDisplayName);
  await page.getByRole('button', { name: 'Save name' }).click();
  await expect(page.getByText('Environment name updated.')).toBeVisible();

  const heading = page.getByRole('heading', { level: 1 });
  // The markup is characters, not elements, and the override is shown as its
  // escaped source rather than obeyed, so `.exe` is still the ending.
  await expect(heading).toContainText('<img src=x onerror=alert(1)>');
  await expect(heading).toContainText('\\u202e');
  await expect(heading).toContainText('\\u200b');
  expect(await heading.locator('img, svg, script, iframe, object, embed, a').count()).toBe(0);
  expect(dialogs).toEqual([]);

  const healthySince = currentHealthTransitionTime();
  let connection = await publishHealth(identityDirectory, 1, healthySince);
  const deviceLink = page.getByRole('link', { name: new RegExp(`edge-${testInfo.project.name}.*active`, 's') });
  await expect(deviceLink).toBeVisible();
  await deviceLink.click();
  await expect(page.getByRole('heading', { name: 'Eight-condition health' })).toBeVisible();
  await expect(page.getByText('Healthy', { exact: true }).first()).toBeVisible();
  await expect(page.getByRole('list', { name: 'Device health conditions' }).getByRole('listitem')).toHaveCount(8);

  connection.close();
  await expect(page.getByText('Action required', { exact: true }).first()).toBeVisible({ timeout: 20_000 });
  await new Promise((resolve) => setTimeout(resolve, 100));
  connection = await publishHealth(identityDirectory, 2, healthySince);
  await expect(page.getByText('Healthy', { exact: true }).first()).toBeVisible({ timeout: 20_000 });

  expect(await page.evaluate(async () => ({
    local: localStorage.length,
    session: sessionStorage.length,
    databases: typeof indexedDB.databases === 'function' ? (await indexedDB.databases()).length : 0,
    reduced: matchMedia('(prefers-reduced-motion: reduce)').matches,
  }))).toEqual({ local: 0, session: 0, databases: 0, reduced: true });
  const accessibility = await new AxeBuilder({ page }).analyze();
  expect(accessibility.violations.filter((item) => ['critical', 'serious'].includes(item.impact ?? ''))).toEqual([]);

  const screenshotDirectory = join(required('GUARDIAN_E2E_OUTPUT_DIR'), 'screenshots');
  await mkdir(screenshotDirectory, { recursive: true });
  await page.screenshot({ path: join(screenshotDirectory, `onboarding-complete-${testInfo.project.name}.png`), fullPage: true });

  // Hostile backend text travels the real device channel, Control Plane
  // persistence, and API before the browser renders it.
  connection.close();
  await expect(page.getByText('Action required', { exact: true }).first()).toBeVisible({ timeout: 20_000 });
  await new Promise((resolve) => setTimeout(resolve, 100));
  connection = await publishHealth(identityDirectory, 3, healthySince, {
    type: 'HEALTH_CONDITION_TYPE_SPOOL_HEALTHY',
    status: 'HEALTH_CONDITION_STATUS_FALSE',
    reason: 'capacity_critical',
    message: hostileHealthMessage,
    lastTransitionTime: currentHealthTransitionTime(),
  });
  const conditionList = page.getByRole('list', { name: 'Device health conditions' });
  await expect(conditionList.getByText(hostileHealthMessage, { exact: true })).toBeVisible({ timeout: 20_000 });
  expect(await conditionList.locator('img, svg, script, iframe, object, embed').count()).toBe(0);
  await expect(page.getByText(/Blocking: Event spool/)).toHaveText(/capacity_critical/);
  expect(dialogs).toEqual([]);
  connection.close();

  // ── WCX-09 section 10.3.1 and 10.3.3 ────────────────────────────────────
  //
  // Attached to this flow rather than to a file of its own because both need
  // what it already built: a live environment, an enrolled Edge, and a zone.
  // Recreating those to test a revoke would double the most expensive part of
  // the suite.
  //
  // Level 3 actions are not exercised here. Each one costs a recovery code and
  // bootstrap issues ten, of which this flow already spends two per engine;
  // `operator-lifecycle.spec.ts` runs the step-up gate once, on one engine,
  // out of what is left. Section 10.3.2's revoke and 10.3.2a's re-enrollment
  // are therefore covered by `DeviceLifecycle.test.tsx` against a mocked
  // Control Plane and are recorded as a browser-coverage gap.
  await page.goto(environmentPath);

  // The token this flow created is listed as used, and its value is nowhere.
  const tokenTable = page.getByRole('table', { name: /Enrollment tokens for this environment/ });
  await expect(tokenTable).toBeVisible();
  await expect(tokenTable.getByText('Used for enrollment')).toBeVisible();
  expect(await page.content(), 'no token value may appear in the document').not.toContain(secretText!);

  // A second token, listed as open, then revoked at level 2 — no step-up, so
  // no recovery code. The device must then be unable to enrol with it.
  const spareDevice = `spare-${testInfo.project.name}`;
  await page.getByLabel('Device name').fill(spareDevice);
  await page.getByRole('button', { name: 'Create one-time secret' }).click();
  const spareSecret = await page.getByTestId('one-time-secret').locator('code').textContent();
  expect(spareSecret).toMatch(/^[A-Za-z0-9_-]{43}$/);
  await page.getByRole('button', { name: 'I have stored it securely' }).click();

  const revokeSpare = page.getByRole('button', { name: `Revoke the token for ${spareDevice}` });
  await expect(revokeSpare).toBeEnabled();
  await revokeSpare.click();
  await page.getByRole('dialog').getByRole('button', { name: 'Revoke enrollment token' }).click();
  await expect(revokeSpare).toBeDisabled();

  // The revocation is real, not presentational: the Edge is refused.
  const revokedInput = Buffer.from(`${spareSecret}\n`);
  expect(await runEdgeEnrollment(edgeConfig, runtimeConfig, revokedInput)).not.toBe(0);
  revokedInput.fill(0);

  // Zone rename and delete (10.3.3), both under optimistic concurrency. Level
  // 1 and level 2, so neither costs a recovery code.
  const zoneName = `Zone ${testInfo.project.name}`;
  const zoneRow = page.getByRole('listitem').filter({ hasText: zoneName });
  await zoneRow.getByRole('button', { name: `Edit ${zoneName}` }).click();
  const editedName = `${zoneName} edited`;
  await zoneRow.getByLabel('Zone name').fill(editedName);
  await zoneRow.getByRole('button', { name: 'Save zone' }).click();
  await expect(page.getByText(editedName)).toBeVisible();

  const editedRow = page.getByRole('listitem').filter({ hasText: editedName });
  await editedRow.getByRole('button', { name: `Delete ${editedName}` }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete zone' }).click();
  await expect(page.getByText(editedName)).toHaveCount(0);

  // ── WCX-10 sections 10.2.3 and 10.2.4 ───────────────────────────────────
  //
  // A scoped link resolves the same way for whoever opens it, and a scope the
  // console cannot resolve never falls back to one it can.
  const environmentId = environmentPath.split('/').pop()!;
  await page.goto(`/?env=${environmentId}`);
  await expect(page.getByRole('heading', { name: 'Current environment' })).toBeVisible();
  await expect(page.getByLabel('Environment scope')).toHaveValue(environmentId);

  /*
   * The same address in a fresh document: a pasted link resolves to the same
   * view, which is the whole reason `WC-D14` puts scope in the URL.
   *
   * A new tab in this context rather than a second browser context, because a
   * second context needs its own sign-in and every sign-in costs one of the
   * ten one-time recovery codes bootstrap issues — six are spent by this flow
   * and two by `operator-lifecycle.spec.ts`. A new tab is also the case an
   * operator actually hits: there is one owner, so a shared link is opened by
   * the same session.
   */
  const sharedPage = await context.newPage();
  try {
    await sharedPage.goto(`/?env=${environmentId}`);
    await expect(sharedPage.getByLabel('Environment scope')).toHaveValue(environmentId);
    await expect(sharedPage.getByRole('heading', { name: 'Current environment' })).toBeVisible();
  } finally {
    await sharedPage.close();
  }

  // A well-formed identifier for an environment that does not exist. There is
  // one owner and no role model (`IA-06`), so a *denied* environment is not
  // reachable in this deployment; `scope.test.tsx` covers the 403 against a
  // mocked Control Plane. What matters equally here is the refusal to
  // substitute: the environment this session can read is not shown.
  await page.goto('/?env=018f1f7e-0000-7000-8000-0000000000ff');
  await expect(page.getByRole('heading', { name: 'Current environment' })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Current environment' })).not.toContainText(environmentName);

  // A malformed one is not looked up at all, and says so.
  await page.goto('/?env=../../admin');
  await expect(page.getByRole('heading', { name: 'Page not found', level: 1 })).toBeVisible();
  await expect(page.getByText(/deliberately not fallen back to a different environment/)).toBeVisible();
  // Navigation stays mounted, so the operator is not stranded (section 9.7.3).
  await expect(page.getByRole('navigation', { name: 'Primary navigation' })).toBeVisible();
  await expectNoSeriousAxeViolations(page, 'the not-found route');

  expect(dialogs).toEqual([]);

  await context.clearCookies();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
});

/**
 * Everything that can be checked without a session (WCX-06 sections 10.3.4 and
 * 10.3.5).
 *
 * Kept separate from the onboarding flow because neither claim depends on a
 * signed-in operator, and a failure here should name the header or the
 * workbench rather than be buried in a twelve-step journey.
 */
test('browser security headers and a production build without the workbench', async ({ page }) => {
  const response = await page.request.get('/livez');
  expect(response.status()).toBe(200);
  const headers = response.headers();

  // Served over TLS, so the pin is expected. `preload` is deliberately absent:
  // it is effectively irreversible and therefore an owner decision.
  expect(headers['strict-transport-security']).toBe('max-age=31536000; includeSubDomains');
  expect(headers['strict-transport-security']).not.toContain('preload');

  for (const feature of ['camera=()', 'microphone=()', 'geolocation=()', 'payment=()', 'usb=()', 'serial=()']) {
    expect(headers['permissions-policy'], feature).toContain(feature);
  }

  // The headers that were already there must be untouched.
  expect(headers['content-security-policy']).toContain("frame-ancestors 'none'");
  expect(headers['content-security-policy']).not.toContain('unsafe-inline');
  expect(headers['content-security-policy']).not.toContain('unsafe-eval');
  expect(headers['referrer-policy']).toBe('no-referrer');
  expect(headers['x-content-type-options']).toBe('nosniff');

  // The third exclusion proof for the workbench: this is a production build,
  // served from `apps/web-console/dist`. The route must fall through to the
  // ordinary SPA shell, and no chunk may carry the workbench or its fixtures.
  const workbench = await page.request.get('/__components');
  expect(workbench.status()).toBe(200);
  const shell = await workbench.text();
  expect(shell).toContain('<div id="root">');
  expect(shell).not.toContain('guardian-component-workbench');
  expect(shell).not.toContain('Component workbench');

  await page.goto('/__components');
  await expect(page.getByRole('heading', { name: 'Component workbench' })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
});

async function signIn(page: import('@playwright/test').Page, recoveryCode: string) {
  await page.getByLabel('Username').fill(required('GUARDIAN_E2E_USERNAME'));
  await page.getByLabel('Password').fill(required('GUARDIAN_E2E_PASSWORD'));
  await page.getByRole('button', { name: 'Recovery code' }).click();
  await page.getByLabel('Recovery code', { exact: true }).fill(recoveryCode);
  await page.getByRole('button', { name: 'Continue securely' }).click();
  await expect(page.getByRole('heading', { name: 'Environments', exact: true })).toBeVisible();
}

function runEdgeEnrollment(configPath: string, runtimeConfigPath: string, input: Buffer) {
  return new Promise<number>((resolve, reject) => {
    const containerRuntime = required('GUARDIAN_E2E_EDGE_RUNTIME') === 'container';
    const command = containerRuntime ? 'docker' : required('GUARDIAN_E2E_EDGE_BIN');
    const args = containerRuntime ? [
      'run', '--rm', '--interactive', '--add-host', 'host.docker.internal:host-gateway',
      '--env', 'SSL_CERT_FILE=/fixture/server-ca.crt',
      '--mount', `type=bind,src=${required('GUARDIAN_E2E_FIXTURE_DIR')},dst=/fixture`,
      'golang:1.27-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466',
      '/fixture/guardian-edge', 'enroll', '--config', runtimeConfigPath,
    ] : ['enroll', '--config', configPath];
    const child = spawn(command, args, {
      env: { ...process.env, ...(containerRuntime ? {} : { SSL_CERT_FILE: required('GUARDIAN_E2E_TLS_CA') }) },
      stdio: ['pipe', 'ignore', 'ignore'],
    });
    child.once('error', reject);
    child.once('exit', (code) => resolve(code ?? -1));
    child.stdin.end(input);
  });
}

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}
