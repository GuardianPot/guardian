import { useQuery } from '@tanstack/react-query';
import { useEffect, useId } from 'react';
import { reveal } from '@shared/api/untrusted';
import { environmentsQuery } from '@features/environments';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';
import { useScope } from './scope';

/**
 * The environment scope selector (WCX-10 section 9.2, WC-D14).
 *
 * One query for the whole console: every consumer reads
 * `environmentsQuery()`, so the list is fetched once per freshness interval
 * however many components ask for it (section 9.9).
 *
 * The rule that shapes everything else here is section 9.2.3: **nothing is
 * auto-chosen among several environments.** A console that quietly picked the
 * first one would put an operator in front of a network they did not ask for,
 * and every subsequent screen would look correct. So with no parameter and
 * more than one environment, nothing is selected and scope-aware surfaces say
 * so.
 *
 * The one exception is section 9.2.4: with exactly one environment there is
 * no ambiguity to resolve, so it is preselected *and written to the URL* —
 * written, because a link that resolves differently depending on how many
 * environments the reader can see is not a deterministic link.
 */
export function ScopeSelector() {
  const environments = useQuery(environmentsQuery());
  const { scope, select } = useScope();
  const labelId = useId();

  const list = environments.data ?? [];
  const only = list.length === 1 ? list[0] : undefined;

  useEffect(() => {
    // Section 9.2.4, and only this case. `select` replaces rather than pushes,
    // so this does not add a history entry the operator has to press back
    // through.
    if (scope.state === 'absent' && only !== undefined) select(only.environment_id);
  }, [scope.state, only, select]);

  /** Why the selector cannot be used, or `undefined` when it can. */
  const unavailable = environments.isPending
    ? t('scope.loading')
    : environments.error !== null
      ? t('scope.unavailable')
      : list.length === 0
        ? t('scope.none')
        : undefined;

  return (
    <div className={styles.scope}>
      <label className={styles.scopeLabel} id={labelId} htmlFor="environment-scope">
        {t('scope.label')}
      </label>
      <select
        id="environment-scope"
        className={styles.scopeSelect}
        value={scope.state === 'selected' ? scope.environmentId : ''}
        disabled={unavailable !== undefined}
        aria-describedby={unavailable === undefined ? undefined : `${labelId}-reason`}
        onChange={(event) => { select(event.target.value === '' ? null : event.target.value); }}
      >
        {/*
          The empty option is the honest representation of "no environment
          chosen". Removing it would make the first environment look selected
          when nothing is.
        */}
        <option value="">{t('scope.unselected')}</option>
        {list.map((environment) => (
          <option key={environment.environment_id} value={environment.environment_id}>
            {/*
              A display name round-trips through the API, so it is untrusted.
              An `option` renders text and nothing else — the browser will not
              build an element from it — which is why this is the one place a
              revealed value is written directly rather than through
              `UntrustedText`. There is no markup for it to become.
            */}
            {reveal(environment.display_name)}
          </option>
        ))}
      </select>
      {/* WC-D07: disabled with a reason, never hidden (section 9.2.5). */}
      {unavailable !== undefined && (
        <span className={styles.scopeReason} id={`${labelId}-reason`}>{unavailable}</span>
      )}
    </div>
  );
}
