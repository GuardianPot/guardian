import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toConsoleError } from '@shared/api/error';
import { reveal } from '@shared/api/untrusted';
import type { Device } from '@shared/api/types';
import { useAuth, useCapability, useStepUp } from '@features/auth';
import {
  Banner,
  Button,
  ConfirmationDialog,
  InlineMessage,
  OneTimeSecretDialog,
  Panel,
  Timestamp,
  confirmationFor,
  type ConfirmableAction,
  type OneTimeSecret,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, tx } from '@shared/text';
import {
  createReenrollmentToken,
  deviceInvalidation,
  disableDevice,
  revokeDevice,
  DEVICE_TRANSITIONS,
  type DeviceTransition,
} from './api';

/**
 * The device lifecycle section (WCX-09 sections 9.4 and 9.5).
 *
 * Three rules the arrangement exists to hold:
 *
 * 1. **Every transition is visible in every state**, available or not, with
 *    the reason it is unavailable (`WC-D07`). A control that disappears tells
 *    an operator nothing; a control that says "the contract offers no path
 *    from revoked back to active" tells them where they stand.
 * 2. **The effect is stated before the control is used**, not only inside the
 *    confirmation. By the time a modal is open the operator has already
 *    decided.
 * 3. **Nothing here is optimistic** (section 9.8.4). A transition succeeds and
 *    the queries are invalidated; the state shown is whatever the Control
 *    Plane says on the next read. On failure the displayed state is untouched
 *    and the message says so, because an operator left unsure whether a revoke
 *    landed will either repeat it or trust a device they meant to remove.
 */
const ACTION_FOR: Readonly<Record<DeviceTransition, ConfirmableAction>> = {
  disable: 'device.disable',
  revoke: 'device.revoke',
  reenroll: 'device.reenroll',
};

const EFFECT_DETAIL = {
  disable: 'devices.lifecycle.disableEffect',
  revoke: 'devices.lifecycle.revokeEffect',
  reenroll: 'devices.lifecycle.reenrollEffect',
} as const;

const UNAVAILABLE_IN_STATE = {
  pending: 'devices.lifecycle.unavailablePending',
  active: 'devices.lifecycle.unavailableActive',
  disabled: 'devices.lifecycle.unavailableDisabled',
  revoked: 'devices.lifecycle.unavailableRevoked',
} as const;

export function DeviceLifecycle({ device }: { device: Device }) {
  const auth = useAuth();
  const client = useQueryClient();
  const stepUp = useStepUp();
  const [pending, setPending] = useState<DeviceTransition | null>(null);
  const [confirming, setConfirming] = useState<DeviceTransition | null>(null);
  const [failed, setFailed] = useState<string>('');
  // Route-local, never a query: this is one-time material (section 8.11).
  const [secret, setSecret] = useState<OneTimeSecret | null>(null);

  const name = reveal(device.display_name);
  const available = DEVICE_TRANSITIONS[device.state];

  async function run(transition: DeviceTransition) {
    setConfirming(null);
    setFailed('');
    if (!auth.csrf) {
      setFailed(t('devices.lifecycle.reauthenticate'));
      return;
    }
    setPending(transition);
    try {
      if (transition === 'reenroll') {
        const issued = await createReenrollmentToken(device.environment_id, device.device_id, auth.csrf);
        setSecret({ token: issued.token, expires_at: issued.expires_at });
      } else if (transition === 'disable') {
        await disableDevice(device.environment_id, device.device_id, auth.csrf);
      } else {
        await revokeDevice(device.environment_id, device.device_id, auth.csrf);
      }
      // The refetch is the only thing that changes what is displayed.
      await deviceInvalidation.afterLifecycle(client, device.environment_id, device.device_id);
    } catch (caught) {
      const error = toConsoleError(caught);
      setFailed(
        error.kind === 'conflict'
          ? t('devices.lifecycle.conflict')
          : t('devices.lifecycle.failed', { effect: t(confirmationFor(ACTION_FOR[transition]).effect) }),
      );
    } finally {
      setPending(null);
    }
  }

  return (
    <Panel heading={t('devices.lifecycle.heading')} headingLevel={2} eyebrow={t('devices.lifecycle.eyebrow')}>
      {/*
        Section 9.5.2. A revoked device stays in inventory with its state and
        the time of the change. Removing it would hide the history an operator
        needs in order to reconstruct what happened.
      */}
      {device.state === 'revoked' && (
        <Banner tone="restricted">
          {tx('devices.lifecycle.revokedNotice', { time: <Timestamp value={device.updated_at} precision="second" /> })}
        </Banner>
      )}
      <ul className={styles.lifecycleList}>
        {(['disable', 'revoke', 'reenroll'] as const).map((transition) => {
          const action = ACTION_FOR[transition];
          const confirmation = confirmationFor(action);
          const permitted = available.includes(transition);
          return (
            <li key={transition} className={styles.lifecycleRow}>
              <div>
                <strong>{t(confirmation.effect)}</strong>
                {/* Rule 1: the effect, in the operator's terms, before use. */}
                <p>{t(EFFECT_DETAIL[transition])}</p>
                {/*
                  Section 9.4 and `SEC-06`: re-enrollment leaves the Edge's
                  decoys unmanaged until it completes. `WCX-11` renders the
                  same sentence on the decoy side, so an operator meets one
                  fact rather than two descriptions of it.
                */}
                {transition === 'reenroll' && <p>{t('devices.lifecycle.unmanagedDecoys')}</p>}
              </div>
              <LifecycleControl
                action={action}
                permitted={permitted}
                unavailableReason={t(UNAVAILABLE_IN_STATE[device.state])}
                pending={pending === transition}
                onStart={() => {
                  setFailed('');
                  setConfirming(transition);
                }}
              />
            </li>
          );
        })}
      </ul>
      {failed !== '' && <InlineMessage tone="error">{failed}</InlineMessage>}

      {confirming !== null && (
        <ConfirmationDialog
          action={ACTION_FOR[confirming]}
          objectName={name}
          open
          stepUp={stepUp}
          onCancel={() => { setConfirming(null); }}
          onConfirm={() => { void run(confirming); }}
        />
      )}
      {stepUp.element}

      <OneTimeSecretDialog
        secret={secret}
        onDismiss={() => { setSecret(null); }}
        titleKey="devices.reenrollSecret.title"
        descriptionKey="devices.reenrollSecret.description"
        labelKey="devices.reenrollSecret.label"
      />
    </Panel>
  );
}

/**
 * One transition control.
 *
 * The capability check and the state check are separate and both are reported,
 * because they are different facts: a read-only session is a thing the
 * operator can fix, and a transition the contract does not offer from this
 * state is not.
 */
function LifecycleControl({
  action,
  permitted,
  unavailableReason,
  pending,
  onStart,
}: {
  action: ConfirmableAction;
  permitted: boolean;
  unavailableReason: string;
  pending: boolean;
  onStart: () => void;
}) {
  const capability = useCapability(confirmationFor(action).capability);
  const blocked = !permitted
    ? unavailableReason
    : capability.allowed
      ? undefined
      : t('devices.lifecycle.reauthenticate');
  return (
    <Button
      variant={action === 'device.reenroll' ? 'secondary' : 'destructive'}
      pending={pending}
      onClick={onStart}
      {...(blocked === undefined ? {} : { disabledReason: blocked })}
    >
      {t(confirmationFor(action).effect)}
    </Button>
  );
}
