import { Link } from 'react-router';
import { reveal } from '@shared/api/untrusted';
import { decoyDesiredEncoding, decoyObservedEncoding } from '@shared/theme/statusEncoding';
import { Button, StatusBadge, Timestamp, UntrustedText, formatAge } from '@shared/ui';
import { t } from '@shared/text';
import styles from '@shared/styles/app.module.css';
import type { DecoyView } from '@shared/api/types';
import { convergenceOf, hasBeenObserved } from './convergence';

/**
 * The decoy list (WCX-11 section 9.3, UX-06).
 *
 * The load-bearing decision is that **observed** and **configuration** are two
 * columns. An operator wants one number and the console must not give them one:
 * "the Edge applied revision 4" and "the decoy is answering on the network" are
 * different claims, and a product whose whole pitch is honest coverage cannot
 * merge them into a tick.
 *
 * `last interaction` renders `Unknown` when nothing has reported, never
 * `Never`. `Never` is a claim that no attacker has touched this decoy, which
 * nobody has established — the observation itself is missing (section 9.3.2).
 */
export function DecoyTable({
  environmentID,
  decoys,
  onEnable,
  onDisable,
  onRemove,
  pendingDecoyID,
  canEnable,
  canRemove,
  disabledReason,
}: {
  environmentID: string;
  decoys: readonly DecoyView[];
  onEnable: (view: DecoyView) => void;
  onDisable: (view: DecoyView) => void;
  onRemove: (view: DecoyView) => void;
  pendingDecoyID: string | null;
  canEnable: boolean;
  canRemove: boolean;
  disabledReason: string;
}) {
  return (
    <table className={styles.dataTable}>
      <caption>{t('decoys.caption')}</caption>
      <thead>
        <tr>
          <th scope="col">{t('decoys.column.name')}</th>
          <th scope="col">{t('decoys.column.kind')}</th>
          <th scope="col">{t('decoys.column.placement')}</th>
          <th scope="col">{t('decoys.column.observed')}</th>
          <th scope="col">{t('decoys.column.convergence')}</th>
          <th scope="col">{t('decoys.column.version')}</th>
          <th scope="col">{t('decoys.column.interaction')}</th>
          <th scope="col">{t('decoys.column.actions')}</th>
        </tr>
      </thead>
      <tbody>
        {decoys.map((view) => {
          const { decoy, observed } = view;
          const name = reveal(decoy.display_name);
          const convergence = convergenceOf(view);
          const busy = pendingDecoyID === decoy.decoy_id;
          const enabled = decoy.desired_state === 'deployed';
          return (
            <tr key={decoy.decoy_id}>
              <th scope="row">
                <Link to={`/environments/${environmentID}/decoys/${decoy.decoy_id}`}>
                  {/* Operator-supplied, so it is never rendered directly. */}
                  <UntrustedText value={decoy.display_name} />
                </Link>
              </th>
              <td>
                {/* Closed tokens, so they may be shown as categories. The
                    persona is labelled as an emulation because AC-SMB-002
                    forbids implying a real host of this kind exists. */}
                {decoy.family} · {decoy.persona}
                <span className={styles.rowNote}>{t('decoys.persona.emulatedLabel')}</span>
              </td>
              <td>
                <code>{decoy.address}</code>
              </td>
              <td>
                <StatusBadge encoding={decoyObservedEncoding(observed.observed_state)} />
                {observed.observed_state === 'unmanaged' && (
                  <span className={styles.rowNote}>{t('decoys.observed.unmanaged')}</span>
                )}
                {!hasBeenObserved(view) && observed.observed_state !== 'unmanaged' && (
                  <span className={styles.rowNote}>{t('decoys.observed.neverObserved')}</span>
                )}
              </td>
              <td>
                <StatusBadge encoding={decoyDesiredEncoding(decoy.desired_state)} />
                <span className={styles.rowNote}>{convergenceNote(convergence, decoy.updated_at)}</span>
              </td>
              <td>{decoy.pack_version}</td>
              <td>
                {observed.last_interaction_at === null || observed.last_interaction_at === undefined ? (
                  t('decoys.observed.interactionUnknown')
                ) : (
                  <Timestamp value={observed.last_interaction_at} precision="second" />
                )}
              </td>
              <td>
                <Button
                  variant="quiet"
                  pending={busy}
                  onClick={() => { (enabled ? onDisable : onEnable)(view); }}
                  {...(canEnable ? {} : { disabledReason })}
                >
                  {enabled
                    ? t('decoys.action.disableNamed', { decoy: name })
                    : t('decoys.action.enableNamed', { decoy: name })}
                </Button>
                <Button
                  variant="destructive"
                  pending={busy}
                  onClick={() => { onRemove(view); }}
                  {...(canRemove ? {} : { disabledReason })}
                >
                  {t('decoys.action.removeNamed', { decoy: name })}
                </Button>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

/**
 * The one sentence under the configuration badge.
 *
 * `overdue` deliberately reports a wait rather than a failure. Guardian has not
 * been told anything went wrong; it has been told nothing at all, and section
 * 9.5 forbids turning that into an invented timeout.
 */
export function convergenceNote(
  convergence: ReturnType<typeof convergenceOf>,
  changedAt: string,
): string {
  switch (convergence.state) {
    case 'converged':
      return t('decoys.convergence.converged');
    case 'pending':
      return t('decoys.convergence.pendingFor', {
        age: formatAge(changedAt),
        desired: convergence.desiredRevision,
      });
    case 'overdue':
      return t('decoys.convergence.overdue', {
        age: formatAge(changedAt),
        desired: convergence.desiredRevision,
      });
    case 'unmanaged':
      return t('decoys.convergence.unmanaged');
  }
}
