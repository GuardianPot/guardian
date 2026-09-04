// Boundary fixture: a relative import that escapes the feature must fail lint.
// Never imported by the application; `test/check-boundaries.mjs` lints it.
import { api } from '../../../shared/api/client';

export const fixture = api;
