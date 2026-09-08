import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import process from 'node:process';

/**
 * Decoy management in a real browser (WCX-11 section 10.2).
 *
 * This carries the Phase 2 exit-gate evidence: **four decoy families visible in
 * the Web Console**, deployed through the console's own form against a real
 * Control Plane.
 *
 * **Chromium only, and the reason is the recovery-code fixture.** Bootstrap
 * issues ten codes; `onboarding.spec.ts` spends six, two per engine, and
 * `operator-lifecycle.spec.ts` spends two. Two remain, and a sign-in consumes
 * one and cannot be repeated. Running this on three engines would need three.
 * What is lost is cross-engine coverage of these screens, and it is bought back
 * cheaply: axe runs here, and `DecoysPage.test.tsx` and `DecoyDetailPage.test.tsx`
 * drive every state against a mocked Control Plane on every run.
 *
 * **What no Edge means.** This harness runs a Control Plane and no Edge, so
 * nothing ever reports on a decoy. That is not a gap in the evidence — it is
 * the most important state to get right, and section 8.7 turns on it: every
 * decoy here must read `Unknown`, and the console must never present one as
 * deployed because somebody asked for it. The other half, an Edge reporting
 * `degraded` with its own reason, needs `P2-W3`'s runtime and belongs to the
 * package that builds it.
 */
const CODE_OFFSET = 8;

