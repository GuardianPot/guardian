import type { HealthStatus, HealthView } from '../api/types';
import styles from '../styles/app.module.css';

const labels: Record<string, string> = {
  edge_connected: 'Edge connection',
  device_certificate_ready: 'Device certificate',
  config_converged: 'Configuration convergence',
  local_database_healthy: 'Local database',
  spool_healthy: 'Event spool',
  clock_quality: 'Clock quality',
  container_runtime_reachable: 'Container runtime',
  privileged_helper_reachable: 'Privileged helper',
};

function statusLabel(status: HealthStatus) {
  if (status === 'True') return 'Healthy';
  if (status === 'False') return 'Action required';
  return 'Unknown';
}

export function HealthPanel({ health }: { health: HealthView }) {
  const aggregate = health.aggregate.status;
  return (
    <section className={styles.panel} aria-labelledby="health-heading">
      <div className={styles.panelHeading}>
        <div>
          <p className={styles.eyebrow}>Backend health projection</p>
          <h2 id="health-heading">Eight-condition health</h2>
        </div>
        <span className={`${styles.statusBadge} ${styles[`health${aggregate}`]}`}>
          {statusLabel(aggregate)}
        </span>
      </div>
      {health.aggregate.blocking_type && (
        <p className={styles.blocking}>
          Blocking: {labels[health.aggregate.blocking_type] ?? health.aggregate.blocking_type}
          {health.aggregate.reason ? ` — ${health.aggregate.reason}` : ''}
          {health.aggregate.blocking_device_id ? ` · source ${health.aggregate.blocking_device_id}` : ''}
        </p>
      )}
      <ul className={styles.conditionGrid} aria-label="Device health conditions">
        {health.conditions.map((condition) => (
          <li key={condition.type} className={styles.condition}>
            <span className={`${styles.conditionDot} ${styles[`health${condition.status}`]}`} aria-hidden="true" />
            <div>
              <strong>{labels[condition.type] ?? condition.type}</strong>
              <span>{statusLabel(condition.status)} · {condition.reason}</span>
              {condition.message && <p>{condition.message}</p>}
              {condition.source_device_id && <small>Source device: {condition.source_device_id}</small>}
            </div>
          </li>
        ))}
      </ul>
      <p className={styles.timestamp}>Control Plane received this projection {formatTime(health.received_at)}.</p>
    </section>
  );
}

export function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
