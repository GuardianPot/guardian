import { useRef, useState } from 'react';
import { MemoryRouter } from 'react-router';
import { configEncoding, deviceEncoding, healthEncoding, severityEncoding } from '@shared/theme/statusEncoding';
import {
  Banner,
  Breadcrumbs,
  Button,
  ConfidenceMeter,
  ConfirmationDialog,
  DataBoundary,
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
  RouteErrorBoundary,
  OneTimeSecretDialog,
  Skeleton,
  StaleState,
  StatusBadge,
  TextField,
  Timestamp,
  ToastRegion,
  UnknownState,
  UntrustedBlock,
  UntrustedText,
  useToasts,
} from '@shared/ui';
import { untrusted } from '@shared/api/untrusted';

/**
 * Every component in the layer, on one page, driven by one string.
 *
 * `storage.test.tsx` renders this and asserts that no storage area was
 * touched; `hostileContent.test.tsx` renders it with attacker-shaped strings
 * and asserts that none of them became markup. One exercise rather than two
 * means a component added later is covered by both the moment it is listed
 * here — and a component that is *not* listed shows up as a gap rather than as
 * silence, because `storage.test.tsx` compares this list against the layer's
 * exports.
 *
 * The two dialogs open from controls rather than on mount: a modal hides the
 * rest of the document from assistive technology, and two open at once would
 * hide everything this page exists to exercise.
 */
export const EXERCISED_COMPONENTS = [
  'Button',
  'TextField',
  'Panel',
  'StatusBadge',
  'StatusGlyph',
  'ConfidenceMeter',
  'Dialog',
  'DescriptionList',
  'Skeleton',
  'Timestamp',
  'Breadcrumbs',
  'OneTimeSecretDialog',
  'Banner',
  'InlineMessage',
  'ToastRegion',
  'PendingOnObject',
  'LoadingState',
  'EmptyState',
  'UnknownState',
  'StaleState',
  'PartialState',
  'DegradedState',
  'DeniedState',
  'ErrorState',
  'DataBoundary',
  'ConfirmationDialog',
  'RouteErrorBoundary',
  'UntrustedText',
  'UntrustedBlock',
] as const;

/**
 * A step-up that always succeeds, so the level 3 confirmation body is
 * reachable here. The real one reauthenticates; this page has no network and
 * no session, and a refusing step-up would leave the typed-confirmation field
 * unrendered and therefore unexercised by the hostile-content test.
 */
const ALWAYS_STEPPED_UP = {
  request: () => Promise.resolve({ satisfied: true as const }),
  consume: () => true,
};

/** Fixed at import so the exercise renders identically on every re-render. */
const OBSERVED_AT = '2026-09-01T12:00:00.000Z';

export function ExerciseEveryComponent({ text }: { text: string }) {
  const toasts = useToasts();
  const cancel = useRef<HTMLButtonElement>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [secretOpen, setSecretOpen] = useState(false);
  const observedAt = OBSERVED_AT;

  return (
    // `RouteErrorBoundary` resets on navigation, so it needs a router.
    <MemoryRouter>
      <RouteErrorBoundary>
        <Panel heading={text} headingLevel={2} eyebrow={text} aside={<StatusBadge encoding={configEncoding('needs_zones')} />}>
          <StatusBadge encoding={healthEncoding('Unknown')} />
          <StatusBadge encoding={severityEncoding('critical')} />
          <StatusBadge encoding={deviceEncoding('revoked')} dimension="Inventory" />
          <ConfidenceMeter value="Medium" />
          <Skeleton lines={2} />
          <DescriptionList label="Device inventory facts" entries={[{ term: 'Display name', value: text }]} />
          <PendingOnObject startedAt={observedAt} reason={text} />
          <TextField name="display_name" label={text} description={text} error={text} />
          <TextField name="locked" label="Locked field" disabledReason={text} />
          <Button variant="primary">Act</Button>
          <Button variant="secondary" pending>Act while pending</Button>
          <Button variant="destructive" disabledReason={text}>Act destructively</Button>
          <Button variant="quiet" onClick={() => toasts.show(text)}>Show a confirmation</Button>
          <Button variant="secondary" onClick={() => setDialogOpen(true)}>Open the dialog</Button>
          <Button variant="destructive" onClick={() => setConfirmOpen(true)}>Revoke the device</Button>
          <Button variant="secondary" onClick={() => setSecretOpen(true)}>Show the one-time secret</Button>
        </Panel>

        {/*
          The untrusted pair takes the same string every other component here
          takes, so the hostile-content test drives them through the whole
          corpus alongside everything else.
        */}
        <Timestamp value={observedAt} />
        <Timestamp value={observedAt} precision="second" mode="absoluteWithRelative" uncertainClock />
        <Timestamp value={null} />

        {/*
          A trail whose last entry is backend text, so the hostile-content
          test drives an attacker-shaped name through a navigation landmark.
        */}
        <Breadcrumbs trail={[
          { label: text, to: '/environments' },
          { label: <UntrustedText value={untrusted(text)} /> },
        ]} />

        <UntrustedText value={untrusted(text)} />
        <UntrustedBlock value={untrusted(text)} label="Captured transcript" />

        <Banner tone="informational">{text}</Banner>
        <Banner tone="blocking">{text}</Banner>
        <Banner tone="restricted">{text}</Banner>
        <InlineMessage tone="error">{text}</InlineMessage>
        <InlineMessage tone="success">{text}</InlineMessage>

        <LoadingState activity={text} skeletonLines={2} />
        <EmptyState collection={text} action={<Button variant="secondary">Create the first</Button>} />
        <UnknownState subject={text} observationSource={text} />
        <StaleState observedAt={observedAt} reason={text}><p>{text}</p></StaleState>
        <PartialState unavailable={[text]} onRetry={() => undefined}><p>{text}</p></PartialState>
        <DegradedState dependency={text} stillWorks={text} doesNotWork={text} onRetry={() => undefined} />
        <DeniedState reauthenticate />
        <ErrorState retryable onRetry={() => undefined} />

        <DataBoundary
          query={{ isPending: false, data: [text], error: null }}
          subject={{ name: text, observationSource: text, dependency: text, stillWorks: text, doesNotWork: text, staleReason: text }}
        >
          {(rows) => <ul>{rows.map((row) => <li key={row}>{row}</li>)}</ul>}
        </DataBoundary>

        <Dialog
          open={dialogOpen}
          onClose={() => setDialogOpen(false)}
          title={text}
          description={text}
          initialFocus={cancel}
          actions={<Button variant="quiet" buttonRef={cancel} onClick={() => setDialogOpen(false)}>Close the dialog</Button>}
        >
          <p>{text}</p>
        </Dialog>

        <ConfirmationDialog
          action="device.revoke"
          objectName={text}
          open={confirmOpen}
          stepUp={ALWAYS_STEPPED_UP}
          onCancel={() => setConfirmOpen(false)}
          onConfirm={() => setConfirmOpen(false)}
        />

        {/*
          The token is driven by the same string as everything else, so the
          hostile-content test proves one-time material renders as text too.
        */}
        <OneTimeSecretDialog
          secret={secretOpen ? { token: text, expires_at: observedAt } : null}
          onDismiss={() => setSecretOpen(false)}
          titleKey="environment.secret.title"
          descriptionKey="environment.secret.description"
          labelKey="environment.secret.label"
        />

        <ToastRegion toasts={toasts.toasts} onDismiss={toasts.dismiss} />
      </RouteErrorBoundary>
    </MemoryRouter>
  );
}
