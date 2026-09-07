import { lazy } from 'react';

export { deviceKeys, deviceQuery, devicesQuery } from './api';

/** Route component as its own chunk. See `@features/auth` for why. */
export const DeviceRoute = lazy(() => import('./DevicePage').then((module) => ({ default: module.DevicePage })));
