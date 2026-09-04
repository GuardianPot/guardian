import { toConsoleError, type ConsoleError } from './error';

/**
 * Server-state conventions (WC-D04).
 *
 * Key shape is `[feature, resource, ...scopeIds, params?]` and every key comes
 * from a per-feature factory, so no component builds one by hand. Reads retry
 * only when the classified error is retryable; mutations never retry.
 */
export const DEFAULT_PAGE_SIZE = 200;
export const MAX_READ_RETRIES = 2;

/** Retries a read at most twice, and only for a retryable classification. */
export function retryRead(failureCount: number, error: unknown): boolean {
  if (failureCount >= MAX_READ_RETRIES) return false;
  return toConsoleError(error).retryable;
}

/** Exponential backoff for the retries `retryRead` permits. */
export function retryDelay(failureCount: number): number {
  return Math.min(1_000 * 2 ** failureCount, 8_000);
}

/** Narrows an unknown query error to the console taxonomy. */
export function asConsoleError(error: unknown): ConsoleError {
  return toConsoleError(error);
}
