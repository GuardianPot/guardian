import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { Link, useParams } from 'react-router';
import { configEncoding, deviceEncoding } from '@shared/theme/statusEncoding';
import { devicesQuery } from '@features/devices';
import {
  createEnrollmentSecret,
  createZone as createZoneRequest,
  environmentInvalidation,
  environmentKeys,
  environmentQuery,
  updateEnvironment as updateEnvironmentRequest,
  zonesQuery,
} from './api';
import type { EnrollmentSecret } from '@shared/api/types';
import { useAuth, useCapability, useStepUp } from '@features/auth';
import { FormMessage, maxLengthOf, schemaFor, useConsoleForm } from '@shared/forms';
import { environmentHealthQuery, HealthPanel } from '@features/health';
import { SecretDialog } from './SecretDialog';
import { EnrollmentTokenPanel } from './EnrollmentTokenPanel';
import { ZoneRow } from './ZoneRow';
import { Banner, Breadcrumbs, Button, DataBoundary, Panel, StatusBadge, TextField, UntrustedText } from '@shared/ui';
import { reveal } from '@shared/api/untrusted';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

type PageMessage = { text: string; tone: 'informational' | 'blocking' };

/*
 * Contract-derived validators (WCX-11 section 8.2).
 *
 * Every bound on this screen used to be a literal typed next to the control.
 * They are read from the generated contract now, so a length changed in
 * `openapi/guardian.yaml` cannot leave a form accepting what the Control Plane
 * refuses.
 */
const renameSchema = schemaFor<'EnvironmentWriteRequest', { display_name: string }>(
  'EnvironmentWriteRequest',
  ['display_name'],
);
const zoneSchema = schemaFor<'ZoneWriteRequest', { display_name: string; cidr: string }>(
  'ZoneWriteRequest',
  ['display_name', 'cidr'],
);
const NAME_LIMIT = maxLengthOf('EnvironmentWriteRequest', 'display_name') ?? 512;
const ZONE_NAME_LIMIT = maxLengthOf('ZoneWriteRequest', 'display_name') ?? 512;
const CIDR_LIMIT = maxLengthOf('ZoneWriteRequest', 'cidr') ?? 18;