const FAMILIES = [
  { family: 'ssh', persona: 'linux_admin_server', pack: 'ssh-cowrie', address: '10.60.0.11' },
  { family: 'http', persona: 'internal_admin_web_app', pack: 'http-admin', address: '10.60.0.12' },
  { family: 'postgres', persona: 'database_server', pack: 'postgres-service', address: '10.60.0.13' },
  { family: 'smb', persona: 'windows_file_service_host', pack: 'smb-fileshare', address: '10.60.0.14' },
] as const;

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function spareCode(): string {
  const all = JSON.parse(required('GUARDIAN_E2E_RECOVERY_CODES')) as string[];
  const code = all[CODE_OFFSET];
  if (!code) throw new Error('bootstrap did not supply a spare recovery code');
  return code;
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

/** Fills and submits the decoy form. The pack fields carry no default. */
async function fillDecoy(
  page: Page,
  entry: { family: string; persona: string; pack: string; address: string },
  name: string,
) {
  await page.getByLabel('Display name').fill(name);
  await page.getByLabel('Type', { exact: true }).selectOption(entry.family);
  await page.getByLabel('Persona', { exact: true }).selectOption(entry.persona);
  await page.getByLabel('Address', { exact: true }).fill(entry.address);
  await page.getByLabel('Pack', { exact: true }).fill(entry.pack);
  await page.getByLabel('Pack version').fill('0.1.0');
}

test('four decoy families reach the console, and none of them claims to be working', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium', 'one engine: recovery codes are a bounded fixture, see the file comment');

  await page.goto('/login');
  await signIn(page, spareCode());

  // An environment and a zone wide enough for the four addresses below.
  const environmentName = `Decoy lab ${Date.now()}`;
  await page.getByLabel('Display name').fill(environmentName);
  await page.getByRole('button', { name: 'Create environment' }).click();
  await page.getByRole('link', { name: environmentName }).click();
  await page.getByLabel('Zone name').fill('Decoy zone');
  await page.getByLabel('Private CIDR').fill('10.60.0.0/24');
  await page.getByRole('button', { name: 'Add zone' }).click();
  await expect(page.getByText('Private network zone added.')).toBeVisible();

  const environmentUrl = new URL(page.url());
  await page.goto(`${environmentUrl.pathname}/decoys`);
  await expect(page.getByRole('heading', { name: 'Decoys', exact: true })).toBeVisible();

  // ── 10.2.1: the Phase 2 exit gate ────────────────────────────────────────
  for (const entry of FAMILIES) {
    await page.getByRole('button', { name: 'Deploy decoy' }).first().click();
    await fillDecoy(page, entry, `${entry.family} decoy`);
    await page.getByRole('button', { name: 'Deploy decoy' }).last().click();
    await expect(page.getByText('Decoy saved. The Edge has not confirmed it yet.')).toBeVisible();
  }

  const table = page.getByRole('table');
  for (const entry of FAMILIES) {
    await expect(table.getByRole('row', { name: new RegExp(`${entry.family} decoy`) })).toBeVisible();
  }

  /*
   * 10.2.4, in the direction this harness can prove. No Edge has reported, so
   * every decoy is `Unknown` and the configuration column says Guardian is
   * still waiting. What must never appear is a fabricated failure or a claim of
   * deployment: section 9.5 forbids the console inventing a timeout, and
   * section 8.7 forbids presenting a request as an observation.
   */
  for (const entry of FAMILIES) {
    const row = table.getByRole('row', { name: new RegExp(`${entry.family} decoy`) });
    await expect(row.getByText('Unknown').first()).toBeVisible();
    await expect(row.getByText('Deployed')).toHaveCount(0);
  }
  await expect(page.getByText(/Nothing has reported on this decoy yet/).first()).toBeVisible();
  // Every persona is labelled as an emulation (AC-SMB-002).
  await expect(page.getByText('Emulated').first()).toBeVisible();

  await expectNoSeriousAxeViolations(page, 'the decoy list');

  // ── 10.2.5: a rejection the client cannot predict, rendered truthfully ────
  //
  // The address is a well-formed private IPv4 host address, so the console's
  // own validator passes it. It is outside the zone, which only the Control
  // Plane knows, and change proposal 0004 carries that back as a field error on
  // `address`. This is the whole reason that proposal exists.
  await page.getByRole('button', { name: 'Deploy decoy' }).first().click();
  await fillDecoy(page, { ...FAMILIES[0], address: '10.99.0.40' }, 'Outside the zone');
  await page.getByRole('button', { name: 'Deploy decoy' }).last().click();
  const address = page.getByLabel('Address', { exact: true });
  await expect(address).toHaveAttribute('aria-invalid', 'true');
  await expect(page.getByText('This address is not inside the selected zone.')).toBeVisible();
  await page.getByRole('button', { name: 'Cancel' }).click();

  // ── 10.2.2: disable and re-enable, reported rather than assumed ──────────
  const sshRow = table.getByRole('row', { name: /ssh decoy/ });
  await sshRow.getByRole('button', { name: /Disable ssh decoy/ }).click();
  await expect(page.getByText('Disable requested.', { exact: false })).toBeVisible();
  await expect(sshRow.getByRole('button', { name: /Enable ssh decoy/ })).toBeVisible();
  // Disabling changed what was asked for and nothing about what is observed.
  await expect(sshRow.getByText('Unknown').first()).toBeVisible();
  await sshRow.getByRole('button', { name: /Enable ssh decoy/ }).click();
  await expect(page.getByText('Enable requested.', { exact: false })).toBeVisible();

  // ── 10.2.6: hostile decoy content renders inert ─────────────────────────
  const hostile = '<img src=x onerror="alert(1)">';
  await page.getByRole('button', { name: 'Deploy decoy' }).first().click();
  await fillDecoy(page, { ...FAMILIES[3], address: '10.60.0.20' }, hostile);
  await page.getByRole('button', { name: 'Deploy decoy' }).last().click();
  await expect(page.getByText('Decoy saved. The Edge has not confirmed it yet.')).toBeVisible();
  // The name is present as text and created no element.
  await expect(page.getByText('onerror').first()).toBeVisible();
  expect(await page.locator('table img').count()).toBe(0);

  // ── The detail screen ───────────────────────────────────────────────────
  await table.getByRole('row', { name: /http decoy/ }).getByRole('link').click();
  await expect(page.getByRole('heading', { name: 'http decoy', level: 1 })).toBeVisible();
  await expect(page.getByText(/No digest has been recorded for this pack yet/)).toBeVisible();
  await expect(page.getByText(/Observed: Unknown/)).toBeVisible();
  // The attacker-visibility warning is on the field, not merely on the page.
  await page.getByRole('button', { name: 'Edit configuration' }).click();
  const nameField = page.getByLabel('Display name');
  const describedBy = await nameField.getAttribute('aria-describedby');
  expect(describedBy).toBeTruthy();
  await expect(page.locator(`#${describedBy!.split(' ')[0]}`)).toContainText('Attackers can see this value');
  await expectNoSeriousAxeViolations(page, 'the decoy detail screen');

  // ── 10.2.7: narrow viewport ─────────────────────────────────────────────
  await page.setViewportSize({ width: 375, height: 800 });
  await expect(page.getByRole('heading', { name: 'http decoy', level: 1 })).toBeVisible();
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow, 'the page must not scroll horizontally at 375px').toBeLessThanOrEqual(1);
  await page.setViewportSize({ width: 1280, height: 800 });

  // ── 10.2.3: removal keeps what was recorded ─────────────────────────────
  await page.goBack();
  await page.getByRole('link', { name: 'All decoys' }).click().catch(() => undefined);
  await expect(page.getByRole('heading', { name: 'Decoys', exact: true })).toBeVisible();
  const smbRow = table.getByRole('row', { name: /smb decoy/ });
  await smbRow.getByRole('button', { name: /Remove smb decoy/ }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Anything Guardian already recorded about it is kept');
  await expectNoSeriousAxeViolations(page, 'the removal confirmation');
  await dialog.getByRole('button', { name: 'Remove decoy' }).click();
  await expect(page.getByText('Decoy removed from the active list.')).toBeVisible();
  await expect(table.getByRole('row', { name: /smb decoy/ })).toHaveCount(0);

  // The three that remain are still there, and still honest about themselves.
  for (const entry of FAMILIES.slice(0, 3)) {
    await expect(table.getByRole('row', { name: new RegExp(`${entry.family} decoy`) })).toBeVisible();
  }
});
