import { useId } from 'react';
import { Button, TextField } from '@shared/ui';
import { FormMessage, UnsavedChangesGuard, choicesOf, maxLengthOf, schemaFor, useConsoleForm } from '@shared/forms';
import { t } from '@shared/text';
import styles from '@shared/styles/app.module.css';
import type { DecoyWriteRequest, Zone } from '@shared/api/types';

const DECOY_FIELDS = [
  'zone_id',
  'display_name',
  'family',
  'persona',
  'address',
  'pack',
  'pack_version',
] as const;

const schema = schemaFor<'DecoyWriteRequest', DecoyWriteRequest>('DecoyWriteRequest', DECOY_FIELDS);
const NAME_LIMIT = maxLengthOf('DecoyWriteRequest', 'display_name') ?? 512;
const ADDRESS_LIMIT = maxLengthOf('DecoyWriteRequest', 'address') ?? 15;

/**
 * The decoy configuration form (WCX-11 sections 9.4, 8.3 and 8.9).
 *
 * This is the form change proposal 0004 exists for: seven interdependent
 * fields, where a single form-level "something is wrong" would leave a
 * generalist operator without a SOC guessing which one.
 *
 * Two things here are security wording rather than UX wording.
 *
 * `ON-05` lets an operator override the address and the persona, and both are
 * shown to whoever probes the decoy. So the attacker-visibility warning sits on
 * the field, associated through `description` so a screen reader reaches it
 * with the control, rather than in a page-level note nobody reads twice.
 *
 * `AC-SMB-002` requires an emulated persona to be labelled as emulated. The
 * console never says a Windows, database, or application host exists, because
 * an operator who believes one does will reason about a machine that is not
 * there.
 *
 * Every constraint comes from `schemaFor`, which reads the generated contract.
 * Nothing here writes a length or a pattern.
 */
export function DecoyForm({
  zones,
  defaults,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  zones: readonly Zone[];
  defaults: DecoyWriteRequest;
  submitLabel: string;
  onSubmit: (values: DecoyWriteRequest) => Promise<void>;
  onCancel?: () => void;
}) {
  const formId = useId();
  const { form, submit, submitting, formError, unattached, formErrorId, isDirty, clearFormError } =
    useConsoleForm<DecoyWriteRequest>({
      schema,
      defaultValues: defaults,
      onSubmit,
    });
  const { errors } = form.formState;

  const field = (name: keyof DecoyWriteRequest) => ({
    name,
    registration: form.register(name),
    ...(errors[name]?.message === undefined ? {} : { error: String(errors[name].message) }),
  });

  return (
    <>
      <UnsavedChangesGuard when={isDirty && !submitting} />
      <form id={formId} className={styles.stackedForm} onSubmit={(event) => { void submit(event); }} onChange={clearFormError}>
        <FormMessage error={formError} unattached={unattached} id={formErrorId} />

        {/* Attacker-visible. The warning is associated with the control, not
            adjacent to it (section 9.7.3). */}
        <TextField
          {...field('display_name')}
          label={t('decoys.field.displayName')}
          description={t('decoys.attackerVisible.warning')}
          defaultValue={defaults.display_name}
          required
          maxLength={NAME_LIMIT}
        />

        <Choice
          name="family"
          label={t('decoys.field.family')}
          options={choicesOf('DecoyWriteRequest', 'family')}
          defaultValue={defaults.family}
          registration={form.register('family')}
          {...(errors.family?.message === undefined ? {} : { error: errors.family.message })}
        />

        <Choice
          name="persona"
          label={t('decoys.field.persona')}
          description={t('decoys.persona.emulatedNote')}
          options={choicesOf('DecoyWriteRequest', 'persona')}
          defaultValue={defaults.persona}
          registration={form.register('persona')}
          {...(errors.persona?.message === undefined ? {} : { error: errors.persona.message })}
        />

        <Choice
          name="zone_id"
          label={t('decoys.field.zone')}
          options={zones.map((zone) => zone.zone_id)}
          optionLabels={Object.fromEntries(zones.map((zone) => [zone.zone_id, zone.cidr]))}
          defaultValue={defaults.zone_id}
          registration={form.register('zone_id')}
          {...(errors.zone_id?.message === undefined ? {} : { error: errors.zone_id.message })}
        />

        <TextField
          {...field('address')}
          label={t('decoys.field.address')}
          description={t('decoys.attackerVisible.warning')}
          defaultValue={defaults.address}
          required
          maxLength={ADDRESS_LIMIT}
        />

        <TextField
          {...field('pack')}
          label={t('decoys.field.pack')}
          defaultValue={defaults.pack}
          required
        />
        <TextField
          {...field('pack_version')}
          label={t('decoys.field.packVersion')}
          defaultValue={defaults.pack_version}
          required
        />

        <p className={styles.rowNote}>{t('decoys.convergence.takesEffect')}</p>

        <div className={styles.formActions}>
          <Button variant="primary" type="submit" form={formId} pending={submitting}>
            {submitLabel}
          </Button>
          {onCancel !== undefined && (
            <Button variant="quiet" onClick={onCancel}>{t('common.cancel')}</Button>
          )}
        </div>
      </form>
    </>
  );
}

/**
 * A closed-vocabulary control.
 *
 * A `<select>` rather than free text because every one of these is a closed
 * token in the contract. The option list comes from the generated constraints,
 * so a family or persona added to the contract appears here without anyone
 * editing this file, and one removed disappears.
 */
function Choice({
  name,
  label,
  description,
  options,
  optionLabels,
  defaultValue,
  registration,
  error,
}: {
  name: string;
  label: string;
  description?: string;
  options: readonly string[];
  optionLabels?: Record<string, string>;
  defaultValue: string;
  registration: ReturnType<ReturnType<typeof useConsoleForm>['form']['register']>;
  error?: string;
}) {
  const fieldId = useId();
  const describedBy = [
    description === undefined ? '' : `${fieldId}-description`,
    error === undefined ? '' : `${fieldId}-error`,
  ].filter(Boolean).join(' ');
  return (
    <div className={styles.field}>
      <label htmlFor={fieldId}>{label}</label>
      <select
        id={fieldId}
        defaultValue={defaultValue}
        aria-invalid={error === undefined ? undefined : true}
        aria-describedby={describedBy === '' ? undefined : describedBy}
        {...registration}
        name={name}
      >
        {options.map((option) => (
          <option key={option} value={option}>{optionLabels?.[option] ?? option}</option>
        ))}
      </select>
      {description !== undefined && (
        <span className={styles.fieldDescription} id={`${fieldId}-description`}>{description}</span>
      )}
      {error !== undefined && (
        <span className={styles.formError} id={`${fieldId}-error`} role="alert">{error}</span>
      )}
    </div>
  );
}
