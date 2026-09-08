import { InlineMessage } from '@shared/ui';
import { consoleErrorText, type ConsoleError } from '@shared/api/error';
import { t } from '@shared/text';

/**
 * The form-level half of a rejection (WCX-11 sections 9.2 and 9.9).
 *
 * A rejection reaches an operator in one of two places. When change proposal
 * 0004 names a field this form renders, the message goes on that control and
 * this renders nothing more than the taxonomy sentence. When the backend names
 * a field this screen does not have, that is surfaced here rather than dropped:
 * the proposal's failure behaviour is explicit that a mismatch between backend
 * and console must never hide a rejection reason, because an operator left with
 * a failure and no cause has nothing to act on.
 *
 * The id is supplied by `useConsoleForm` so a field covered by a form-level
 * message can point at it through `invalidatedBy`, and the message is announced
 * once rather than once per field.
 */
export function FormMessage({
  error,
  unattached,
  id,
}: {
  error: ConsoleError | undefined;
  unattached: boolean;
  id: string;
}) {
  if (error === undefined) return null;
  return (
    <div id={id}>
      <InlineMessage tone="error">{consoleErrorText(error.kind)}</InlineMessage>
      {unattached && <InlineMessage tone="error">{t('errors.field.unattached')}</InlineMessage>}
    </div>
  );
}
