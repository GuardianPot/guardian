export type Session = {
  session_id: string;
  user_id: string;
  username: string;
  role: 'owner';
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  revoked_at?: string;
  current: boolean;
};

export type Environment = {
  environment_id: string;
  organization_id: string;
  display_name: string;
  revision: number;
  zone_count: number;
  status: 'needs_zones' | 'zones_defined';
  created_at: string;
  updated_at: string;
};

export type Zone = {
  zone_id: string;
  environment_id: string;
  display_name: string;
  cidr: string;
  revision: number;
  created_at: string;
  updated_at: string;
};

export type DeviceState = 'pending' | 'active' | 'disabled' | 'revoked';

export type Device = {
  device_id: string;
  environment_id: string;
  display_name: string;
  state: DeviceState;
  created_at: string;
  updated_at: string;
  active_certificate_expires_at?: string;
};

export type EnrollmentSecret = {
  token_id: string;
  device_id: string;
  environment_id: string;
  device_name: string;
  token: string;
  expires_at: string;
};

export type HealthStatus = 'True' | 'False' | 'Unknown';

export type HealthCondition = {
  type: string;
  status: HealthStatus;
  reason: string;
  message: string;
  observed_revision?: number;
  source_device_id?: string;
  last_transition_time: string;
};

export type HealthView = {
  aggregate: {
    status: HealthStatus;
    blocking_type?: string;
    reason?: string;
    blocking_device_id?: string;
  };
  conditions: HealthCondition[];
  received_at: string;
  device_id?: string;
  environment_id?: string;
};
