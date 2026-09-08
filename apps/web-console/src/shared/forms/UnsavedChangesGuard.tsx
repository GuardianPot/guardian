import { useEffect, useRef } from 'react';
import { useBlocker } from 'react-router';
import { Button, Dialog } from '@shared/ui';
import { t } from '@shared/text';

/**
 * Warns before leaving a form with unsaved edits (WCX-11 sections 9.1.5 and
 * 9.7.5).
 *
 * It warns. It does not save, and it does not remember: section 8.6 forbids
 * autosaving form state to browser storage, so there is deliberately no draft
 * to restore. An operator half-way through configuring a decoy has typed a
 * persona and an address that are attacker-visible by design; writing those to
 * `localStorage` would leave them on the machine after the tab closed, for a
 * convenience nobody asked for.
 *
 * Two exits are covered. In-app navigation goes through the router's blocker
 * and gets the dialog. A tab close or reload gets the browser's own prompt,
 * which cannot be styled or worded — `beforeunload` is the only hook, and the
 * browser ignores custom text. That is the platform's limit, not a choice.
 */
export function UnsavedChangesGuard({ when }: { when: boolean }) {
  const stay = useRef<HTMLButtonElement | null>(null);
  const blocker = useBlocker(({ currentLocation, nextLocation }) =>
    when && currentLocation.pathname !== nextLocation.pathname,
  );

  useEffect(() => {
    if (!when) return undefined;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener('beforeunload', warn);
    return () => { window.removeEventListener('beforeunload', warn); };
  }, [when]);

  if (blocker.state !== 'blocked') return null;
  return (
    <Dialog
      open
      onClose={() => { blocker.reset?.(); }}
      title={t('forms.unsaved.title')}
      description={t('forms.unsaved.body')}
      initialFocus={stay}
      actions={
        <>
          {/* Keep editing is the safe option and takes focus, matching the
              confirmation contract in WCX-04 section 9.3. */}
          <Button variant="secondary" buttonRef={stay} onClick={() => { blocker.reset?.(); }}>
            {t('forms.unsaved.stay')}
          </Button>
          <Button variant="destructive" onClick={() => { blocker.proceed?.(); }}>
            {t('forms.unsaved.discard')}
          </Button>
        </>
      }
    />
  );
}
