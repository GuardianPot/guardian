import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { deviceEncoding } from '@shared/theme/statusEncoding';
import { deviceQuery } from './api';
import { deviceHealthQuery, HealthPanel, HEALTH_TEXT, formatTime } from '@features/health';
import { DataBoundary, DescriptionList, StatusBadge } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { DEVICE_TEXT as TEXT } from './text';

export function DevicePage() {
  const { environmentId = '', deviceId = '' } = useParams();
  const device = useQuery(deviceQuery(environmentId, deviceId));
  const health = useQuery(deviceHealthQuery(deviceId));
  return (
    <div>
      <Link className={styles.backLink} to={`/environments/${environmentId}`}>{TEXT.back}</Link>
      <DataBoundary
        query={device}
        subject={{
          name: TEXT.subject,
          dependency: TEXT.dependency,
          stillWorks: TEXT.stillWorks,
          doesNotWork: TEXT.doesNotWork,
          staleReason: TEXT.staleReason,
        }}
        onRetry={() => { void device.refetch(); }}
      >
        {(record) => (
          <>
            <header className={styles.pageHeader}>
              <div>
                <p className={styles.eyebrow}>{TEXT.eyebrow}</p>
                <h1 tabIndex={-1}>{record.display_name}</h1>
                <p className={styles.mono}>{record.device_id}</p>
              </div>
              <StatusBadge encoding={deviceEncoding(record.state)} dimension={TEXT.inventoryDimension} />
            </header>
            <DescriptionList
              label={TEXT.factsLabel}
              entries={[
                { term: TEXT.inventoryState, value: record.state },
                { term: TEXT.recordUpdated, value: formatTime(record.updated_at) },
                {
                  term: TEXT.certificateExpiry,
                  value: record.active_certificate_expires_at
                    ? formatTime(record.active_certificate_expires_at)
                    : TEXT.noCertificate,
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
                name: HEALTH_TEXT.deviceSubject,
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
          </>
        )}
      </DataBoundary>
    </div>
  );
}
