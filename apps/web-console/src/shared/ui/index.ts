/**
 * The shared component layer (WCX-04).
 *
 * Every screen composes from here. Three things are deliberately absent:
 *
 * - no component takes a `className`. `WCX-04` section 9.5 permits one
 *   documented layout slot; this layer offers none, because a screen can
 *   position a component with its own container element and never needs to
 *   reach inside one. A status indicator therefore cannot be restyled by the
 *   screen that renders it.
 * - no component writes to `localStorage`, `sessionStorage`, or IndexedDB.
 *   `storage.test.tsx` exercises every component and asserts all three stay
 *   empty and untouched.
 * - no `dangerouslySetInnerHTML`. Every value from the backend renders as
 *   text, and `hostileContent.test.tsx` asserts it creates no markup.
 */
export { Button, type ButtonVariant } from './controls/Button';
export { ConfidenceMeter } from './controls/ConfidenceMeter';
export { DescriptionList, type DescriptionEntry } from './controls/DescriptionList';
export { Dialog } from './controls/Dialog';
export { Panel } from './controls/Panel';
export { Skeleton } from './controls/Skeleton';
export { StatusBadge, StatusGlyph } from './controls/StatusBadge';
export { TextField } from './controls/TextField';

export { UntrustedText } from './untrusted/UntrustedText';
export { UntrustedBlock } from './untrusted/UntrustedBlock';
export { BLOCK_LIMIT, TEXT_LIMIT, transformUntrusted, type Segment, type Transformed } from './untrusted/transform';

export { Timestamp, formatAge, parseInstant, type TimestampMode, type TimestampPrecision } from './time/Timestamp';

export { OneTimeSecretDialog, type OneTimeSecret } from './secret/OneTimeSecretDialog';

export { Breadcrumbs, type Crumb } from './nav/Breadcrumbs';

export { Banner, type BannerTone } from './feedback/Banner';
export { InlineMessage, type InlineTone } from './feedback/InlineMessage';
export { PendingOnObject } from './feedback/PendingOnObject';
export { ToastRegion, useToasts, TOAST_MINIMUM_MS, type ToastMessage } from './feedback/Toast';

export {
  DegradedState,
  DeniedState,
  EmptyState,
  ErrorState,
  LoadingState,
  PartialState,
  StaleState,
  UnknownState,
} from './state/states';
export { DataBoundary, describeQuery, type DataSubject, type QueryLike } from './state/DataBoundary';
export {
  DATA_STATES,
  resolveDataState,
  type DataOutcome,
  type DataState,
  type DataStateInput,
} from './state/resolveDataState';
export { DEFAULT_FRESHNESS_CLASS, isBeyondFreshness } from './state/freshness';

export { RootErrorBoundary, RouteErrorBoundary } from './boundary/ErrorBoundary';
export {
  clearLastRenderError,
  readLastRenderError,
  recordRenderError,
  type RecordedRenderError,
} from './boundary/lastRenderError';

export { ConfirmationDialog } from './confirm/ConfirmationDialog';
export {
  ACTION_CONFIRMATION,
  confirmationFor,
  stepUpUnavailable,
  type ActionConfirmation,
  type ConfirmableAction,
  type ConfirmationLevel,
  type StepUpOutcome,
  type StepUpReauthentication,
} from './confirm/levels';
