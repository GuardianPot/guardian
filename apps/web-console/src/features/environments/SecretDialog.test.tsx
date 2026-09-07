import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import type { EnrollmentSecret } from '@shared/api/types';
import { untrusted } from '@shared/api/untrusted';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { SecretDialog } from './SecretDialog';

const secret: EnrollmentSecret = {
  token_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c3',
  device_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c2',
  environment_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c1',
  device_name: untrusted('test-edge'),
  token: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  expires_at: '2026-08-29T12:15:00Z',
};

function Harness() {
  const [value, setValue] = useState<EnrollmentSecret | null>(secret);
  return <SecretDialog secret={value} onDismiss={() => setValue(null)} />;
}

describe('SecretDialog', () => {
  it('removes the one-time secret from the DOM on explicit dismissal without browser storage', async () => {
    localStorage.clear(); sessionStorage.clear();
    render(<Harness />);
    expect(screen.getByTestId('enrollment-secret')).toHaveTextContent(secret.token);
    await userEvent.click(screen.getByRole('button', { name: 'I have stored it securely' }));
    expect(screen.queryByText(secret.token)).not.toBeInTheDocument();
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });

  it('reports no serious or critical axe violation', async () => {
    const { baseElement } = render(<Harness />);
    await screen.findByRole('dialog');
    await expectNoAxeViolations(baseElement);
  });

  it('announces nothing that contains the secret', async () => {
    // Section 8.2. A live region is read aloud and is exactly the surface a
    // one-time secret must never reach; the value lives in the dialog body,
    // which is not a live region and is not part of the dialog's name or
    // description.
    const { baseElement } = render(<Harness />);
    const dialog = await screen.findByRole('dialog');

    const live = [...baseElement.querySelectorAll('[aria-live], [role="status"], [role="alert"], [role="log"]')];
    for (const region of live) {
      expect(region.textContent ?? '', 'a live region must not carry the secret').not.toContain(secret.token);
    }
    expect(dialog.getAttribute('aria-label') ?? '').not.toContain(secret.token);
    const describedBy = dialog.getAttribute('aria-describedby');
    const description = describedBy ? document.getElementById(describedBy)?.textContent ?? '' : '';
    expect(description).not.toContain(secret.token);
    const labelledBy = dialog.getAttribute('aria-labelledby');
    const label = labelledBy ? document.getElementById(labelledBy)?.textContent ?? '' : '';
    expect(label).not.toContain(secret.token);
  });
});
