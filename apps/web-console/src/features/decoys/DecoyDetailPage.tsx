import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useParams } from 'react-router';
import { reveal } from '@shared/api/untrusted';
import { toConsoleError } from '@shared/api/error';
import { decoyDesiredEncoding, decoyObservedEncoding, healthEncoding } from '@shared/theme/statusEncoding';
import { useAuth, useCapability } from '@features/auth';
import { zonesQuery } from '@features/environments';
import {
  Banner,
  Breadcrumbs,
  Button,
  DataBoundary,
  DescriptionList,
  InlineMessage,
  Panel,
  StatusBadge,
  Timestamp,
  UntrustedText,
  confirmationFor,
  formatAge,
} from '@shared/ui';
import { t, type PlainCatalogueKey } from '@shared/text';
import styles from '@shared/styles/app.module.css';
import type { DecoyCondition, DecoyView, DecoyWriteRequest } from '@shared/api/types';
import { decoyInvalidation, decoyQuery, updateDecoy } from './api';
import { DecoyForm } from './DecoyForm';
import { convergenceOf, hasBeenObserved, observationAgeMs } from './convergence';

/**
 * The freshness class an observation is judged against.
 *
 * Deliberately generous. A decoy report is not a heartbeat, and calling an
 * observation stale the moment it stops being seconds old would train an
 * operator to ignore the word. What matters is that when it *is* old, the age
 * is stated rather than the observation being left to look current
 * (section 9.9.3).
 */
const OBSERVATION_STALE_AFTER_MS = 300_000;

const CONDITION_LABEL: Readonly<Record<string, PlainCatalogueKey>> = {
  runtime_healthy: 'decoys.condition.runtime_healthy',
  address_applied: 'decoys.condition.address_applied',
  port_responding: 'decoys.condition.port_responding',
  telemetry_reporting: 'decoys.condition.telemetry_reporting',
  policy_applied: 'decoys.condition.policy_applied',
  version_matches_desired: 'decoys.condition.version_matches_desired',
};

/**
 * One decoy in full (WCX-11 section 9.4).
 *
 * The header carries desired and observed as two badges and never resolves
 * them into one. Below, `Runtime state` reports the six dimensions an Edge
 * reports on, each with its own status, so killing the process, removing the
 * address, and breaking telemetry stay three distinguishable failures rather
 * than one word.
 *
 * Configuration is edited here, pessimistically, with the revision the
 * operator was looking at. A `412` renders as a conflict and offers the
 * current value; nothing is written on the strength of a stale form.
 */
