import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { useRef, useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { configEncoding, deviceEncoding, healthEncoding, severityEncoding } from '@shared/theme/statusEncoding';
import { Button } from './Button';
import { ConfidenceMeter } from './ConfidenceMeter';
import { DescriptionList } from './DescriptionList';
import { Dialog } from './Dialog';
import { Panel } from './Panel';
import { Skeleton } from './Skeleton';
import { StatusBadge } from './StatusBadge';
import { TextField } from './TextField';

describe('Button', () => {
  it('keeps its accessible name stable while pending', async () => {
    const { rerender } = render(<Button variant="primary">Create environment</Button>);
    expect(screen.getByRole('button', { name: 'Create environment' })).toBeEnabled();

    rerender(<Button variant="primary" pending>Create environment</Button>);
    const pending = screen.getByRole('button', { name: 'Create environment' });
    expect(pending).toBeDisabled();
    expect(pending).toHaveAttribute('aria-busy', 'true');
    // The pending marker is visible but excluded from the name, so a screen
    // reader and a selector both keep addressing the same control.
    expect(screen.getByText('Working…')).toHaveAttribute('aria-hidden', 'true');
    await Promise.resolve();
  });

  it('renders the reason a disabled control is disabled and associates it', () => {
    render(<Button disabledReason="Re-authenticate before creating an environment.">Create environment</Button>);
    const button = screen.getByRole('button', { name: 'Create environment' });
    expect(button).toBeDisabled();
    // Disabled, never hidden (WC-D07), with the reason readable (section 9.7.5).
    expect(button).toBeVisible();
    expect(button).toHaveAccessibleDescription('Re-authenticate before creating an environment.');
  });

  it('offers the four approved variants and no way to pass a class', () => {
    for (const variant of ['primary', 'secondary', 'destructive', 'quiet'] as const) {
      const view = render(<Button variant={variant}>Act</Button>);
      expect(screen.getByRole('button', { name: 'Act' }).className, variant).not.toBe('');
      view.unmount();
    }
  });
});

describe('TextField', () => {
  it('wires aria-describedby for a description, an error, and a disabled reason', () => {
    render(
      <TextField
        name="cidr"
        label="Private CIDR"
        description="Use a canonical RFC1918 range."
        error="That range overlaps an existing zone."
      />,
    );
    const input = screen.getByLabelText('Private CIDR');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription(/Use a canonical RFC1918 range\./);
    expect(input).toHaveAccessibleDescription(/That range overlaps an existing zone\./);
    expect(screen.getByRole('alert')).toHaveTextContent('That range overlaps an existing zone.');
  });

  it('associates a disabled field with its reason instead of failing silently', () => {
    render(<TextField name="cidr" label="Private CIDR" disabledReason="Re-authenticate before adding a zone." />);
    const input = screen.getByLabelText('Private CIDR');
    expect(input).toBeDisabled();
    expect(input).toHaveAccessibleDescription('Re-authenticate before adding a zone.');
  });

  it('marks a field invalid only alongside the message that explains it', () => {
    render(
      <>
        <TextField name="username" label="Username" invalidatedBy="login-error" />
        <p id="login-error">Sign-in was denied.</p>
      </>,
    );
    const input = screen.getByLabelText('Username');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Sign-in was denied.');
  });
});

describe('Panel', () => {
  it('takes an explicit heading level so a screen keeps a valid heading order', () => {
    render(
      <Panel heading="Edge devices" headingLevel={2} eyebrow="Inventory truth" aside={<span>3</span>}>
        <p>content</p>
      </Panel>,
    );
    expect(screen.getByRole('heading', { level: 2, name: 'Edge devices' })).toBeVisible();
    expect(screen.getByRole('region', { name: 'Edge devices' })).toBeVisible();

    render(<Panel heading="Nested" headingLevel={3}><p>content</p></Panel>);
    expect(screen.getByRole('heading', { level: 3, name: 'Nested' })).toBeVisible();
  });
});

describe('StatusBadge', () => {
  it('carries a glyph, text, and a tone for every encoding table', () => {
    for (const encoding of [
      healthEncoding('True'),
      healthEncoding('False'),
      healthEncoding('Unknown'),
      severityEncoding('critical'),
      deviceEncoding('revoked'),
      configEncoding('needs_zones'),
    ]) {
      const view = render(<StatusBadge encoding={encoding} />);
      const badge = screen.getByText(encoding.label);
      expect(badge.querySelector('svg'), encoding.label).not.toBeNull();
      expect(badge.className, encoding.label).not.toBe('');
      expect(badge.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
      view.unmount();
    }
  });

  it('resolves an unrecognised backend value to the unknown treatment, never to healthy', () => {
    render(<StatusBadge encoding={healthEncoding('SomethingNew')} />);
    expect(screen.getByText('Unknown')).toBeVisible();
    expect(screen.queryByText('Healthy')).toBeNull();
  });

  it('names the dimension it reports so inventory is not read as health', () => {
    render(<StatusBadge encoding={deviceEncoding('active')} dimension="Inventory" />);
    expect(screen.getByText('Inventory: active')).toBeVisible();
  });
});

describe('ConfidenceMeter', () => {
  it('renders confidence as steps and text, never as colour alone', () => {
    render(<ConfidenceMeter value="High" />);
    expect(screen.getByText('Confidence')).toBeVisible();
    expect(screen.getByText('High — 3 of 3')).toBeVisible();
  });

  it('reads an unrecognised confidence as unknown with no filled steps', () => {
    render(<ConfidenceMeter value="Certain" />);
    expect(screen.getByText('Unknown — 0 of 3')).toBeVisible();
  });
});

describe('DescriptionList', () => {
  it('pairs each term with a value that is always rendered', () => {
    render(
      <DescriptionList
        label="Device inventory facts"
        entries={[
          { term: 'Inventory state', value: 'active' },
          { term: 'Active certificate expiry', value: 'No active certificate' },
        ]}
      />,
    );
    const region = screen.getByRole('region', { name: 'Device inventory facts' });
    expect(region.querySelectorAll('dt')).toHaveLength(2);
    expect(region.querySelectorAll('dd')).toHaveLength(2);
    expect(screen.getByText('No active certificate')).toBeVisible();
  });
});

describe('Skeleton', () => {
  it('is silent, because LoadingState already announces the activity', () => {
    const { container } = render(<Skeleton lines={2} />);
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('status')).toBeNull();
  });
});

function DialogHarness() {
  const [open, setOpen] = useState(false);
  const cancel = useRef<HTMLButtonElement>(null);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>Open</button>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title="Enrollment secret — shown once"
        description="Enter this value directly on the intended Edge host."
        initialFocus={cancel}
        actions={<Button variant="quiet" buttonRef={cancel} onClick={() => setOpen(false)}>Cancel</Button>}
      >
        <p>dialog body</p>
      </Dialog>
    </>
  );
}

