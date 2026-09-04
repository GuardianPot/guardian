import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { HealthCondition, HealthView } from '@shared/api/types';
import { HealthPanel } from './HealthPanel';

const conditionTypes = [
  'edge_connected',
  'device_certificate_ready',
  'config_converged',
  'local_database_healthy',
  'spool_healthy',
  'clock_quality',
  'container_runtime_reachable',
  'privileged_helper_reachable',
];

function healthView(): HealthView {
  const sourceDeviceID = '018f1f7e-6d31-7cc5-8db8-17547f78e6c2';
  const conditions: HealthCondition[] = conditionTypes.map((type, index) => ({
    type,
    status: index === 0 ? 'False' : 'Unknown',
    reason: index === 0 ? 'channel_disconnected' : 'not_observed',
    message: index === 0 ? '<img src=x onerror=alert(1)> remains plain text' : '',
    source_device_id: sourceDeviceID,
    last_transition_time: '2026-08-29T12:00:00Z',
  }));
  return {
    aggregate: { status: 'False', blocking_type: 'edge_connected', reason: 'channel_disconnected', blocking_device_id: sourceDeviceID },
    conditions,
    received_at: '2026-08-29T12:00:00Z',
  };
}

describe('HealthPanel', () => {
  it('renders all eight backend conditions and keeps false distinct from unknown', () => {
    const { container } = render(<HealthPanel health={healthView()} />);
    expect(screen.getAllByRole('listitem')).toHaveLength(8);
    expect(screen.getAllByText('Action required').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Unknown/).length).toBeGreaterThan(0);
    expect(screen.getByText(/Blocking: Edge connection/)).toHaveTextContent('source 018f1f7e');
    expect(screen.getByText('<img src=x onerror=alert(1)> remains plain text')).toBeVisible();
    expect(container.querySelector('img')).toBeNull();
  });
});
