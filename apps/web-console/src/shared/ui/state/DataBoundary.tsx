import type { ReactNode } from 'react';
import { asConsoleError } from '@shared/api/query';
import {
  DegradedState,
  DeniedState,
  EmptyState,
  ErrorState,
  LoadingState,
  PartialState,
  StaleState,
  UnknownState,
} from './states';
import { resolveDataState, type DataOutcome, type DataStateInput } from './resolveDataState';

/**
 * Maps one read to exactly one data state and renders it (WCX-04 section 9.1).
 *
 * The screen supplies the words — what the collection is, what would produce
 * an observation, which dependency is impaired — and this supplies the
 * classification. That split is the point: `P1-W11` wrote the classification
 * inline in two routes and it had already drifted between them.
 */
export type DataSubject = {
  /** The collection or observation, in the operator's terms. */
  name: string;
  /** For `unknown`: what would produce an observation. */
  observationSource?: string;
  /** For `degraded`: the impaired dependency. */
  dependency?: string;
  /** For `degraded`: what still answers. */
  stillWorks?: string;
  /** For `degraded`: what does not. */
  doesNotWork?: string;
  /** For `stale`: why the refresh is not current. */
  staleReason?: string;
};

/** The shape a TanStack Query result satisfies structurally. */
export type QueryLike<T> = {
  isPending: boolean;
  data: T | undefined;
  error: unknown;
  /** When the cache last accepted this data. TanStack supplies it. */
  dataUpdatedAt?: number;
};

export type DataBoundaryProps<T> = {
  query: QueryLike<T>;
  subject: DataSubject;
  /** Backend confirmed zero items. Defaults to an empty array check. */
  isEmpty?: (data: T) => boolean;
  /** The domain defines absence for this projection. */
  isAbsent?: (data: T) => boolean;
  /** `not-found` on this resource means "no observation exists". */
  observationShaped?: boolean;
  /** Sources that failed while others succeeded. */
  partialFailures?: readonly string[];
  /**
   * When the observation behind this read was made.
   *
   * Supplying it opts the read into the freshness rule: a success older than
   * `FRESHNESS_LIMIT_MS` renders `stale`. That is a per-read policy decision,
   * so it is opt-in rather than derived — `WCX-07` takes the decision away
   * from the call site entirely. When it is absent the stale state still shows
   * an age, taken from when the cache last accepted the data.
   */
  observedAt?: string | null;
  now?: number;
  onRetry?: () => void;
  /** The control that creates the first item, for `empty`. */
  emptyAction?: ReactNode;
  /** Rendered for `ready`, and beside the banner for `stale` and `partial`. */
  children: (data: T) => ReactNode;
};

const defaultIsEmpty = (data: unknown): boolean => Array.isArray(data) && data.length === 0;

/**
 * Fallback wording for a screen that did not name its dependency. Deliberately
 * vague about the cause and specific about the consequence, so it can never
 * read as a working read.
 */
const DEFAULT_DEPENDENCY = 'An upstream dependency';
const DEFAULT_STILL_WORKS = 'the rest of this console';
const DEFAULT_STALE_REASON = 'the last refresh did not finish';

/**
 * The age to show beside stale data.
 *
 * The read's own observation time when it has one, otherwise the moment the
 * query cache last accepted the data. Never a fabricated timestamp: a stale
 * banner claiming "observed 56 years ago" is worse than no age at all.
 */
function observedAtForDisplay<T>(props: DataBoundaryProps<T>): string | undefined {
  if (props.observedAt) return props.observedAt;
  const cached = props.query.dataUpdatedAt;
  return cached ? new Date(cached).toISOString() : undefined;
}

export function describeQuery<T>(props: DataBoundaryProps<T>): DataStateInput {
  const { query } = props;
  const hasData = query.data !== undefined;
  const isEmpty = hasData ? (props.isEmpty ?? defaultIsEmpty)(query.data as T) : false;
  const isAbsent = hasData && props.isAbsent !== undefined ? props.isAbsent(query.data as T) : false;
  return {
    isPending: query.isPending,
    hasData,
    error: query.error === null || query.error === undefined ? null : asConsoleError(query.error),
    isEmpty,
    isAbsent,
    observationShaped: props.observationShaped ?? false,
    partialFailures: props.partialFailures ?? [],
    observedAt: props.observedAt ?? null,
    ...(props.now === undefined ? {} : { now: props.now }),
  };
}

export function DataBoundary<T>(props: DataBoundaryProps<T>) {
  const { query, subject, children, onRetry } = props;
  const outcome: DataOutcome = resolveDataState(describeQuery(props));
  const retry = onRetry ? { onRetry } : {};
  const staleAge = observedAtForDisplay(props);

  switch (outcome) {
    case 'loading':
      return <LoadingState activity={`Loading ${subject.name}`} />;
    case 'empty':
      return <EmptyState collection={subject.name} action={props.emptyAction} />;
    case 'unknown':
      return (
        <UnknownState
          subject={subject.name}
          {...(subject.observationSource === undefined ? {} : { observationSource: subject.observationSource })}
        />
      );
    case 'stale':
      return (
        <StaleState
          {...(staleAge === undefined ? {} : { observedAt: staleAge })}
          reason={subject.staleReason ?? DEFAULT_STALE_REASON}
          {...(props.now === undefined ? {} : { now: props.now })}
        >
          {query.data === undefined ? null : children(query.data)}
        </StaleState>
      );
    case 'partial':
      return (
        <PartialState unavailable={props.partialFailures ?? []} {...retry}>
          {query.data === undefined ? null : children(query.data)}
        </PartialState>
      );
    case 'degraded':
      return (
        <DegradedState
          dependency={subject.dependency ?? DEFAULT_DEPENDENCY}
          stillWorks={subject.stillWorks ?? DEFAULT_STILL_WORKS}
          doesNotWork={subject.doesNotWork ?? subject.name}
          {...retry}
        />
      );
    case 'denied':
      return <DeniedState />;
    case 'error':
      return <ErrorState retryable={asConsoleError(query.error).retryable} {...retry} />;
    case 'ready':
      return <>{query.data === undefined ? null : children(query.data)}</>;
  }
}
