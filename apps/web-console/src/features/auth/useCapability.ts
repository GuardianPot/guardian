import {
  resolveCapability,
  type Capability,
  type CapabilityDecision,
} from '@shared/auth/capability';
import { useAuth } from './AuthContext';

/**
 * Presentation-only capability seam. See `@shared/auth/capability` for why
 * this is never a security authority.
 */
export function useCapability(capability: Capability): CapabilityDecision {
  const auth = useAuth();
  return resolveCapability(capability, Boolean(auth.session && auth.csrf));
}
