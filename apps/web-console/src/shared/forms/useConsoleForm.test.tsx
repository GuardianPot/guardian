import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import * as v from 'valibot';
import { consoleError, ConsoleRequestError } from '@shared/api/error';
import { TextField } from '@shared/ui';
import { FormMessage } from './FormMessage';
import { useConsoleForm } from './useConsoleForm';

/**
 * The guarantees the form stack owes every screen (WCX-11 section 9.1).
 *
 * These are asserted on the stack rather than on a screen, because a screen can
 * only demonstrate that it happens to behave; the stack is what makes it true
 * everywhere.
 */
const schema = v.object({ name: v.pipe(v.string(), v.minLength(1, 'This field is required.')) });

function Harness({ onSubmit }: { onSubmit: (values: { name: string }) => Promise<void> }) {
  const form = useConsoleForm<{ name: string }>({
    schema,
    defaultValues: { name: '' },
    onSubmit,
  });
  return (
    <MemoryRouter>
      <form onSubmit={(event) => { void form.submit(event); }}>
        <FormMessage error={form.formError} unattached={form.unattached} id={form.formErrorId} />
        <TextField
          name="name"
          label="Name"
          registration={form.form.register('name')}
          {...(form.form.formState.errors.name?.message === undefined
            ? {}
            : { error: String(form.form.formState.errors.name.message) })}
        />
        <button type="submit">Save</button>
        <output>{form.submitting ? 'submitting' : 'idle'}</output>
      </form>
    </MemoryRouter>
  );
}

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe('the console form stack', () => {
  /**
   * Section 10.1.5. A submit control is disabled while a write is in flight,
   * but a disabled control is a rendering decision and this is the guarantee
   * underneath it: the stack refuses a second submission even when one is
   * dispatched before React has re-rendered.
   */
  it('refuses a second submission while one is in flight', async () => {
    let release: () => void = () => {};
    const inFlight = new Promise<void>((resolve) => { release = resolve; });
    const onSubmit = vi.fn().mockReturnValue(inFlight);
    render(<Harness onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText('Name'), 'Lab');
    const save = screen.getByRole('button', { name: 'Save' });
    await user.click(save);
    await waitFor(() => { expect(screen.getByText('submitting')).toBeInTheDocument(); });
    // A second click while the first write is still open.
    await user.click(save);

    expect(onSubmit).toHaveBeenCalledTimes(1);
    release();
    await waitFor(() => { expect(screen.getByText('idle')).toBeInTheDocument(); });
  });

  /**
   * Section 9.2 and change proposal 0004's failure behaviour: a field the
   * backend names that this form does not render is surfaced at form level
   * rather than dropped. A rejection reason that disappears leaves an operator
   * with a failure and no cause.
   */
  it('surfaces a field error it cannot attach rather than dropping it', async () => {
    const onSubmit = vi.fn().mockRejectedValue(
      new ConsoleRequestError(
        consoleError('validation', {
          httpStatus: 400,
          fieldErrors: [{ field: 'persona', messageKey: 'errors.field.unsupported' }],
        }),
      ),
    );
    render(<Harness onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText('Name'), 'Lab');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText(/Guardian rejected a value this screen cannot show/)).toBeInTheDocument();
    // The unrecognised field path is never rendered: it is backend data.
    expect(document.body.textContent).not.toContain('persona');
  });

  /** The same rejection, on a field this form does have, lands on the control. */
  it('attaches a field error to the control the backend named', async () => {
    const onSubmit = vi.fn().mockRejectedValue(
      new ConsoleRequestError(
        consoleError('validation', {
          httpStatus: 400,
          fieldErrors: [{ field: 'name', messageKey: 'errors.field.conflicting' }],
        }),
      ),
    );
    render(<Harness onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText('Name'), 'Lab');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    const field = await screen.findByLabelText('Name');
    await waitFor(() => { expect(field).toHaveAttribute('aria-invalid', 'true'); });
    expect(screen.getByText(/Another record in this environment already uses this/)).toBeInTheDocument();
    // Fully attributed, so there is no form-level message shouting the same
    // thing a second time.
    expect(screen.queryByText(/Guardian rejected a value this screen cannot show/)).not.toBeInTheDocument();
  });

  /** Section 8.1: a client-side rejection never reaches the write. */
  it('does not submit a value its own validator rejects', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<Harness onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('This field is required.')).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  /**
   * Section 8.6 and 10.1.4. The unsaved-changes guard warns; it never saves.
   * Typing into a dirty form must leave every storage area untouched.
   */
  it('persists nothing while a form is dirty', async () => {
    render(<Harness onSubmit={vi.fn().mockResolvedValue(undefined)} />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText('Name'), 'A draft nobody asked to keep');

    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
    expect(document.cookie).toBe('');
  });
});
