// Boundary fixture: a relative import that escapes the feature must fail lint.
// Never imported by the application; `test/check-boundaries.mjs` lints it.
import { request } from '../../../shared/api/transport';

export const fixture = request;
