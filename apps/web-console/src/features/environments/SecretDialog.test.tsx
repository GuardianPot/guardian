import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import type { EnrollmentSecret } from '@shared/api/types';
import { SecretDialog } from './SecretDialog';

const secret: EnrollmentSecret = {
  token_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c3',
  device_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c2',
  environment_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c1',
  device_name: 'test-edge',
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
});