export function DecoyDetailPage() {
  const { environmentId = '', decoyId = '' } = useParams();
  const auth = useAuth();
  const client = useQueryClient();
  const view = useQuery(decoyQuery(environmentId, decoyId));
  const zones = useQuery(zonesQuery(environmentId));
  const update = useCapability(confirmationFor('decoy.update').capability);

  const [editing, setEditing] = useState(false);
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);

  async function save(values: DecoyWriteRequest, current: DecoyView) {
    setNotice('');
    setConflict(false);
    if (!auth.csrf) throw new Error('missing csrf');
    try {
      await updateDecoy(environmentId, current.decoy, values, auth.csrf);
    } catch (caught) {
      // A conflict is not a validation failure and must not be rendered as
      // one: the form was right, the world moved (section 9.9.1).
      if (toConsoleError(caught).kind === 'conflict') {
        setConflict(true);
        setEditing(false);
        return;
      }
      throw caught;
    }
    await decoyInvalidation.afterDetailWrite(client, environmentId, decoyId);
    setEditing(false);
    setNotice(t('decoys.result.updated'));
  }

  return (
    <section>
      <Breadcrumbs trail={[{ label: t('decoys.detail.back'), to: `/environments/${environmentId}/decoys` }]} />
      {notice !== '' && <InlineMessage tone="success">{notice}</InlineMessage>}
      {conflict && (
        <div className={styles.zoneConflict} role="alert">
          <p>{t('decoys.result.conflict')}</p>
          <Button
            variant="secondary"
            onClick={() => {
              setConflict(false);
              void decoyInvalidation.afterDetailWrite(client, environmentId, decoyId);
            }}
          >
            {t('decoys.result.reload')}
          </Button>
        </div>
      )}

      <DataBoundary
        query={view}
        observationShaped
        subject={{
          name: t('decoys.title'),
          observationSource: t('decoys.observed.neverObserved'),
          dependency: t('common.controlPlane'),
          stillWorks: t('decoys.title'),
          doesNotWork: t('decoys.caption'),
          staleReason: t('decoys.observed.neverObserved'),
        }}
      >
        {(current) => {
          const { decoy, observed } = current;
          const convergence = convergenceOf(current);
          const ageMs = observationAgeMs(current);
          const stale = ageMs !== null && ageMs > OBSERVATION_STALE_AFTER_MS;
          return (
            <>
              <h1><UntrustedText value={decoy.display_name} /></h1>

              <p>
                <StatusBadge
                  encoding={decoyObservedEncoding(observed.observed_state)}
                  dimension={t('decoys.column.observed')}
                />
                <StatusBadge
                  encoding={decoyDesiredEncoding(decoy.desired_state)}
                  dimension={t('decoys.column.convergence')}
                />
              </p>

              {/* Section 9.5: the pending state lives on the detail header and
                  shows the desired revision, the last observed one, and how
                  long the wait has been. No blocking modal. */}
              <p className={styles.rowNote}>
                {convergence.state === 'converged'
                  ? t('decoys.convergence.converged')
                  : convergence.state === 'unmanaged'
                    ? t('decoys.convergence.unmanaged')
                    : t('decoys.convergence.pending', { desired: decoy.revision })}
              </p>
              <p className={styles.rowNote}>
                {observed.desired_revision === null || observed.desired_revision === undefined
                  ? t('decoys.convergence.neverApplied')
                  : t('decoys.convergence.lastApplied', { observed: observed.desired_revision })}
              </p>

              {observed.observed_state === 'unmanaged' && (
                <Banner tone="restricted">{t('decoys.observed.unmanaged')}</Banner>
              )}
              {!hasBeenObserved(current) && observed.observed_state !== 'unmanaged' && (
                <Banner tone="informational">{t('decoys.observed.neverObserved')}</Banner>
              )}
              {/* Section 9.9.3: an old observation says how old, rather than
                  being left to look current. */}
              {stale && ageMs !== null && observed.reported_at != null && (
                <Banner tone="restricted">
                  {t('decoys.observed.stale', { age: formatAge(observed.reported_at) })}
                </Banner>
              )}

              <Panel heading={t('decoys.section.identity')} headingLevel={2}>
                <DescriptionList
                  label={t('decoys.section.identity')}
                  entries={[
                    { term: t('decoys.field.displayName'), value: <UntrustedText value={decoy.display_name} /> },
                    { term: t('decoys.field.family'), value: decoy.family },
                    {
                      term: t('decoys.field.persona'),
                      // AC-SMB-002: the persona is named and immediately
                      // qualified as an emulation, in the same cell.
                      value: `${decoy.persona} · ${t('decoys.persona.emulatedLabel')}`,
                    },
                    { term: t('decoys.field.interactionLevel'), value: decoy.interaction_level },
                  ]}
                />
                <p className={styles.rowNote}>{t('decoys.persona.emulatedNote')}</p>
              </Panel>

              <Panel heading={t('decoys.section.placement')} headingLevel={2}>
                <DescriptionList
                  label={t('decoys.section.placement')}
                  entries={[
                    { term: t('decoys.field.address'), value: decoy.address },
                    { term: t('decoys.field.zone'), value: decoy.zone_id },
                  ]}
                />
              </Panel>

              <Panel heading={t('decoys.section.version')} headingLevel={2}>
                <DescriptionList
                  label={t('decoys.section.version')}
                  entries={[
                    { term: t('decoys.field.pack'), value: decoy.pack },
                    { term: t('decoys.field.packVersion'), value: decoy.pack_version },
                    {
                      term: t('decoys.field.packDigest'),
                      // Null until P2-W4 supplies a manifest to hash. Saying so
                      // is the honest rendering; a placeholder would assert an
                      // artefact identity nothing verified.
                      value: decoy.pack_digest ?? t('decoys.field.packDigestUnknown'),
                    },
                    { term: t('decoys.field.revision'), value: String(decoy.revision) },
                  ]}
                />
              </Panel>

              <Panel heading={t('decoys.section.runtime')} headingLevel={2}>
                {observed.conditions.length === 0 ? (
                  <p>{t('decoys.observed.noConditions')}</p>
                ) : (
                  <ul aria-label={t('decoys.observed.conditionsLabel')}>
                    {observed.conditions.map((condition) => (
                      <ConditionRow key={condition.type} condition={condition} />
                    ))}
                  </ul>
                )}
              </Panel>

              <Panel heading={t('decoys.section.interaction')} headingLevel={2}>
                <p>
                  {observed.last_interaction_at == null
                    ? t('decoys.observed.interactionUnknown')
                    : <Timestamp value={observed.last_interaction_at} precision="second" />}
                </p>
                {observed.reported_at != null && (
                  <p className={styles.rowNote}>
                    {t('decoys.observed.reported', { age: formatAge(observed.reported_at) })}
                  </p>
                )}
              </Panel>

              <Panel heading={t('decoys.section.configure')} headingLevel={2}>
                {editing ? (
                  <DecoyForm
                    zones={zones.data ?? []}
                    defaults={{
                      zone_id: decoy.zone_id,
                      display_name: reveal(decoy.display_name),
                      family: decoy.family,
                      persona: decoy.persona,
                      address: decoy.address,
                      pack: decoy.pack,
                      pack_version: decoy.pack_version,
                    }}
                    submitLabel={t('decoys.detail.save')}
                    onSubmit={(values) => save(values, current)}
                    onCancel={() => { setEditing(false); }}
                  />
                ) : (
                  <Button
                    variant="secondary"
                    onClick={() => { setNotice(''); setConflict(false); setEditing(true); }}
                    {...(update.allowed ? {} : { disabledReason: t('decoys.reauthenticate') })}
                  >
                    {t('decoys.detail.edit')}
                  </Button>
                )}
              </Panel>
            </>
          );
        }}
      </DataBoundary>
    </section>
  );
}

/**
 * One observed dimension.
 *
 * The reason and the message are Edge-supplied, so both render through the
 * untrusted contract. A dimension nothing reported stays `Unknown`; it is never
 * completed with a favourable value, which is the guarantee that keeps three
 * different failures from collapsing into one.
 */
function ConditionRow({ condition }: { condition: DecoyCondition }) {
  const label = CONDITION_LABEL[condition.type];
  return (
    <li>
      <StatusBadge
        encoding={healthEncoding(condition.status)}
        dimension={label === undefined ? condition.type : t(label)}
      />
      <span className={styles.rowNote}>
        {t('decoys.observed.reasonLabel')}: <UntrustedText value={condition.reason} />
      </span>
      {reveal(condition.message) !== '' && (
        <span className={styles.rowNote}>
          {t('decoys.observed.messageLabel')}: <UntrustedText value={condition.message} />
        </span>
      )}
      {condition.status === 'False' && (
        <span className={styles.rowNote}>{t('decoys.observed.nextAction')}</span>
      )}
    </li>
  );
}
