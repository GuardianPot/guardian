import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router';
import { deviceEncoding } from '@shared/theme/statusEncoding';
import { deviceQuery } from './api';
import { clockQualityIsDegraded, deviceHealthQuery, HealthPanel } from '@features/health';
import { DataBoundary, DescriptionList, StatusBadge, Timestamp, UntrustedText } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

export function DevicePage() {
  const { environmentId = '', deviceId = '' } = useParams();
  const device = useQuery(deviceQuery(environmentId, deviceId));
  const health = useQuery(deviceHealthQuery(deviceId));
  // A device whose clock is not established marks every time it supplies.
  const degradedClock = clockQualityIsDegraded(health.data);
  return (
    <div>
      <Link className={styles.backLink} to={`/environments/${environmentId}`}>{t('devices.back')}</Link>
      <DataBoundary
        query={device}
        subject={{
          name: t('devices.subject'),
          dependency: t('common.controlPlane'),
          stillWorks: t('devices.stillWorks'),
          doesNotWork: t('devices.doesNotWork'),
          staleReason: t('devices.staleReason'),
        }}
        onRetry={() => { void device.refetch(); }}
      >
        {(record) => (
          <>
            <header className={styles.pageHeader}>
              <div>
                <p className={styles.eyebrow}>{t('devices.eyebrow')}</p>
                <h1 tabIndex={-1}><UntrustedText value={record.display_name} /></h1>
                <p className={styles.mono}>{record.device_id}</p>
              </div>
              <StatusBadge encoding={deviceEncoding(record.state)} dimension={t('devices.inventoryDimension')} />
            </header>
            <DescriptionList
              label={t('devices.factsLabel')}
              entries={[
                { term: t('devices.inventoryState'), value: record.state },
                { term: t('devices.recordUpdated'), value: <Timestamp value={record.updated_at} uncertainClock={degradedClock} /> },
                {
                  term: t('devices.certificateExpiry'),
                  value: record.active_certificate_expires_at
                    ? <Timestamp value={record.active_certificate_expires_at} uncertainClock={degradedClock} />
                    : t('devices.noCertificate'),
                },
              ]}
            />
            {/*
              Inventory above, health below, never merged. An `active` record
              is an inventory fact; it is not evidence that anything is
              working, so a missing projection renders `unknown` rather than
              borrowing the inventory state's confidence.
            */}
            <DataBoundary
              query={health}
              observationShaped
              // OPS-03: an old projection is shown as stale, not as current.
              observedAt={health.data?.received_at ?? null}
              subject={{
                name: t('health.deviceSubject'),
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
          </>
        )}
      </DataBoundary>
    </div>
  );
}
