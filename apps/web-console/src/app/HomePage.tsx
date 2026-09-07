import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';
import { environmentQuery } from '@features/environments';
import { DataBoundary, Panel, UntrustedText } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';
import { useScope } from './scope';
import { NotFoundScreen } from './NotFoundPage';

/**
 * The incident-first root, before there are incidents (WCX-10 section 9.4.1).
 *
 * `UX-01` makes an incident dashboard the primary screen and `WC-D14` moves
 * the root here now, so Phase 3 adds a screen instead of rebuilding the frame.
 * Until then this is a placeholder, and the whole difficulty is that a
 * placeholder on an incident dashboard is dangerous.
 *
 * **It must not be readable as a statement about detections.** No empty
 * incident list, no zero count, no "nothing to report", no reassuring
 * green anything. An operator who glanced at this and came away believing
 * Guardian had looked and found nothing would be worse off than one who never
 * opened it — that is the exact failure mode this product exists to prevent,
 * reproduced in its own shell. So it says what it is: the dashboard is not
 * built yet, and here are the two screens that do work.
 *
 * `homePlaceholder.test.tsx` asserts the negative directly.
 */
export function HomePage() {
  const { scope } = useScope();

  // Section 8.2 and 9.7.1: a parameter that is not an identifier is not
  // silently ignored, and does not fall back to anything.
  if (scope.state === 'malformed') return <NotFoundScreen invalidScope />;

  return (
    <div>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>{t('home.eyebrow')}</p>
          <h1 tabIndex={-1}>{t('home.heading')}</h1>
        </div>
      </header>
      <Panel heading={t('home.pendingHeading')} headingLevel={2} eyebrow={t('home.pendingEyebrow')}>
        {/*
          Two sentences, both about the console rather than about the network.
          Nothing here counts, summarises, or reassures.
        */}
        <p>{t('home.pendingBody')}</p>
        <p>{t('home.pendingScope')}</p>
        <nav className={styles.homeLinks} aria-label={t('home.entryPoints')}>
          <Link to="/environments">{t('environments.heading')}</Link>
          <Link to="/account">{t('account.heading')}</Link>
        </nav>
      </Panel>
      {scope.state === 'selected' && <ScopedEnvironment environmentId={scope.environmentId} />}
    </div>
  );
}

/**
 * What the selected scope actually resolves to.
 *
 * This is the scope parameter's only current consumer, and it is here so the
 * refusal rules are exercised by something real: a denied environment renders
 * `denied` and an unknown one renders `not-found`, both through the `WCX-04`
 * boundary, and neither shows a different environment (section 9.7.1).
 */
function ScopedEnvironment({ environmentId }: { environmentId: string }) {
  const environment = useQuery(environmentQuery(environmentId));
  return (
    <Panel heading={t('home.scopeHeading')} headingLevel={2} eyebrow={t('home.scopeEyebrow')}>
      <DataBoundary
        query={environment}
        subject={{
          name: t('home.scopeSubject'),
          dependency: t('common.controlPlane'),
          stillWorks: t('home.scopeStillWorks'),
          doesNotWork: t('home.scopeDoesNotWork'),
          staleReason: t('home.scopeStaleReason'),
        }}
        onRetry={() => { void environment.refetch(); }}
      >
        {(record) => (
          <p>
            <Link to={`/environments/${record.environment_id}`}>
              <UntrustedText value={record.display_name} />
            </Link>
          </p>
        )}
      </DataBoundary>
    </Panel>
  );
}
