// Boundary fixture: shared must never depend on a feature.
// Never imported by the application; `test/check-boundaries.mjs` lints it.
import { useAuth } from '@features/auth';

export const fixture = useAuth;