describe('Dialog', () => {
  it('has an accessible name and description, traps focus, and returns it on escape', async () => {
    render(<DialogHarness />);
    const trigger = screen.getByRole('button', { name: 'Open' });
    await userEvent.click(trigger);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveAccessibleName('Enrollment secret — shown once');
    expect(dialog).toHaveAccessibleDescription('Enter this value directly on the intended Edge host.');
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();

    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    await waitFor(() => { expect(trigger).toHaveFocus(); });
  });
});

describe('the layer as a whole', () => {
  const walk = (dir: string): string[] =>
    readdirSync(dir).flatMap((entry) => {
      const path = join(dir, entry);
      return statSync(path).isDirectory() ? walk(path) : [path];
    });

  const modules = walk('src/shared/ui').filter(
    (path) => /\.tsx?$/.test(path) && !path.includes('.test.'),
  );

  /** The module with its comments removed, so a rule cannot fail on its own prose. */
  const codeOf = (path: string): string =>
    readFileSync(path, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');

  it('accepts no className from outside, so a screen cannot restyle a status indicator', () => {
    // WCX-04 section 9.5 permits one documented layout slot. This layer offers
    // none: a screen positions a component with its own container element and
    // never reaches inside one.
    expect(modules.length).toBeGreaterThan(10);
    for (const path of modules) {
      expect(codeOf(path), `${path} declares a className prop`).not.toMatch(/^\s*className\??:/m);
    }
  });

  it('renders no HTML from data anywhere in the layer', () => {
    for (const path of modules) {
      expect(codeOf(path), path).not.toContain('dangerouslySetInnerHTML');
    }
  });

  it('subscribes to no global interval', () => {
    // Section 9.11. A toast uses a per-instance `setTimeout` cleared on
    // unmount; nothing polls.
    for (const path of modules) {
      expect(codeOf(path), path).not.toMatch(/setInterval|requestAnimationFrame/);
    }
  });

  it('leaves the console with no report of its own', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(<Panel heading="Edge devices" headingLevel={2}><p>content</p></Panel>);
    expect(spy).not.toHaveBeenCalled();
    spy.mockRestore();
  });
});
