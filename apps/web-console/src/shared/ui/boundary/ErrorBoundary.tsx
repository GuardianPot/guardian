import { Component, type ReactNode } from 'react';
import { useLocation } from 'react-router-dom';
import styles from '@shared/styles/app.module.css';
import { Button } from '@shared/ui/controls/Button';
import { recordRenderError } from './lastRenderError';
import { ROOT_BOUNDARY_TEXT, ROUTE_BOUNDARY_TEXT } from './text';

/**
 * Render error containment (WCX-04 section 9.2, remediates `P1-W11` GAP-2).
 *
 * Before this package any unexpected exception blanked the whole console —
 * unacceptable in a security product, because a blank page is indistinguishable
 * from "nothing is wrong". Two boundaries now stand between an exception and
 * that outcome:
 *
 * - a root boundary around the router, whose fallback still identifies the
 *   product and offers a reload. It renders inside the document, so the
 *   document language and the theme stylesheet stay mounted;
 * - a route boundary around each routed element, so a failing screen leaves
 *   navigation and sign-out reachable.
 *
 * Neither fallback renders anything derived from the caught error. The error
 * is handed to `recordRenderError`, which keeps it in memory for `WCX-15` and
 * transmits, logs, and stores nothing.
 */
type BoundaryProps = {
  children: ReactNode;
  /** Rendered instead of the children once an error is caught. */
  fallback: (reset: () => void) => ReactNode;
  /** Changing this value clears a caught error, so navigation recovers. */
  resetKey?: string;
};

type BoundaryState = { failed: boolean };

class RenderErrorBoundary extends Component<BoundaryProps, BoundaryState> {
  override state: BoundaryState = { failed: false };

  static getDerivedStateFromError(): BoundaryState {
    return { failed: true };
  }

  // React also offers the `ErrorInfo` component stack here. It is deliberately
  // not a parameter: it names the component tree an operator's screen was
  // rendering, nothing in this package has a use for it, and a parameter that
  // exists is a parameter someone later renders.
  override componentDidCatch(error: Error): void {
    recordRenderError(error);
  }

  override componentDidUpdate(previous: BoundaryProps): void {
    if (this.state.failed && previous.resetKey !== this.props.resetKey) {
      this.setState({ failed: false });
    }
  }

  private readonly reset = () => this.setState({ failed: false });

  override render(): ReactNode {
    return this.state.failed ? this.props.fallback(this.reset) : this.props.children;
  }
}

/** Wraps the router. Its fallback is the last thing standing. */
export function RootErrorBoundary({ children }: { children: ReactNode }) {
  return (
    <RenderErrorBoundary
      fallback={() => (
        <main className={styles.centered}>
          <div className={styles.stateBlock} role="alert">
            <p className={styles.eyebrow}>{ROOT_BOUNDARY_TEXT.product}</p>
            <p className={styles.stateHeading}>{ROOT_BOUNDARY_TEXT.heading}</p>
            <p className={styles.stateDetail}>{ROOT_BOUNDARY_TEXT.body}</p>
            <Button variant="primary" onClick={() => { window.location.reload(); }}>
              {ROOT_BOUNDARY_TEXT.reload}
            </Button>
          </div>
        </main>
      )}
    >
      {children}
    </RenderErrorBoundary>
  );
}

/**
 * Wraps one routed element. Resets on navigation, so moving away from a broken
 * screen recovers without a full reload.
 */
export function RouteErrorBoundary({ children }: { children: ReactNode }) {
  const location = useLocation();
  return (
    <RenderErrorBoundary
      resetKey={`${location.pathname}${location.search}`}
      fallback={(reset) => (
        <div className={`${styles.stateBlock} ${styles.stateBlocking}`} role="alert">
          <p className={styles.stateHeading}>{ROUTE_BOUNDARY_TEXT.heading}</p>
          <p className={styles.stateDetail}>{ROUTE_BOUNDARY_TEXT.body}</p>
          <Button variant="secondary" onClick={reset}>{ROUTE_BOUNDARY_TEXT.retry}</Button>
        </div>
      )}
    >
      {children}
    </RenderErrorBoundary>
  );
}
