import { useRef, useState } from 'react';
import { untrusted } from '@shared/api/untrusted';
import { HOSTILE_CORPUS } from '@shared/hostile/corpus';
import {
  configEncoding,
  deviceEncoding,
  healthEncoding,
  severityEncoding,
  CONFIDENCE_ORDER,
  ENCODING_TABLES,
} from '@shared/theme/statusEncoding';
import {
  Banner,
  Button,
  ConfidenceMeter,
  ConfirmationDialog,
  DegradedState,
  DeniedState,
  DescriptionList,
  Dialog,
  EmptyState,
  ErrorState,
  InlineMessage,
  LoadingState,
  Panel,
  PartialState,
  PendingOnObject,
  Skeleton,
  StaleState,
  StatusBadge,
  TextField,
  ToastRegion,
  UnknownState,
  UntrustedBlock,
  UntrustedText,
  useToasts,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import workbench from './workbench.module.css';

/**
 * The component workbench (WCX-06 section 9.5, decision WC-D27).
 *
 * A repository-local development route rather than Storybook: one file, no new
 * toolchain, no second build, and it renders the same components the console
 * renders, from the same source, so it cannot drift from what ships.
 *
 * It is fixture-only and performs no network call. Its reason for existing is
 * that some states are hard to reach in the running product — a `partial` read
 * needs one source to fail while another succeeds, a `degraded` read needs an
 * upstream to be down — and a state nobody can look at is a state nobody
 * checks.
 *
 * **It must never reach a production build.** Three independent proofs:
 * `router.tsx` mounts it behind `import.meta.env.DEV`, which Vite replaces
 * with `false` so Rollup drops the dynamic import; `check-bundle.mjs` asserts
 * no production chunk contains its marker or a corpus fixture id; and a
 * browser scenario asks a production build for `/__components` and asserts it
 * gets the ordinary SPA fallback.
 */
export const WORKBENCH_MARKER = 'guardian-component-workbench';

const OBSERVED_AT = '2026-09-01T12:00:00.000Z';

export function Workbench() {
  const toasts = useToasts();
  const cancel = useRef<HTMLButtonElement>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  return (
    <main className={styles.main} data-workbench={WORKBENCH_MARKER} tabIndex={-1}>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Development only</p>
          <h1 tabIndex={-1}>Component workbench</h1>
          <p className={styles.mono}>Fixture data only. This route performs no network call and never ships.</p>
        </div>
      </header>

      <Panel heading="Status encoding" headingLevel={2} eyebrow="WCX-03">
        <p>Every value in every table, including the unknown fallback.</p>
        <div className={workbench.row}>
          {Object.entries(ENCODING_TABLES).map(([table, values]) =>
            Object.keys(values).map((value) => (
              <StatusBadge
                key={`${table}-${value}`}
                encoding={
                  table === 'HEALTH' ? healthEncoding(value)
                    : table === 'SEVERITY' ? severityEncoding(value)
                      : table === 'DEVICE' ? deviceEncoding(value)
                        : configEncoding(value)
                }
              />
            )),
          )}
          <StatusBadge encoding={healthEncoding('a value the backend added later')} />
        </div>
        <div className={workbench.row}>
          {CONFIDENCE_ORDER.map((value) => <ConfidenceMeter key={value} value={value} />)}
          <ConfidenceMeter value="unrecognised" />
        </div>
      </Panel>

      <Panel heading="Data states" headingLevel={2} eyebrow="WCX-04">
        <p>All eight, in the order the mapping table resolves them.</p>
        <LoadingState activity="Loading the device inventory" skeletonLines={2} />
        <EmptyState collection="Edge devices" action={<Button variant="secondary">Enroll the first Edge</Button>} />
        <UnknownState subject="the health of this device" observationSource="an enrolled Edge reports its eight health conditions" />
        <StaleState observedAt={OBSERVED_AT} reason="the channel is closed">
          <p>the last data Guardian received</p>
        </StaleState>
        <PartialState unavailable={['device health', 'zone list']} onRetry={() => undefined}>
          <p>the sources that answered</p>
        </PartialState>
        <DegradedState dependency="The health projection" stillWorks="inventory and configuration" doesNotWork="every health condition" onRetry={() => undefined} />
        <DeniedState reauthenticate />
        <ErrorState retryable onRetry={() => undefined} />
      </Panel>

      <Panel heading="Controls" headingLevel={2} eyebrow="WCX-04">
        <div className={workbench.row}>
          <Button variant="primary">Primary</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="destructive">Destructive</Button>
          <Button variant="quiet">Quiet</Button>
          <Button variant="primary" pending>Pending</Button>
          <Button variant="secondary" disabledReason="Re-authenticate before changing configuration.">Disabled</Button>
        </div>
        <TextField name="plain" label="Plain field" description="A description that is always associated." />
        <TextField name="invalid" label="Rejected field" error="Use a canonical, non-overlapping RFC1918 CIDR." />
        <TextField name="locked" label="Unavailable field" disabledReason="Re-authenticate before adding a zone." />
        <Skeleton lines={3} />
        <DescriptionList
          label="Workbench facts"
          entries={[
            { term: 'Inventory state', value: 'active' },
            { term: 'Active certificate expiry', value: 'No active certificate' },
          ]}
        />
        <PendingOnObject startedAt={OBSERVED_AT} reason="waiting for the Edge to acknowledge" />
      </Panel>

      <Panel heading="Feedback" headingLevel={2} eyebrow="WCX-04">
        <Banner tone="informational">An informational condition, announced politely.</Banner>
        <Banner tone="blocking">A blocking condition, announced as an alert.</Banner>
        <Banner tone="restricted">Read-only session restored.</Banner>
        <InlineMessage tone="error">A field-scoped error.</InlineMessage>
        <InlineMessage tone="success">A field-scoped result.</InlineMessage>
        <div className={workbench.row}>
          <Button variant="secondary" onClick={() => toasts.show('A completed action.')}>Raise a toast</Button>
          <Button variant="secondary" onClick={() => setDialogOpen(true)}>Open a dialog</Button>
          <Button variant="destructive" onClick={() => setConfirmOpen(true)}>Open a level 3 confirmation</Button>
        </div>
      </Panel>

      <Panel heading="Hostile content" headingLevel={2} eyebrow="WCX-06">
        <p>
          The permanent corpus, rendered through the untrusted contract. Nothing here executes,
          links, or reorders. Each entry names what an attacker is trying to achieve.
        </p>
        {HOSTILE_CORPUS.map((fixture) => (
          <section key={fixture.id} className={workbench.fixture}>
            <p className={styles.eyebrow}>{fixture.id}</p>
            <p>{fixture.intent}</p>
            <p><UntrustedText value={untrusted(fixture.value)} /></p>
            <UntrustedBlock value={untrusted(fixture.value)} label={`Captured value: ${fixture.id}`} />
          </section>
        ))}
      </Panel>

      <Dialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        title="A dialog"
        description="With a name, a description, a focus trap, and focus return."
        initialFocus={cancel}
        actions={<Button variant="quiet" buttonRef={cancel} onClick={() => setDialogOpen(false)}>Close</Button>}
      >
        <p>Fixture content.</p>
      </Dialog>

      <ConfirmationDialog
        action="device.revoke"
        objectName="edge-one"
        open={confirmOpen}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={() => setConfirmOpen(false)}
      />

      <ToastRegion toasts={toasts.toasts} onDismiss={toasts.dismiss} />
    </main>
  );
}
