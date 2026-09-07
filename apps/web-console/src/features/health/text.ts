/**
 * Health projection text (WCX-04 section 9.10).
 *
 * The condition labels and the two status words are asserted by the browser
 * suite and are unchanged. The subject strings below are new: they are what
 * the shared `unknown`, `degraded`, and `stale` states say about a health read,
 * and they exist so those states name *health* rather than "this data".
 */
export const HEALTH_TEXT = {
  eyebrow: 'Backend health projection',
  heading: 'Eight-condition health',
  conditionsLabel: 'Device health conditions',
  blocking: 'Blocking: ',
  receivedAt: (time: string) => `Control Plane received this projection ${time}.`,
  /** Named in the `unknown` state, so absence is about health specifically. */
  environmentSubject: 'the health of this environment',
  deviceSubject: 'the health of this device',
  observationSource: 'an enrolled Edge reports its eight health conditions',
  dependency: 'The health projection',
  stillWorks: 'inventory and configuration, which are separate reads',
  doesNotWork: 'every health condition for this scope',
  staleReason: 'the last health refresh did not return',
  conditions: {
    edge_connected: 'Edge connection',
    device_certificate_ready: 'Device certificate',
    config_converged: 'Configuration convergence',
    local_database_healthy: 'Local database',
    spool_healthy: 'Event spool',
    clock_quality: 'Clock quality',
    container_runtime_reachable: 'Container runtime',
    privileged_helper_reachable: 'Privileged helper',
  } as Readonly<Record<string, string>>,
} as const;