export function EnvironmentPage() {
  const { environmentId = '' } = useParams();
  const auth = useAuth();
  // One step-up per screen, shared by every irreversible action on it: the
  // mark is per-action, so sharing the prompt cannot share an approval.
  const stepUp = useStepUp();
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<PageMessage | null>(null);
  const [secret, setSecret] = useState<EnrollmentSecret | null>(null);
  const secretRef = useRef<EnrollmentSecret | null>(null);
  const [creatingSecret, setCreatingSecret] = useState(false);
  const deviceNameField = useRef<HTMLInputElement>(null);
  const zoneNameField = useRef<HTMLInputElement>(null);
  const enrollDevice = useCapability('device.enroll');
  const defineZone = useCapability('zone.create');
  const updateEnvironment = useCapability('environment.update');
  const environment = useQuery(environmentQuery(environmentId));
  const zones = useQuery(zonesQuery(environmentId));
  const devices = useQuery(devicesQuery(environmentId));
  const health = useQuery(environmentHealthQuery(environmentId));

  useEffect(() => { secretRef.current = secret; }, [secret]);
  useEffect(() => {
    const clear = () => { secretRef.current = null; setSecret(null); };
    window.addEventListener('beforeunload', clear);
    return () => { window.removeEventListener('beforeunload', clear); secretRef.current = null; };
  }, []);

  const rename = useMutation({
    mutationFn: (displayName: string) => updateEnvironmentRequest(environment.data!, displayName, auth.csrf!),
    onSuccess: (updated) => {
      queryClient.setQueryData(environmentKeys.detail(environmentId), updated);
      void environmentInvalidation.afterEnvironmentWrite(queryClient);
    },
  });
  const createZone = useMutation({
    mutationFn: (input: { display_name: string; cidr: string }) => createZoneRequest(environmentId, input, auth.csrf!),
    onSuccess: () => environmentInvalidation.afterZoneWrite(queryClient, environmentId),
  });

  const renameForm = useConsoleForm<{ display_name: string }>({
    schema: renameSchema,
    defaultValues: { display_name: '' },
    onSubmit: async (values) => {
      setMessage(null);
      if (!auth.csrf || !environment.data) {
        setMessage({ text: t('environment.reauthenticateToRename'), tone: 'blocking' });
        return;
      }
      try {
        await rename.mutateAsync(values.display_name);
        setMessage({ text: t('environment.renamed'), tone: 'informational' });
      } catch {
        setMessage({ text: t('environment.renameFailed'), tone: 'blocking' });
      }
    },
  });

  /*
   * The rename field is seeded from the record, which arrives after the form is
   * created. Without this the form stack would hold the empty default and a
   * rename that changed nothing would submit an empty name — the uncontrolled
   * input showed the right value while the validator held the wrong one.
   */
  const currentName = environment.data === undefined ? undefined : reveal(environment.data.display_name);
  useEffect(() => {
    if (currentName !== undefined) renameForm.form.reset({ display_name: currentName });
    // The form object is stable for the life of the component; resetting on
    // every render would discard whatever the operator had typed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentName]);

  const zoneForm = useConsoleForm<{ display_name: string; cidr: string }>({
    schema: zoneSchema,
    defaultValues: { display_name: '', cidr: '' },
    onSubmit: async (values) => {
      setMessage(null);
      if (!auth.csrf) {
        setMessage({ text: t('environment.reauthenticateToAddZone'), tone: 'blocking' });
        return;
      }
      try {
        await createZone.mutateAsync(values);
        zoneForm.form.reset({ display_name: '', cidr: '' });
        setMessage({ text: t('environment.zoneAdded'), tone: 'informational' });
      } catch {
        setMessage({ text: t('environment.zoneFailed'), tone: 'blocking' });
      }
    },
  });

  /*
   * The enrollment device name has no contract schema of its own: the request
   * body is defined inline on the operation, not as a named component, so
   * there is nothing for `schemaFor` to read. Rather than hand-copy a bound —
   * which is what section 8.2 forbids — it reuses the environment name schema,
   * which is the same shape and the same limits, and a comment records why.
   */
  const enrollmentForm = useConsoleForm<{ device_name: string }>({
    schema: schemaFor<'EnvironmentWriteRequest', { device_name: string }>(
      'EnvironmentWriteRequest',
      [],
      { device_name: ['display_name'] },
    ),
    defaultValues: { device_name: '' },
    onSubmit: async (values) => {
      setMessage(null);
      if (!auth.csrf) {
        setMessage({ text: t('environment.reauthenticateToEnroll'), tone: 'blocking' });
        return;
      }
      setCreatingSecret(true);
      try {
        const created = await createEnrollmentSecret(environmentId, values.device_name, auth.csrf);
        enrollmentForm.form.reset({ device_name: '' });
        setSecret(created);
        await environmentInvalidation.afterEnrollmentSecret(queryClient, environmentId);
      } catch {
        setMessage({ text: t('environment.secretDenied'), tone: 'blocking' });
      } finally {
        setCreatingSecret(false);
      }
    },
  });

  const enrollReason = enrollDevice.allowed ? undefined : t('environment.reauthenticateToEnroll');
  const zoneReason = defineZone.allowed ? undefined : t('environment.reauthenticateToAddZone');
  const renameReason = updateEnvironment.allowed ? undefined : t('environment.reauthenticateToRename');

  return (
    <div>
      {/*
        Section 9.4.2. The trail names where the operator is, not only one
        step out of it. The environment's own name is the last entry and is
        untrusted, so it arrives as a node rather than as a string — and until
        the read lands it is the generic screen name rather than a blank,
        because an empty crumb reads as a missing record.
      */}
      <Breadcrumbs trail={[
        { label: t('environments.heading'), to: '/environments' },
        {
          label: environment.data
            ? <UntrustedText value={environment.data.display_name} />
            : t('environment.eyebrow'),
        },
      ]} />
      <DataBoundary
        query={environment}
        subject={{
          name: t('environment.loading'),
          dependency: t('common.controlPlane'),
          stillWorks: t('environment.stillWorks'),
          doesNotWork: t('environment.doesNotWork'),
          staleReason: t('environment.staleReason'),
        }}
        onRetry={() => { void environment.refetch(); }}
      >
        {(record) => (
          <>
            <header className={styles.pageHeader}>
              <div>
                <p className={styles.eyebrow}>{t('environment.eyebrow')}</p>
                <h1 tabIndex={-1}><UntrustedText value={record.display_name} /></h1>
                <p className={styles.mono}>{record.environment_id}</p>
              </div>
              <StatusBadge encoding={configEncoding(record.status)} />
            </header>
            {message && <Banner tone={message.tone}>{message.text}</Banner>}
            <div className={styles.twoColumn}>
              <Panel
                heading={t('environment.inventoryHeading')}
                headingLevel={2}
                eyebrow={t('environment.inventoryEyebrow')}
                aside={<span className={styles.panelCount}>{devices.data?.length ?? 0}</span>}
              >
                <DataBoundary
                  query={devices}
                  subject={{
                    name: t('environment.deviceCollection'),
                    dependency: t('common.controlPlane'),
                    stillWorks: t('environment.deviceStillWorks'),
                    doesNotWork: t('environment.deviceDoesNotWork'),
                    staleReason: t('environment.deviceStaleReason'),
                  }}
                  onRetry={() => { void devices.refetch(); }}
                  emptyAction={
                    enrollDevice.allowed
                      ? <Button variant="secondary" onClick={() => deviceNameField.current?.focus()}>{t('environment.enrollFirst')}</Button>
                      : undefined
                  }
                >
                  {(list) => (
                    <ul className={styles.deviceList} aria-label={t('environment.deviceCollection')}>
                      {list.map((device) => (
                        <li key={device.device_id}>
                          <Link to={`/environments/${environmentId}/devices/${device.device_id}`}>
                            <span><strong><UntrustedText value={device.display_name} /></strong><small>{device.device_id}</small></span>
                            <StatusBadge encoding={deviceEncoding(device.state)} />
                          </Link>
                        </li>
                      ))}
                    </ul>
                  )}
                </DataBoundary>
              </Panel>
              <Panel heading={t('environment.enrollHeading')} headingLevel={2} eyebrow={t('environment.enrollEyebrow')}>
                <p>{t('environment.enrollIntro')}</p>
                <form className={styles.form} onSubmit={(event) => { void enrollmentForm.submit(event); }}>
                  <FormMessage error={enrollmentForm.formError} unattached={enrollmentForm.unattached} id={enrollmentForm.formErrorId} />
                  <TextField
                    name="device_name"
                    label={t('environment.deviceName')}
                    required
                    maxLength={NAME_LIMIT}
                    inputRef={deviceNameField}
                    registration={enrollmentForm.form.register('device_name')}
                    {...(enrollmentForm.form.formState.errors.device_name?.message === undefined
                      ? {}
                      : { error: String(enrollmentForm.form.formState.errors.device_name.message) })}
                    {...(enrollReason === undefined ? {} : { disabledReason: enrollReason })}
                  />
                  <Button
                    variant="primary"
                    type="submit"
                    pending={creatingSecret}
                    {...(enrollReason === undefined ? {} : { disabledReason: enrollReason })}
                  >
                    {t('environment.createSecret')}
                  </Button>
                </form>
              </Panel>
            </div>
            {/*
              The handoffs the enrollment form above produces, with the
              window each one has left. Never a token value (section 8.9).
            */}
            <EnrollmentTokenPanel environmentID={environmentId} />
            {/*
              Health is a separate read with separate truth. Inventory presence
              above says nothing about it, and a missing projection renders as
              `unknown` — never as an empty or a healthy panel.
            */}
            <DataBoundary
              query={health}
              observationShaped
              // OPS-03: a health projection older than the freshness policy is
              // shown as stale even though the read succeeded. An old
              // observation is not the current state of a network.
              observedAt={health.data?.received_at ?? null}
              subject={{
                name: t('health.environmentSubject'),
                observationSource: t('health.observationSource'),
                dependency: t('health.dependency'),
                stillWorks: t('health.stillWorks'),
                doesNotWork: t('health.doesNotWork'),
                staleReason: t('health.staleReason'),
              }}
              onRetry={() => { void health.refetch(); }}
            >
              {(view) => <HealthPanel health={view} />}
            </DataBoundary>
            <div className={styles.twoColumn}>
              <Panel
                heading={t('environment.zonesHeading')}
                headingLevel={2}
                aside={<span className={styles.panelCount}>{zones.data?.length ?? 0}</span>}
              >
                <DataBoundary
                  query={zones}
                  subject={{
                    name: t('environment.zoneCollection'),
                    dependency: t('common.controlPlane'),
                    stillWorks: t('environment.zoneStillWorks'),
                    doesNotWork: t('environment.zoneDoesNotWork'),
                    staleReason: t('environment.zoneStaleReason'),
                  }}
                  onRetry={() => { void zones.refetch(); }}
                  emptyAction={
                    defineZone.allowed
                      ? <Button variant="secondary" onClick={() => zoneNameField.current?.focus()}>{t('environment.nameFirstZone')}</Button>
                      : undefined
                  }
                >
                  {(list) => (
                    <ul className={styles.zoneList}>
                      {list.map((zone) => (
                        <ZoneRow key={zone.zone_id} zone={zone} stepUp={stepUp} />
                      ))}
                    </ul>
                  )}
                </DataBoundary>
                <form className={styles.inlineForm} onSubmit={(event) => { void zoneForm.submit(event); }}>
                  <FormMessage error={zoneForm.formError} unattached={zoneForm.unattached} id={zoneForm.formErrorId} />
                  <TextField
                    name="display_name"
                    label={t('environment.zoneName')}
                    required
                    maxLength={ZONE_NAME_LIMIT}
                    inputRef={zoneNameField}
                    registration={zoneForm.form.register('display_name')}
                    {...(zoneForm.form.formState.errors.display_name?.message === undefined
                      ? {}
                      : { error: String(zoneForm.form.formState.errors.display_name.message) })}
                    {...(zoneReason === undefined ? {} : { disabledReason: zoneReason })}
                  />
                  <TextField
                    name="cidr"
                    label={t('environment.zoneCidr')}
                    placeholder="10.20.0.0/24"
                    required
                    maxLength={CIDR_LIMIT}
                    registration={zoneForm.form.register('cidr')}
                    {...(zoneForm.form.formState.errors.cidr?.message === undefined
                      ? {}
                      : { error: String(zoneForm.form.formState.errors.cidr.message) })}
                    {...(zoneReason === undefined ? {} : { disabledReason: zoneReason })}
                  />
                  <Button
                    variant="secondary"
                    type="submit"
                    pending={createZone.isPending}
                    {...(zoneReason === undefined ? {} : { disabledReason: zoneReason })}
                  >
                    {t('environment.addZone')}
                  </Button>
                </form>
              </Panel>
              <Panel heading={t('environment.settingsHeading')} headingLevel={2} eyebrow={t('environment.settingsEyebrow')}>
                <form className={styles.form} onSubmit={(event) => { void renameForm.submit(event); }}>
                  <FormMessage error={renameForm.formError} unattached={renameForm.unattached} id={renameForm.formErrorId} />
                  <TextField
                    name="display_name"
                    label={t('environment.displayNameLabel')}
                    /*
                      `reveal` is correct here and nowhere else on this screen:
                      the operator is editing the value, so the field needs the
                      original. React sets an input value as a property, never
                      as parsed markup, so nothing is interpreted on the way in.
                    */
                    defaultValue={reveal(record.display_name)}
                    required
                    maxLength={NAME_LIMIT}
                    registration={renameForm.form.register('display_name')}
                    {...(renameForm.form.formState.errors.display_name?.message === undefined
                      ? {}
                      : { error: String(renameForm.form.formState.errors.display_name.message) })}
                    {...(renameReason === undefined ? {} : { disabledReason: renameReason })}
                  />
                  <Button
                    variant="secondary"
                    type="submit"
                    pending={rename.isPending}
                    {...(renameReason === undefined ? {} : { disabledReason: renameReason })}
                  >
                    {t('environment.saveName')}
                  </Button>
                </form>
              </Panel>
            </div>
            {/*
              The handoffs the enrollment form above produces, with the
              window each one has left. Never a token value (section 8.9).
            */}
            <EnrollmentTokenPanel environmentID={environmentId} />
          </>
        )}
      </DataBoundary>
      <SecretDialog secret={secret} onDismiss={() => setSecret(null)} />
    </div>
  );
}
