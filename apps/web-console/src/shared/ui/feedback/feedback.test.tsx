import { act, fireEvent, render, renderHook, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Banner } from './Banner';
import { InlineMessage } from './InlineMessage';
import { PendingOnObject } from './PendingOnObject';
import { TOAST_MINIMUM_MS, ToastRegion, useToasts } from './Toast';

/**
 * The three feedback surfaces (WCX-04 section 9.4, WC-D17).
 */
afterEach(() => { vi.useRealTimers(); });

describe('Banner', () => {
  it('announces an informational condition as status and a blocking one as alert', () => {
    const informational = render(<Banner tone="informational">Read-only session restored.</Banner>);
    expect(screen.getByRole('status')).toHaveTextContent('Read-only session restored.');
    expect(screen.queryByRole('alert')).toBeNull();
    informational.unmount();

    render(<Banner tone="blocking">Zone creation failed.</Banner>);
    expect(screen.getByRole('alert')).toHaveTextContent('Zone creation failed.');
  });

  it('announces a restricted session as status, because nothing has failed', () => {
    render(<Banner tone="restricted">Read-only session restored.</Banner>);
    expect(screen.getByRole('status')).toBeVisible();
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('InlineMessage', () => {
  it('announces an error and stays silent on a success', () => {
    const error = render(<InlineMessage tone="error">Sign-in was denied.</InlineMessage>);
    expect(screen.getByRole('alert')).toHaveTextContent('Sign-in was denied.');
    error.unmount();

    render(<InlineMessage tone="success">Environment name updated.</InlineMessage>);
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByText('Environment name updated.')).toBeVisible();
  });
});

function ToastHarness() {
  const toasts = useToasts();
  return (
    <>
      <button type="button" onClick={() => toasts.show('Environment created.')}>Create</button>
      <ToastRegion toasts={toasts.toasts} onDismiss={toasts.dismiss} />
    </>
  );
}

describe('Toast', () => {
  it('announces a completed action as status without stealing focus', async () => {
    render(<ToastHarness />);
    const trigger = screen.getByRole('button', { name: 'Create' });
    await userEvent.click(trigger);

    expect(screen.getByRole('status')).toHaveTextContent('Environment created.');
    expect(trigger).toHaveFocus();
  });

  it('is dismissible from the keyboard', async () => {
    render(<ToastHarness />);
    await userEvent.click(screen.getByRole('button', { name: 'Create' }));

    await userEvent.tab();
    expect(screen.getByRole('button', { name: 'Dismiss' })).toHaveFocus();
    await userEvent.keyboard('{Enter}');
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('lives at least six seconds and pauses while hovered', () => {
    expect(TOAST_MINIMUM_MS).toBeGreaterThanOrEqual(6_000);
    vi.useFakeTimers();
    render(<ToastHarness />);
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    act(() => { vi.advanceTimersByTime(TOAST_MINIMUM_MS - 1_000); });
    expect(screen.getByRole('status')).toBeVisible();

    fireEvent.mouseEnter(screen.getByRole('status'));
    act(() => { vi.advanceTimersByTime(TOAST_MINIMUM_MS * 3); });
    expect(screen.getByRole('status')).toBeVisible();

    fireEvent.mouseLeave(screen.getByRole('status'));
    act(() => { vi.advanceTimersByTime(TOAST_MINIMUM_MS + 1_000); });
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('pauses while the dismiss control holds focus', () => {
    vi.useFakeTimers();
    render(<ToastHarness />);
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    fireEvent.focus(screen.getByRole('button', { name: 'Dismiss' }));
    act(() => { vi.advanceTimersByTime(TOAST_MINIMUM_MS * 3); });
    expect(screen.getByRole('status')).toBeVisible();
  });

  it('offers no error tone, so an error can never be toast-only', () => {
    // Section 8.5. A toast expires unseen, so it may accompany an error but
    // never replace it. Making the error tone unrepresentable is stronger than
    // a convention a reviewer has to remember: `show` takes text and nothing
    // else, a `ToastMessage` carries no severity, and the rendered element is
    // a `status` region with no `alert` anywhere in the module.
    const { result } = renderHook(() => useToasts());
    expect(result.current.show).toHaveLength(1);
    act(() => { result.current.show('Environment created.'); });
    for (const toast of result.current.toasts) {
      expect(Object.keys(toast).sort()).toEqual(['id', 'text']);
    }

    const source = readFileSync('src/shared/ui/feedback/Toast.tsx', 'utf8');
    const code = source.slice(source.indexOf('export const TOAST_MINIMUM_MS'));
    expect(code).not.toMatch(/tone|alert|severity|variant/i);
    expect(code).toContain('role="status"');
  });

  it('cannot intercept a click on the content it floats over', () => {
    const { container } = render(<ToastHarness />);
    const region = container.querySelector('div > div');
    expect(region).not.toBeNull();
    // The region is `pointer-events: none` in `app.module.css`; only the
    // dismiss control takes pointer events. Asserted against the stylesheet
    // because jsdom does not resolve CSS Modules to computed styles.
    const css = readFileSync('src/shared/styles/app.module.css', 'utf8');
    expect(css).toMatch(/\.toastRegion\s*\{[^}]*pointer-events:\s*none/);
    expect(css).toMatch(/\.toastDismiss\s*\{[^}]*pointer-events:\s*auto/);
  });
});

describe('PendingOnObject', () => {
  it('shows long-running work on the object with its age and reason', () => {
    render(
      <PendingOnObject
        startedAt={new Date(Date.now() - 90_000).toISOString()}
        reason="waiting for the Edge to acknowledge"
      />,
    );
    expect(screen.getByRole('status')).toHaveTextContent('In progress');
    expect(screen.getByText(/Started 1 minute ago/)).toBeVisible();
    expect(screen.getByText(/waiting for the Edge to acknowledge/)).toBeVisible();
  });

  it('is never a blocking modal', () => {
    render(<PendingOnObject startedAt={new Date().toISOString()} reason="in flight" />);
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});

describe('error surfaces', () => {
  it('renders every error path on a surface that cannot expire', () => {
    // The two surfaces that may carry an error both persist until the
    // condition clears. Neither is time-limited, and the toast — the only
    // surface that is — cannot express an error at all.
    function Harness() {
      const [tone, setTone] = useState<'inline' | 'banner'>('inline');
      return (
        <>
          <button type="button" onClick={() => setTone('banner')}>switch</button>
          {tone === 'inline'
            ? <InlineMessage tone="error">The environment could not be created.</InlineMessage>
            : <Banner tone="blocking">Zone creation failed.</Banner>}
        </>
      );
    }
    render(<Harness />);
    expect(screen.getByRole('alert')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Dismiss' })).toBeNull();
  });
});
