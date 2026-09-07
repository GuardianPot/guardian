import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toConsoleError } from '@shared/api/error';
import { reveal } from '@shared/api/untrusted';
import type { EnrollmentToken } from '@shared/api/types';
import { useAuth, useCapability, useStepUp } from '@features/auth';
import {
  Button,
  ConfirmationDialog,
  DataBoundary,
  InlineMessage,
  Panel,
  Timestamp,
  UntrustedText,
  confirmationFor,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, type PlainCatalogueKey } from '@shared/text';
import {
  enrollmentTokenState,
  enrollmentTokensQuery,
  environmentInvalidation,
  revokeEnrollmentToken,
  type EnrollmentTokenState,
} from './api';

/**
 * Active and closed enrollment handoffs (WCX-09 section 9.4).
 *
 * **No token value is listed, ever** (section 8.9). The contract's summary has
 * no field for one, so this is not a filter the console applies — it is a
 * secret the console is never sent. The one-time value is shown exactly once,
 * at creation, by `OneTimeSecretDialog`.
 *
 * A consumed, revoked, or expired token stays in the list (section 9.5.4).
 * Removing it would hide that a handoff window closed, which is the first
 * thing an operator wants when an Edge failed to enrol.
 *
 * A semantic table with a caption and header cells (section 9.6.5), because
 * this is tabular: four facts about each of several rows. Every row action
 * names its row, so "Revoke" alone is never an accessible name.
 */
const STATE_LABEL: Readonly<Record<EnrollmentTokenState, PlainCatalogueKey>> = {
  active: 'environment.tokens.stateActive',
  consumed: 'environment.tokens.stateConsumed',
  revoked: 'environment.tokens.stateRevoked',
  expired: 'environment.tokens.stateExpired',
};

export function EnrollmentTokenPanel({ environmentID }: { environmentID: string }) {
  const tokens = useQuery(enrollmentTokensQuery(environmentID));
  const stepUp = useStepUp();

  return (
    <Panel
      heading={t('environment.tokens.heading')}
      headingLevel={2}
      eyebrow={t('environment.tokens.eyebrow')}
    >
      <p>{t('environment.tokens.intro')}</p>
      <DataBoundary
        query={tokens}
        subject={{
          name: t('environment.tokens.collection'),
          dependency: t('common.controlPlane'),
          stillWorks: t('environment.tokens.stillWorks'),
          doesNotWork: t('environment.tokens.doesNotWork'),
          staleReason: t('environment.tokens.staleReason'),
        }}
        onRetry={() => { void tokens.refetch(); }}
      >
        {(rows) => (
          <TokenTable environmentID={environmentID} rows={rows} stepUp={stepUp} />
        )}
      </DataBoundary>
      {stepUp.element}
    </Panel>
  );
}

function TokenTable({
  environmentID,
  rows,
  stepUp,
}: {
  environmentID: string;
  rows: readonly EnrollmentToken[];
  stepUp: ReturnType<typeof useStepUp>;
}) {
  const auth = useAuth();
  const client = useQueryClient();
  const capability = useCapability(confirmationFor('enrollment.revoke').capability);
  const [confirming, setConfirming] = useState<EnrollmentToken | null>(null);
  const [pending, setPending] = useState<string | null>(null);
  const [failed, setFailed] = useState('');

  async function revoke(token: EnrollmentToken) {
    setConfirming(null);
    setFailed('');
    if (!auth.csrf) {
      setFailed(t('environment.tokens.reauthenticate'));
      return;
    }
    setPending(token.token_id);
    try {
      await revokeEnrollmentToken(environmentID, token.token_id, auth.csrf);
      await environmentInvalidation.afterTokenRevoke(client, environmentID);
    } catch (caught) {
      // A `409` means the token was already consumed or revoked — the state
      // changed under the operator, and nothing was applied.
      setFailed(
        toConsoleError(caught).kind === 'conflict'
          ? t('environment.tokens.alreadyClosed')
          : t('environment.tokens.revokeFailed'),
      );
    } finally {
      setPending(null);
    }
  }

  return (
    <>
      <table className={styles.dataTable}>
        <caption>{t('environment.tokens.caption')}</caption>
        <thead>
          <tr>
            <th scope="col">{t('environment.tokens.columnDevice')}</th>
            <th scope="col">{t('environment.tokens.columnState')}</th>
            <th scope="col">{t('environment.tokens.columnExpires')}</th>
            <th scope="col">{t('environment.tokens.columnAction')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((token) => {
            const state = enrollmentTokenState(token);
            const name = reveal(token.device_name);
            return (
              <tr key={token.token_id}>
                {/*
                  The device name comes from the operator through the API, so
                  the console cannot tell it from an attacker with a stolen
                  session. It renders through the untrusted contract.
                */}
                <th scope="row"><UntrustedText value={token.device_name} /></th>
                <td>{t(STATE_LABEL[state])}</td>
                <td><Timestamp value={token.expires_at} precision="second" /></td>
                <td>
                  <Button
                    variant="destructive"
                    pending={pending === token.token_id}
                    onClick={() => { setFailed(''); setConfirming(token); }}
                    {...(state !== 'active'
                      ? { disabledReason: t('environment.tokens.onlyActive') }
                      : capability.allowed
                        ? {}
                        : { disabledReason: t('environment.tokens.reauthenticate') })}
                  >
                    {/* Section 9.6.5: the action names its row, not just itself. */}
                    {t('environment.tokens.revokeNamed', { device: name })}
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {failed !== '' && <InlineMessage tone="error">{failed}</InlineMessage>}
      {confirming !== null && (
        <ConfirmationDialog
          action="enrollment.revoke"
          objectName={reveal(confirming.device_name)}
          open
          stepUp={stepUp}
          onCancel={() => { setConfirming(null); }}
          onConfirm={() => { void revoke(confirming); }}
        />
      )}
    </>
  );
}
