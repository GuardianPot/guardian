import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
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
    setCsrf(null);
    queryClient.setQueryData(authKeys.session(), null);
    queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'auth' });
  }, [queryClient]);

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
