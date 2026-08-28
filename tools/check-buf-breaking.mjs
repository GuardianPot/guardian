import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const bufFileAnnotationExitCode = 100;

export function selectBaselineRef(environment = process.env) {
  const override = environment.GUARDIAN_BUF_BASE_REF?.trim();
  if (override) return override;

  const pullRequestBase = environment.GITHUB_BASE_REF?.trim();
  if (pullRequestBase) return `origin/${pullRequestBase}`;

  return 'main';
}

export function checkBufBreaking({
  environment = process.env,
  execute = execFileSync,
  log = console.log,
  reportError = console.error,
} = {}) {
  const baselineRef = selectBaselineRef(environment);
  let baselineCommit;

  try {
    baselineCommit = execute('git', ['rev-parse', '--verify', `${baselineRef}^{commit}`], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim();
  } catch {
    reportError(`Unable to resolve Buf breaking baseline "${baselineRef}" to a commit.`);
    return 1;
  }

  if (!/^[0-9a-f]{40}([0-9a-f]{24})?$/i.test(baselineCommit)) {
    reportError(`Git returned an invalid commit ID for Buf breaking baseline "${baselineRef}".`);
    return 1;
  }

  let baseline;
  try {
    baseline = execute('git', ['ls-tree', '-r', '--name-only', baselineCommit, 'proto'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  } catch {
    reportError(`Unable to inspect Buf breaking baseline commit ${baselineCommit}.`);
    return 1;
  }

  if (!baseline.split(/\r?\n/).some((path) => path.endsWith('.proto'))) {
    log(
      `No Protobuf baseline exists at ${baselineCommit}; ` +
        'breaking signal is deferred until the first contract merge.',
    );
    return 0;
  }

  try {
    execute('buf', ['breaking', '--against', `.git#ref=${baselineCommit},subdir=proto`], {
      stdio: 'inherit',
    });
    log(`Buf breaking signal passed against ${baselineCommit}.`);
    return 0;
  } catch (error) {
    if (error?.status === bufFileAnnotationExitCode) {
      reportError(
        'Buf reported a compatibility finding. Development policy permits owner-reviewed ' +
          'breaking changes; all consumers must be updated together.',
      );
      return 0;
    }

    const status = Number.isInteger(error?.status) && error.status !== 0 ? error.status : 1;
    reportError(`Buf breaking execution failed with status ${status}.`);
    return status;
  }
}

const invokedPath = process.argv[1] ? resolve(process.argv[1]) : '';
if (invokedPath === fileURLToPath(import.meta.url)) {
  process.exitCode = checkBufBreaking();
}
