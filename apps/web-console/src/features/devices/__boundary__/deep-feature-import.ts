// Boundary fixture: importing a file inside another feature must fail lint.
// Never imported by the application; `test/check-boundaries.mjs` lints it.
import { HealthPanel } from '@features/health/HealthPanel';

export const fixture = HealthPanel;
