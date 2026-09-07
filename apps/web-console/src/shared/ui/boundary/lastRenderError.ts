/**
 * The last render error, in memory only (WCX-04 sections 9.2.4 and 9.12).
 *
 * Nothing here is transmitted, logged, or written to any storage area. It is
 * a module-level variable that dies with the tab, and it exists so that
 * `WCX-15` can build the user-triggered diagnostic report on top of an
 * interface that already exists rather than adding a capture path later.
 *
 * The recorded message is never rendered. Section 8.8 forbids a fallback from
 * showing an exception message, a stack, a component stack, or any request or
 * response content, and the fallbacks in `ErrorBoundary` read nothing from
 * here.
 */
export type RecordedRenderError = {
  /** Diagnostics only. Never rendered by any component in this package. */
  message: string;
  /** Milliseconds since the epoch, for the future report's ordering. */
  at: number;
};

let recorded: RecordedRenderError | null = null;

export function recordRenderError(error: unknown, at: number = Date.now()): void {
  recorded = {
    message: error instanceof Error ? error.message : String(error),
    at,
  };
}

export function readLastRenderError(): RecordedRenderError | null {
  return recorded;
}

export function clearLastRenderError(): void {
  recorded = null;
}
