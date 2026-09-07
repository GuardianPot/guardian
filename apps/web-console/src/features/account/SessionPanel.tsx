import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toConsoleError } from '@shared/api/error';
import type { Session } from '@shared/api/types';
import { useAuth, useCapability, useStepUp } from '@features/auth';
import type { StepUp } from '@features/auth';
import {
  Button,
  ConfirmationDialog,
  DataBoundary,
  InlineMessage,
  Panel,
  Timestamp,
  confirmationFor,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';
import { accountInvalidation, revokeSession, sessionsQuery } from './api';

/**
 * The owner's sessions (WCX-09 sections 8.6, 9.4, and 9.5.3).
 *
 * The distinction the whole panel turns on: **this** session and every other
 * one. Revoking another session must leave this one untouched, and this one
 * carries no revoke control at all — signing out is the existing, correct way
 * to end it, and offering two paths to the same effect invites an operator to
 * revoke themselves while meaning to revoke a stolen session.
 *
 * Revoked sessions stay listed. A session that ended is history an operator
 * investigating an intrusion needs, and removing it would hide exactly the row
 * that matters.
 *
 * Every timestamp is at second precision: session ordering is what an
 * incident reconstruction is made of.
 */
export function SessionPanel() {
  const sessions = useQuery(sessionsQuery());
  const stepUp = useStepUp();

  return (
    <Panel
      heading={t('account.sessions.heading')}
      headingLevel={2}
      eyebrow={t('account.sessions.eyebrow')}
    >
      <p>{t('account.sessions.intro')}</p>
      <DataBoundary
        query={sessions}
        subject={{
          name: t('account.sessions.collection'),
          dependency: t('common.controlPlane'),
          stillWorks: t('account.sessions.stillWorks'),
          doesNotWork: t('account.sessions.doesNotWork'),
          staleReason: t('account.sessions.staleReason'),
        }}
        onRetry={() => { void sessions.refetch(); }}
      >
        {(rows) => <SessionTable rows={rows} stepUp={stepUp} />}
      </DataBoundary>
      {stepUp.element}
    </Panel>
  );
}

/** What a row's state is, in the order the checks have to happen. */
function sessionState(session: Session): 'current' | 'revoked' | 'active' {
  if (session.current) return 'current';
  return session.revoked_at === undefined ? 'active' : 'revoked';
}

function SessionTable({ rows, stepUp }: { rows: readonly Session[]; stepUp: StepUp }) {
  const auth = useAuth();
  const client = useQueryClient();
  const capability = useCapability(confirmationFor('session.revoke').capability);
  const [confirming, setConfirming] = useState<Session | null>(null);
  const [pending, setPending] = useState<string | null>(null);
  const [failed, setFailed] = useState('');

  async function revoke(session: Session) {
    setConfirming(null);
    setFailed('');
    if (!auth.csrf) {
      setFailed(t('account.sessions.reauthenticate'));
      return;
    }
    setPending(session.session_id);
    try {
      await revokeSession(session.session_id, auth.csrf);
      await accountInvalidation.afterSessionChange(client);
    } catch (caught) {
      // Nothing changed on the target: section 9.5.5 requires saying so
      // rather than leaving an operator unsure whether access was removed.
      setFailed(
        toConsoleError(caught).kind === 'forbidden'
          ? t('account.sessions.revokeDenied')
          : t('account.sessions.revokeFailed'),
      );
    } finally {
      setPending(null);
    }
  }

  return (
    <>
      <table className={styles.dataTable}>
        <caption>{t('account.sessions.caption')}</caption>
        <thead>
          <tr>
            <th scope="col">{t('account.sessions.columnSession')}</th>
            <th scope="col">{t('account.sessions.columnCreated')}</th>
            <th scope="col">{t('account.sessions.columnLastSeen')}</th>
            <th scope="col">{t('account.sessions.columnExpires')}</th>
            <th scope="col">{t('account.sessions.columnAction')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((session) => {
            const state = sessionState(session);
            return (
              <tr key={session.session_id}>
                <th scope="row">
                  <span className={styles.mono}>{session.session_id}</span>
                  {state === 'current' && (
                    <span className={styles.currentSession}>{t('account.sessions.thisSession')}</span>
                  )}
                  {state === 'revoked' && (
                    <span className={styles.revokedSession}>{t('account.sessions.revoked')}</span>
                  )}
                </th>
                <td><Timestamp value={session.created_at} precision="second" /></td>
                <td><Timestamp value={session.last_seen_at} precision="second" /></td>
                <td><Timestamp value={session.expires_at} precision="second" /></td>
                <td>
                  {state === 'current' ? (
                    /*
                      Section 9.5.3 and 8.6: no revoke control on this session.
                      Sign-out in the sidebar already ends it, and a second
                      control for the same effect is a control an operator can
                      press by mistake while aiming at a stolen session.
                    */
                    <span className={styles.rowNote}>{t('account.sessions.useSignOut')}</span>
                  ) : (
                    <Button
                      variant="destructive"
                      pending={pending === session.session_id}
                      onClick={() => { setFailed(''); setConfirming(session); }}
                      {...(state === 'revoked'
                        ? { disabledReason: t('account.sessions.alreadyRevoked') }
                        : capability.allowed
                          ? {}
                          : { disabledReason: t('account.sessions.reauthenticate') })}
                    >
                      {/* Section 9.6.5: the action identifies its row. */}
                      {t('account.sessions.revokeNamed', { session: session.session_id })}
                    </Button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {failed !== '' && <InlineMessage tone="error">{failed}</InlineMessage>}
      {confirming !== null && (
        <ConfirmationDialog
          action="session.revoke"
          objectName={confirming.session_id}
          open
          stepUp={stepUp}
          onCancel={() => { setConfirming(null); }}
          onConfirm={() => { void revoke(confirming); }}
        />
      )}
    </>
  );
}
