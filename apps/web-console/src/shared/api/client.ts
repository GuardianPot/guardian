import type {
  Device,
  EnrollmentSecret,
  Environment,
  HealthView,
  Session,
  Zone,
} from './types';

export class ApiError extends Error {
  constructor(public readonly status: number) {
    super(status === 401 ? 'Your session or authorization proof is no longer valid.' : 'Guardian could not complete the request.');
    this.name = 'ApiError';
  }
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE';
  body?: unknown;
  csrf?: string;
  etag?: number;
  allowUnauthorized?: boolean;
};

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers({ Accept: 'application/json' });
  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json');
  }
  if (options.csrf) {
    headers.set('X-CSRF-Token', options.csrf);
    headers.set('Origin', window.location.origin);
  }
  if (options.etag !== undefined) {
    headers.set('If-Match', `"${options.etag}"`);
  }
  const response = await fetch(path, {
    method: options.method ?? 'GET',
    // Spread rather than assign `undefined`: under exactOptionalPropertyTypes
    // an explicit `undefined` body is not the same as an omitted one.
    ...(options.body === undefined ? {} : { body: JSON.stringify(options.body) }),
    headers,
    credentials: 'include',
    cache: 'no-store',
  });
  if (!response.ok) {
    if (response.status === 401 && !options.allowUnauthorized) {
      window.dispatchEvent(new Event('guardian:unauthorized'));
    }
    throw new ApiError(response.status);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export const api = {
  async session(): Promise<Session | null> {
    try {
      const response = await request<{ session: Session }>('/v1/auth/session', { allowUnauthorized: true });
      return response.session;
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) return null;
      throw error;
    }
  },

  async login(input: { username: string; password: string; totp_code?: string; recovery_code?: string }) {
    return request<{ csrf_token: string; session: Session }>('/v1/auth/login', {
      method: 'POST',
      body: input,
      allowUnauthorized: true,
    });
  },

  logout(csrf: string) {
    return request<void>('/v1/auth/logout', { method: 'POST', csrf });
  },

  async environments() {
    return (await request<{ environments: Environment[] }>('/v1/environments?limit=200')).environments;
  },

  async environment(environmentID: string) {
    return (await request<{ environment: Environment }>(`/v1/environments/${environmentID}`)).environment;
  },

  async createEnvironment(displayName: string, csrf: string) {
    return (await request<{ environment: Environment }>('/v1/environments', {
      method: 'POST',
      body: { display_name: displayName },
      csrf,
    })).environment;
  },

  async updateEnvironment(environment: Environment, displayName: string, csrf: string) {
    return (await request<{ environment: Environment }>(`/v1/environments/${environment.environment_id}`, {
      method: 'PATCH',
      body: { display_name: displayName },
      csrf,
      etag: environment.revision,
    })).environment;
  },

  async zones(environmentID: string) {
    return (await request<{ zones: Zone[] }>(`/v1/environments/${environmentID}/zones?limit=200`)).zones;
  },

  async createZone(environmentID: string, input: { display_name: string; cidr: string }, csrf: string) {
    return (await request<{ zone: Zone }>(`/v1/environments/${environmentID}/zones`, {
      method: 'POST',
      body: input,
      csrf,
    })).zone;
  },

  async devices(environmentID: string) {
    return (await request<{ devices: Device[] }>(`/v1/environments/${environmentID}/devices`)).devices;
  },

  async device(environmentID: string, deviceID: string) {
    return (await request<{ device: Device }>(`/v1/environments/${environmentID}/devices/${deviceID}`)).device;
  },

  createEnrollmentSecret(environmentID: string, deviceName: string, csrf: string) {
    return request<EnrollmentSecret>(`/v1/environments/${environmentID}/enrollment-tokens`, {
      method: 'POST',
      body: { device_name: deviceName },
      csrf,
    });
  },

  environmentHealth(environmentID: string) {
    return request<HealthView>(`/v1/environments/${environmentID}/health`);
  },

  deviceHealth(deviceID: string) {
    return request<HealthView>(`/v1/devices/${deviceID}/health`);
  },
};
