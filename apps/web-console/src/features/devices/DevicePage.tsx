import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { deviceEncoding } from '@shared/theme/statusEncoding';
import { deviceQuery } from './api';
import { deviceHealthQuery, HealthPanel, formatTime } from '@features/health';
import { ErrorState, LoadingState } from '@shared/ui/Status';
import styles from '@shared/styles/app.module.css';

export function DevicePage() {
  const { environmentId = '', deviceId = '' } = useParams();
  const device = useQuery(deviceQuery(environmentId, deviceId));
  const health = useQuery(deviceHealthQuery(deviceId));
  if (device.isPending) return <LoadingState label="Loading device inventory…" />;
  if (device.isError || !device.data) return <ErrorState>The device record is unavailable or outside this environment.</ErrorState>;
  return (
    <div>
      <Link className={styles.backLink} to={`/environments/${environmentId}`}>← Environment overview</Link>
      <header className={styles.pageHeader}>
        <div><p className={styles.eyebrow}>Edge device</p><h1>{device.data.display_name}</h1><p className={styles.mono}>{device.data.device_id}</p></div>
        <span className={`${styles.statusBadge} ${styles[deviceEncoding(device.data.state).tone] ?? ''}`}>Inventory: {deviceEncoding(device.data.state).label}</span>
      </header>
      <section className={styles.factGrid} aria-label="Device inventory facts">
        <div><span>Inventory state</span><strong>{device.data.state}</strong></div>
        <div><span>Record updated</span><strong>{formatTime(device.data.updated_at)}</strong></div>
        <div><span>Active certificate expiry</span><strong>{device.data.active_certificate_expires_at ? formatTime(device.data.active_certificate_expires_at) : 'No active certificate'}</strong></div>
      </section>
      {health.isPending && <LoadingState label="Loading backend health projection…" />}
      {health.data && <HealthPanel health={health.data} />}
      {health.isError && <ErrorState>No current device health projection exists. Inventory presence is not a healthy signal.</ErrorState>}
    </div>
  );
}
