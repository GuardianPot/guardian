import assert from 'node:assert/strict';
import test from 'node:test';

import { checkBufBreaking, selectBaselineRef } from './check-buf-breaking.mjs';

const commit = '0123456789abcdef0123456789abcdef01234567';

test('explicit baseline override takes precedence', () => {
  assert.equal(
    selectBaselineRef({
      GUARDIAN_BUF_BASE_REF: `  ${commit}  `,
      GITHUB_BASE_REF: 'main',
    }),
    commit,
  );
});

test('pull-request base selects the remote-tracking ref', () => {
  assert.equal(selectBaselineRef({ GITHUB_BASE_REF: 'release/candidate' }), 'origin/release/candidate');
});

test('local and push execution fall back to main', () => {
  assert.equal(selectBaselineRef({}), 'main');
});

test('resolved commit is used for baseline inspection and Buf comparison', () => {
  const calls = [];
  const logs = [];
  const execute = (command, args, options) => {
    calls.push({ command, args, options });
    if (command === 'git' && args[0] === 'rev-parse') return `${commit}\n`;
    if (command === 'git' && args[0] === 'ls-tree') return 'proto/guardian/v1/example.proto\n';
    if (command === 'buf') return '';
    throw new Error(`Unexpected command: ${command}`);
  };

  assert.equal(
    checkBufBreaking({
      environment: { GUARDIAN_BUF_BASE_REF: 'origin/main' },
      execute,
      log: (message) => logs.push(message),
    }),
    0,
  );

  assert.deepEqual(
    calls.map(({ command, args }) => [command, args]),
    [
      ['git', ['rev-parse', '--verify', 'origin/main^{commit}']],
      ['git', ['ls-tree', '-r', '--name-only', commit, 'proto']],
      ['buf', ['breaking', '--against', `.git#ref=${commit},subdir=proto`]],
    ],
  );
  assert.ok(calls.every(({ options }) => !Object.hasOwn(options, 'shell')));
  assert.deepEqual(logs, [`Buf breaking signal passed against ${commit}.`]);
});

test('missing baseline ref fails closed with a concise error', () => {
  const errors = [];
  const execute = () => {
    throw Object.assign(new Error('missing ref'), { status: 128 });
  };

  assert.equal(
    checkBufBreaking({
      environment: { GUARDIAN_BUF_BASE_REF: 'origin/missing' },
      execute,
      reportError: (message) => errors.push(message),
    }),
    1,
  );
  assert.deepEqual(errors, ['Unable to resolve Buf breaking baseline "origin/missing" to a commit.']);
});

test('invalid resolved commit is rejected before baseline inspection', () => {
  let calls = 0;
  const errors = [];
  const execute = () => {
    calls += 1;
    return 'not-a-commit\n';
  };

  assert.equal(
    checkBufBreaking({
      execute,
      reportError: (message) => errors.push(message),
    }),
    1,
  );
  assert.equal(calls, 1);
  assert.deepEqual(errors, ['Git returned an invalid commit ID for Buf breaking baseline "main".']);
});

test('missing Protobuf baseline defers without invoking Buf', () => {
  const calls = [];
  const logs = [];
  const execute = (command, args) => {
    calls.push([command, args]);
    if (args[0] === 'rev-parse') return `${commit}\n`;
    if (args[0] === 'ls-tree') return 'proto/.gitkeep\n';
    throw new Error('Buf must not run without a Protobuf baseline.');
  };

  assert.equal(checkBufBreaking({ execute, log: (message) => logs.push(message) }), 0);
  assert.equal(calls.length, 2);
  assert.match(logs[0], /breaking signal is deferred/);
});

test('Buf compatibility finding remains informational', () => {
  const errors = [];
  const execute = (command, args) => {
    if (command === 'git' && args[0] === 'rev-parse') return `${commit}\n`;
    if (command === 'git' && args[0] === 'ls-tree') return 'proto/example.proto\n';
    throw Object.assign(new Error('breaking change'), { status: 100 });
  };

  assert.equal(checkBufBreaking({ execute, reportError: (message) => errors.push(message) }), 0);
  assert.match(errors[0], /Buf reported a compatibility finding/);
});

test('Buf operational failure remains blocking', () => {
  const errors = [];
  const execute = (command, args) => {
    if (command === 'git' && args[0] === 'rev-parse') return `${commit}\n`;
    if (command === 'git' && args[0] === 'ls-tree') return 'proto/example.proto\n';
    throw Object.assign(new Error('operational failure'), { status: 1 });
  };

  assert.equal(checkBufBreaking({ execute, reportError: (message) => errors.push(message) }), 1);
  assert.deepEqual(errors, ['Buf breaking execution failed with status 1.']);
});
