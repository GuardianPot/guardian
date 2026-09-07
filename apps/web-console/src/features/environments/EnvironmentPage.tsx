import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
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
import { useAuth, useCapability } from '@features/auth';
import { textField } from '@shared/forms/textField';
import { environmentHealthQuery, HealthPanel, HEALTH_TEXT, formatTime } from '@features/health';
import { SecretDialog } from './SecretDialog';
import { Banner, Button, DataBoundary, Panel, StatusBadge, TextField, UntrustedText } from '@shared/ui';
import { reveal } from '@shared/api/untrusted';
import styles from '@shared/styles/app.module.css';
import { ENVIRONMENT_TEXT as TEXT } from './text';

type PageMessage = { text: string; tone: 'informational' | 'blocking' };

export function EnvironmentPage() {
  const { environmentId = '' } = useParams();
  const auth = useAuth();
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

  async function submitRename(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setMessage(null);
    if (!auth.csrf || !environment.data) {
      return setMessage({ text: TEXT.reauthenticateToRename, tone: 'blocking' });
    }
    const form = event.currentTarget;
    try {
      await rename.mutateAsync(textField(new FormData(form), 'display_name'));
      setMessage({ text: TEXT.renamed, tone: 'informational' });
    } catch {
      setMessage({ text: TEXT.renameFailed, tone: 'blocking' });
    }
  }

  async function submitZone(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setMessage(null);
    if (!auth.csrf) return setMessage({ text: TEXT.reauthenticateToAddZone, tone: 'blocking' });
    const form = event.currentTarget; const data = new FormData(form);
    try {
      await createZone.mutateAsync({ display_name: textField(data, 'display_name'), cidr: textField(data, 'cidr') });
      form.reset();
      setMessage({ text: TEXT.zoneAdded, tone: 'informational' });
    } catch {
      setMessage({ text: TEXT.zoneFailed, tone: 'blocking' });
    }
  }

  async function submitEnrollment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setMessage(null);
    if (!auth.csrf) return setMessage({ text: TEXT.reauthenticateToEnroll, tone: 'blocking' });
    const form = event.currentTarget;
    setCreatingSecret(true);
    try {
      const created = await createEnrollmentSecret(environmentId, textField(new FormData(form), 'device_name'), auth.csrf);
      form.reset(); setSecret(created);
      await environmentInvalidation.afterEnrollmentSecret(queryClient, environmentId);
    } catch {
      setMessage({ text: TEXT.secretDenied, tone: 'blocking' });
    } finally {
      setCreatingSecret(false);
    }
  }

  const enrollReason = enrollDevice.allowed ? undefined : TEXT.reauthenticateToEnroll;
  const zoneReason = defineZone.allowed ? undefined : TEXT.reauthenticateToAddZone;
  const renameReason = updateEnvironment.allowed ? undefined : TEXT.reauthenticateToRename;

  return (
    <div>
      <Link className={styles.backLink} to="/environments">{TEXT.back}</Link>
      <DataBoundary
        query={environment}
        subject={{
          name: TEXT.loading,
          dependency: TEXT.dependency,
          stillWorks: TEXT.stillWorks,
          doesNotWork: TEXT.doesNotWork,
          staleReason: TEXT.staleReason,
        }}
        onRetry={() => { void environment.refetch(); }}
      >
        {(record) => (
          <>
            <header className={styles.pageHeader}>
              <div>
                <p className={styles.eyebrow}>{TEXT.eyebrow}</p>
                <h1 tabIndex={-1}><UntrustedText value={record.display_name} /></h1>
                <p className={styles.mono}>{record.environment_id}</p>
              </div>
              <StatusBadge encoding={configEncoding(record.status)} />
            </header>
            {message && <Banner tone={message.tone}>{message.text}</Banner>}
            <div className={styles.twoColumn}>
              <Panel
                heading={TEXT.inventoryHeading}
                headingLevel={2}
                eyebrow={TEXT.inventoryEyebrow}
                aside={<span className={styles.panelCount}>{devices.data?.length ?? 0}</span>}
              >
                <DataBoundary
                  query={devices}
                  subject={{
                    name: TEXT.deviceCollection,
                    dependency: TEXT.deviceDependency,
                    stillWorks: TEXT.deviceStillWorks,
                    doesNotWork: TEXT.deviceDoesNotWork,
                    staleReason: TEXT.deviceStaleReason,
                  }}
                  onRetry={() => { void devices.refetch(); }}
                  emptyAction={
                    enrollDevice.allowed
                      ? <Button variant="secondary" onClick={() => deviceNameField.current?.focus()}>{TEXT.enrollFirst}</Button>
                      : undefined
                  }
                >
                  {(list) => (
                    <ul className={styles.deviceList}>
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
              <Panel heading={TEXT.enrollHeading} headingLevel={2} eyebrow={TEXT.enrollEyebrow}>
                <p>{TEXT.enrollIntro}</p>
                <form className={styles.form} onSubmit={(event) => { void submitEnrollment(event); }}>
                  <TextField
                    name="device_name"
                    label={TEXT.deviceName}
                    required
                    maxLength={128}
                    inputRef={deviceNameField}
                    {...(enrollReason === undefined ? {} : { disabledReason: enrollReason })}
                  />
                  <Button
                    variant="primary"
                    type="submit"
                    pending={creatingSecret}
                    {...(enrollReason === undefined ? {} : { disabledReason: enrollReason })}
                  >
                    {TEXT.createSecret}
                  </Button>
                </form>
              </Panel>
            </div>
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
                name: HEALTH_TEXT.environmentSubject,
                observationSource: HEALTH_TEXT.observationSource,
                dependency: HEALTH_TEXT.dependency,
                stillWorks: HEALTH_TEXT.stillWorks,
                doesNotWork: HEALTH_TEXT.doesNotWork,
                staleReason: HEALTH_TEXT.staleReason,
              }}
              onRetry={() => { void health.refetch(); }}
            >
              {(view) => <HealthPanel health={view} />}
            </DataBoundary>
            <div className={styles.twoColumn}>
              <Panel
                heading={TEXT.zonesHeading}
                headingLevel={2}
                aside={<span className={styles.panelCount}>{zones.data?.length ?? 0}</span>}
              >
                <DataBoundary
                  query={zones}
                  subject={{
                    name: TEXT.zoneCollection,
                    dependency: TEXT.zoneDependency,
                    stillWorks: TEXT.zoneStillWorks,
                    doesNotWork: TEXT.zoneDoesNotWork,
                    staleReason: TEXT.zoneStaleReason,
                  }}
                  onRetry={() => { void zones.refetch(); }}
                  emptyAction={
                    defineZone.allowed
                      ? <Button variant="secondary" onClick={() => zoneNameField.current?.focus()}>{TEXT.nameFirstZone}</Button>
                      : undefined
                  }
                >
                  {(list) => (
                    <ul className={styles.zoneList}>
                      {list.map((zone) => (
                        <li key={zone.zone_id}>
                          <span><strong><UntrustedText value={zone.display_name} /></strong><small>{TEXT.updated(formatTime(zone.updated_at))}</small></span>
                          <code>{zone.cidr}</code>
                        </li>
                      ))}
                    </ul>
                  )}
                </DataBoundary>
                <form className={styles.inlineForm} onSubmit={(event) => { void submitZone(event); }}>
                  <TextField
                    name="display_name"
                    label={TEXT.zoneName}
                    required
                    maxLength={128}
                    inputRef={zoneNameField}
                    {...(zoneReason === undefined ? {} : { disabledReason: zoneReason })}
                  />
                  <TextField
                    name="cidr"
                    label={TEXT.zoneCidr}
                    placeholder="10.20.0.0/24"
                    required
                    maxLength={18}
                    {...(zoneReason === undefined ? {} : { disabledReason: zoneReason })}
                  />
                  <Button
                    variant="secondary"
                    type="submit"
                    pending={createZone.isPending}
                    {...(zoneReason === undefined ? {} : { disabledReason: zoneReason })}
                  >
                    {TEXT.addZone}
                  </Button>
                </form>
              </Panel>
              <Panel heading={TEXT.settingsHeading} headingLevel={2} eyebrow={TEXT.settingsEyebrow}>
                <form className={styles.form} onSubmit={(event) => { void submitRename(event); }}>
                  <TextField
                    name="display_name"
                    label={TEXT.displayNameLabel}
                    /*
                      `reveal` is correct here and nowhere else on this screen:
                      the operator is editing the value, so the field needs the
                      original. React sets an input value as a property, never
                      as parsed markup, so nothing is interpreted on the way in.
                    */
                    defaultValue={reveal(record.display_name)}
                    required
                    maxLength={128}
                    {...(renameReason === undefined ? {} : { disabledReason: renameReason })}
                  />
                  <Button
                    variant="secondary"
                    type="submit"
                    pending={rename.isPending}
                    {...(renameReason === undefined ? {} : { disabledReason: renameReason })}
                  >
                    {TEXT.saveName}
                  </Button>
                </form>
              </Panel>
            </div>
          </>
        )}
      </DataBoundary>
      <SecretDialog secret={secret} onDismiss={() => setSecret(null)} />
    </div>
  );
}
