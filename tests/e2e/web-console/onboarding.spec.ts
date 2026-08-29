import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import process from 'node:process';
import { spawn } from 'node:child_process';
import { publishHealthy } from './device-health';

test('real owner onboarding, Edge enrollment, health degradation, and recovery', async ({ page, context }, testInfo) => {
  const projectIndex = ['chromium', 'firefox', 'webkit'].indexOf(testInfo.project.name);
  expect(projectIndex).toBeGreaterThanOrEqual(0);
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
  await signIn(page, recoveryCodes[projectIndex * 2]);
  const environmentName = `Browser ${testInfo.project.name}`;
  await page.getByLabel('Display name').fill(environmentName);
  await page.getByRole('button', { name: 'Create environment' }).click();
  await page.getByRole('link', { name: new RegExp(environmentName) }).click();

  await page.reload();
  await expect(page.getByText('Read-only session restored.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Create one-time secret' })).toBeDisabled();
  await page.getByRole('link', { name: 'Re-authenticate' }).first().click();
  await signIn(page, recoveryCodes[projectIndex * 2 + 1]);
  await page.getByRole('link', { name: new RegExp(environmentName) }).click();

  const csrfFailure = await page.evaluate(async () => (await fetch('/v1/environments', {
    method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'invalid' }, body: '{"display_name":"denied"}',
  })).status);
  expect(csrfFailure).toBe(401);

  await page.getByLabel('Zone name').fill(`Zone ${testInfo.project.name}`);
  await page.getByLabel('Private CIDR').fill(`10.${20 + projectIndex}.0.0/24`);
  await page.getByRole('button', { name: 'Add zone' }).click();
  await expect(page.getByText('Private network zone added.')).toBeVisible();

  expect(await runEdgeEnrollment(edgeConfig, runtimeConfig, Buffer.from('invalid\n'))).not.toBe(0);
  await page.getByLabel('Device name').fill(`edge-${testInfo.project.name}`);
  await page.getByRole('button', { name: 'Create one-time secret' }).click();
  const secretLocator = page.getByTestId('enrollment-secret').locator('code');
  const secretText = await secretLocator.textContent();
  expect(secretText).toMatch(/^[A-Za-z0-9_-]{43}$/);
  const enrollmentInput = Buffer.from(`${secretText}\n`);
  expect(await runEdgeEnrollment(edgeConfig, runtimeConfig, enrollmentInput)).toBe(0);
  enrollmentInput.fill(0);
  await page.getByRole('button', { name: 'I have stored it securely' }).click();
  await expect(page.getByTestId('enrollment-secret')).toHaveCount(0);
  await expect(page).not.toHaveURL(new RegExp(secretText!));

  let connection = await publishHealthy(identityDirectory, 1);
  const deviceLink = page.getByRole('link', { name: new RegExp(`edge-${testInfo.project.name}.*active`, 's') });
  await expect(deviceLink).toBeVisible();
  await deviceLink.click();
  await expect(page.getByRole('heading', { name: 'Eight-condition health' })).toBeVisible();
  await expect(page.getByText('Healthy', { exact: true }).first()).toBeVisible();
  await expect(page.getByRole('list', { name: 'Device health conditions' }).getByRole('listitem')).toHaveCount(8);

  connection.close();
  await expect(page.getByText('Action required', { exact: true }).first()).toBeVisible({ timeout: 20_000 });
  await new Promise((resolve) => setTimeout(resolve, 100));
  connection = await publishHealthy(identityDirectory, 2);
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
  connection.close();

  await context.clearCookies();
  await page.reload();
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
