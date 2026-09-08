import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toConsoleError } from '@shared/api/error';
import { reveal } from '@shared/api/untrusted';
import { FormMessage, maxLengthOf, schemaFor, useConsoleForm } from '@shared/forms';
import type { Zone } from '@shared/api/types';
import { useAuth, useCapability } from '@features/auth';
import type { StepUp } from '@features/auth';
import {
  Button,
  ConfirmationDialog,
  InlineMessage,
  TextField,
  Timestamp,
  UntrustedText,
  confirmationFor,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, tx } from '@shared/text';
import { deleteZone, environmentInvalidation, updateZone } from './api';

/**
 * One zone, with its edit and delete controls (WCX-09 sections 9.4 and 9.8.1).
 *
 * The interesting part is the conflict. Both mutations send `If-Match` with
 * the revision the operator was looking at, and a `412` means someone else
 * changed the zone first. Section 9.8.1 forbids resolving that silently: a
 * console that retried without the header would overwrite a change it never
 * showed anyone. So the conflict is rendered as a conflict, distinct from a
 * validation failure, and the only offer is to reload the current value.
 *
 * Rename is level 1 and edits inline. Delete is level 2 and goes through the
 * confirmation, which names the zone.
 */
const zoneSchema = schemaFor<'ZoneWriteRequest', { display_name: string; cidr: string }>(
  'ZoneWriteRequest',
  ['display_name', 'cidr'],
);
const NAME_LIMIT = maxLengthOf('ZoneWriteRequest', 'display_name') ?? 512;
const CIDR_LIMIT = maxLengthOf('ZoneWriteRequest', 'cidr') ?? 18;

export function ZoneRow({ zone, stepUp }: { zone: Zone; stepUp: StepUp }) {
  const auth = useAuth();
  const client = useQueryClient();
  const update = useCapability(confirmationFor('zone.rename').capability);
  const remove = useCapability(confirmationFor('zone.delete').capability);
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [failed, setFailed] = useState('');

  const name = reveal(zone.display_name);
  const environmentID = zone.environment_id;

  const form = useConsoleForm<{ display_name: string; cidr: string }>({
    schema: zoneSchema,
    defaultValues: { display_name: name, cidr: zone.cidr },
    onSubmit: async (values) => {
      setFailed('');
      setConflict(false);
      if (!auth.csrf) {
        setFailed(t('environment.zones.reauthenticate'));
        return;
      }
      setBusy(true);
      try {
        await updateZone(environmentID, zone, values, auth.csrf);
        await environmentInvalidation.afterZoneWrite(client, environmentID);
        setEditing(false);
      } catch (caught) {
        // `conflict` covers 409 and 412 in the WCX-02 taxonomy. Both mean the
        // stored value is not what this form was built from.
        if (toConsoleError(caught).kind === 'conflict') setConflict(true);
        else setFailed(t('environment.zones.saveFailed'));
      } finally {
        setBusy(false);
      }
    },
  });

  async function destroy() {
    setConfirming(false);
    setFailed('');
    setConflict(false);
    if (!auth.csrf) {
      setFailed(t('environment.zones.reauthenticate'));
      return;
    }
    setBusy(true);
    try {
      await deleteZone(environmentID, zone, auth.csrf);
      await environmentInvalidation.afterZoneWrite(client, environmentID);
    } catch (caught) {
      if (toConsoleError(caught).kind === 'conflict') setConflict(true);
      else setFailed(t('environment.zones.deleteFailed'));
    } finally {
      setBusy(false);
    }
  }

  /** Discards local edits and takes whatever the Control Plane now holds. */
  async function reload() {
    setConflict(false);
    setEditing(false);
    await environmentInvalidation.afterZoneWrite(client, environmentID);
  }

  return (
    <li>
      {editing ? (
        <form className={styles.zoneEditForm} onSubmit={(event) => { void form.submit(event); }}>
          <FormMessage error={form.formError} unattached={form.unattached} id={form.formErrorId} />
          <TextField
            name="display_name"
            label={t('environment.zoneName')}
            defaultValue={name}
            required
            maxLength={NAME_LIMIT}
            registration={form.form.register('display_name')}
            {...(form.form.formState.errors.display_name?.message === undefined
              ? {}
              : { error: String(form.form.formState.errors.display_name.message) })}
          />
          <TextField
            name="cidr"
            label={t('environment.zoneCidr')}
            defaultValue={zone.cidr}
            required
            maxLength={CIDR_LIMIT}
            registration={form.form.register('cidr')}
            {...(form.form.formState.errors.cidr?.message === undefined
              ? {}
              : { error: String(form.form.formState.errors.cidr.message) })}
          />
          <Button variant="primary" type="submit" pending={busy}>{t('environment.zones.save')}</Button>
          <Button variant="quiet" onClick={() => { setEditing(false); setFailed(''); setConflict(false); }}>
            {t('common.cancel')}
          </Button>
        </form>
      ) : (
        <>
          <span>
            <strong><UntrustedText value={zone.display_name} /></strong>
            <small>{tx('environment.zoneUpdated', { time: <Timestamp value={zone.updated_at} /> })}</small>
          </span>
          <code>{zone.cidr}</code>
          <span className={styles.zoneActions}>
            <Button
              variant="quiet"
              onClick={() => { setFailed(''); setConflict(false); setEditing(true); }}
              {...(update.allowed ? {} : { disabledReason: t('environment.zones.reauthenticate') })}
            >
              {/* Section 9.6.5: a row action names its row. */}
              {t('environment.zones.editNamed', { zone: name })}
            </Button>
            <Button
              variant="destructive"
              pending={busy}
              onClick={() => { setFailed(''); setConflict(false); setConfirming(true); }}
              {...(remove.allowed ? {} : { disabledReason: t('environment.zones.reauthenticate') })}
            >
              {t('environment.zones.deleteNamed', { zone: name })}
            </Button>
          </span>
        </>
      )}

      {/*
        Section 9.8.1 and 10.1.6: a revision conflict is its own state, not a
        validation failure, and it says explicitly that nothing was written.
      */}
      {conflict && (
        <div className={styles.zoneConflict} role="alert">
          <p>{t('environment.zones.conflict')}</p>
          <Button variant="secondary" onClick={() => { void reload(); }}>
            {t('environment.zones.reload')}
          </Button>
        </div>
      )}
      {failed !== '' && <InlineMessage tone="error">{failed}</InlineMessage>}

      {confirming && (
        <ConfirmationDialog
          action="zone.delete"
          objectName={name}
          open
          stepUp={stepUp}
          onCancel={() => { setConfirming(false); }}
          onConfirm={() => { void destroy(); }}
        />
      )}
    </li>
  );
}
