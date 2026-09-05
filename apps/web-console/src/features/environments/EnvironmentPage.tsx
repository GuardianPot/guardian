import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import { toConsoleError } from '@shared/api/error';
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
import { environmentHealthQuery, HealthPanel, formatTime } from '@features/health';
import { SecretDialog } from './SecretDialog';
import { EmptyState, ErrorState, LoadingState } from '@shared/ui/Status';
import styles from '@shared/styles/app.module.css';

export function EnvironmentPage() {
  const { environmentId = '' } = useParams();
  const auth = useAuth();
  const queryClient = useQueryClient();
  const [message, setMessage] = useState('');
  const [secret, setSecret] = useState<EnrollmentSecret | null>(null);
  const secretRef = useRef<EnrollmentSecret | null>(null);
  const [creatingSecret, setCreatingSecret] = useState(false);
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
    event.preventDefault(); setMessage('');
    if (!auth.csrf || !environment.data) return setMessage('Re-authenticate before changing this environment.');
    const form = event.currentTarget;
    try { await rename.mutateAsync(textField(new FormData(form), 'display_name')); setMessage('Environment name updated.'); }
    catch { setMessage('Rename failed. Refresh if another change was made first.'); }
  }

  async function submitZone(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setMessage('');
    if (!auth.csrf) return setMessage('Re-authenticate before adding a zone.');
    const form = event.currentTarget; const data = new FormData(form);
    try {
      await createZone.mutateAsync({ display_name: textField(data, 'display_name'), cidr: textField(data, 'cidr') });
      form.reset(); setMessage('Private network zone added.');
    } catch { setMessage('Zone creation failed. Use a canonical, non-overlapping RFC1918 CIDR.'); }
  }

  async function submitEnrollment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setMessage('');
    if (!auth.csrf) return setMessage('Re-authenticate before creating an enrollment secret.');
    const form = event.currentTarget;
    setCreatingSecret(true);
    try {
      const created = await createEnrollmentSecret(environmentId, textField(new FormData(form), 'device_name'), auth.csrf);
      form.reset(); setSecret(created);
      await environmentInvalidation.afterEnrollmentSecret(queryClient, environmentId);
    } catch { setMessage('Enrollment secret creation was denied.'); }
    finally { setCreatingSecret(false); }
  }

  if (environment.isPending) return <LoadingState label="Loading environment…" />;
  if (environment.isError || !environment.data) return <ErrorState>This environment is unavailable or access was denied.</ErrorState>;

  return (
    <div>
      <Link className={styles.backLink} to="/environments">← All environments</Link>
      <header className={styles.pageHeader}>
        <div><p className={styles.eyebrow}>Environment</p><h1>{environment.data.display_name}</h1><p className={styles.mono}>{environment.data.environment_id}</p></div>
        <span className={`${styles.statusBadge} ${styles[configEncoding(environment.data.status).tone] ?? ''}`}>
          {environment.data.status === 'zones_defined' ? 'Configuration complete' : 'Zones required'}
        </span>
      </header>
      {message && <p className={styles.notice} role="status">{message}</p>}
      <div className={styles.twoColumn}>
        <section className={styles.panel} aria-labelledby="inventory-heading">
          <div className={styles.panelHeading}><div><p className={styles.eyebrow}>Inventory truth</p><h2 id="inventory-heading">Edge devices</h2></div><span className={styles.panelCount}>{devices.data?.length ?? 0}</span></div>
          {devices.isPending && <LoadingState />}
          {devices.isError && <ErrorState>Device inventory could not be loaded.</ErrorState>}
          {devices.data?.length === 0 && <EmptyState>No device records yet. Create an enrollment secret to begin.</EmptyState>}
          <ul className={styles.deviceList}>
            {devices.data?.map((device) => (
              <li key={device.device_id}>
                <Link to={`/environments/${environmentId}/devices/${device.device_id}`}>
                  <span><strong>{device.display_name}</strong><small>{device.device_id}</small></span>
                  <span className={`${styles.statusBadge} ${styles[deviceEncoding(device.state).tone] ?? ''}`}>{deviceEncoding(device.state).label}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
        <section className={styles.panel} aria-labelledby="enroll-heading">
          <p className={styles.eyebrow}>One-time handoff</p><h2 id="enroll-heading">Enroll an Edge</h2>
          <p>Create a 15-minute secret for one named Edge. Device health remains unknown until the real Edge reports all conditions.</p>
          <form className={styles.form} onSubmit={(event) => { void submitEnrollment(event); }}>
            <label>Device name<input name="device_name" required maxLength={128} disabled={!enrollDevice.allowed || creatingSecret} /></label>
            <button className={styles.primaryButton} disabled={!enrollDevice.allowed || creatingSecret}>{creatingSecret ? 'Creating…' : 'Create one-time secret'}</button>
          </form>
        </section>
      </div>
      {health.data ? <HealthPanel health={health.data} /> : (
        <section className={styles.panel} aria-labelledby="health-unavailable-heading">
          <p className={styles.eyebrow}>Backend health projection</p><h2 id="health-unavailable-heading">Health not yet reported</h2>
          <p>{toConsoleError(health.error).kind === 'not-found' ? 'No active Edge has supplied a health projection.' : 'The current health projection is unavailable. This is not a healthy signal.'}</p>
        </section>
      )}
      <div className={styles.twoColumn}>
        <section className={styles.panel} aria-labelledby="zones-heading">
          <div className={styles.panelHeading}><h2 id="zones-heading">Private network zones</h2><span className={styles.panelCount}>{zones.data?.length ?? 0}</span></div>
          {zones.isPending && <LoadingState />}{zones.isError && <ErrorState />}
          {zones.data?.length === 0 && <EmptyState>No zones defined.</EmptyState>}
          <ul className={styles.zoneList}>{zones.data?.map((zone) => <li key={zone.zone_id}><span><strong>{zone.display_name}</strong><small>Updated {formatTime(zone.updated_at)}</small></span><code>{zone.cidr}</code></li>)}</ul>
          <form className={styles.inlineForm} onSubmit={(event) => { void submitZone(event); }}>
            <label>Zone name<input name="display_name" required maxLength={128} disabled={!defineZone.allowed} /></label>
            <label>Private CIDR<input name="cidr" placeholder="10.20.0.0/24" required maxLength={18} disabled={!defineZone.allowed} /></label>
            <button className={styles.secondaryButton} disabled={!defineZone.allowed || createZone.isPending}>Add zone</button>
          </form>
        </section>
        <section className={styles.panel} aria-labelledby="settings-heading">
          <p className={styles.eyebrow}>Configuration</p><h2 id="settings-heading">Environment settings</h2>
          <form className={styles.form} onSubmit={(event) => { void submitRename(event); }}>
            <label>Display name<input name="display_name" defaultValue={environment.data.display_name} required maxLength={128} disabled={!updateEnvironment.allowed} /></label>
            <button className={styles.secondaryButton} disabled={!updateEnvironment.allowed || rename.isPending}>Save name</button>
          </form>
        </section>
      </div>
      <SecretDialog secret={secret} onDismiss={() => setSecret(null)} />
    </div>
  );
}
