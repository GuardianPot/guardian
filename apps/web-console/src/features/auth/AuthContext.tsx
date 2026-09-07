import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { permitPolling } from '@shared/api/freshness';
import { authKeys, login as loginRequest, logout as logoutRequest, sessionQuery, type LoginInput } from './api';
import type { Session } from '@shared/api/types';

type AuthValue = {
  session: Session | null | undefined;
  csrf: string | null;
  loading: boolean;
  login(input: LoginInput): Promise<void>;
  logout(): Promise<void>;
};

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [csrf, setCsrf] = useState<string | null>(null);
  const session = useQuery(sessionQuery());

  const expire = useCallback(() => {
    // Close the polling gate first. Removing the queries below stops their
    // intervals, but an interval that fires between these two lines would
    // still reach the Control Plane on behalf of a session that has ended
    // (WCX-07 section 8.1).
    permitPolling(false);
    setCsrf(null);
    queryClient.setQueryData(authKeys.session(), null);
    queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'auth' });
  }, [queryClient]);

  // Polling follows the session, not the component tree: it opens when a
  // session exists and closes the moment one does not.
  useEffect(() => { permitPolling(Boolean(session.data)); }, [session.data]);

  useEffect(() => {
    window.addEventListener('guardian:unauthorized', expire);
    return () => window.removeEventListener('guardian:unauthorized', expire);
  }, [expire]);

  const value = useMemo<AuthValue>(() => ({
    session: session.data,
    csrf,
    loading: session.isPending,
    async login(input) {
      const credentials = await loginRequest(input);
      setCsrf(credentials.csrf_token);
      queryClient.setQueryData(authKeys.session(), credentials.session);
    },
    async logout() {
      if (!csrf) throw new Error('Re-authentication is required before logout.');
      await logoutRequest(csrf);
      expire();
    },
  }), [csrf, expire, queryClient, session.data, session.isPending]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}
