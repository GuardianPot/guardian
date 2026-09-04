import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api } from '@shared/api/client';
import type { Session } from '@shared/api/types';

type LoginInput = { username: string; password: string; totp_code?: string; recovery_code?: string };

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
  const sessionQuery = useQuery({
    queryKey: ['session'],
    queryFn: () => api.session(),
    staleTime: 30_000,
    retry: false,
  });

  const expire = useCallback(() => {
    setCsrf(null);
    queryClient.setQueryData(['session'], null);
    queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'session' });
  }, [queryClient]);

  useEffect(() => {
    window.addEventListener('guardian:unauthorized', expire);
    return () => window.removeEventListener('guardian:unauthorized', expire);
  }, [expire]);

  const value = useMemo<AuthValue>(() => ({
    session: sessionQuery.data,
    csrf,
    loading: sessionQuery.isPending,
    async login(input) {
      const credentials = await api.login(input);
      setCsrf(credentials.csrf_token);
      queryClient.setQueryData(['session'], credentials.session);
    },
    async logout() {
      if (!csrf) throw new Error('Re-authentication is required before logout.');
      await api.logout(csrf);
      expire();
    },
  }), [csrf, expire, queryClient, sessionQuery.data, sessionQuery.isPending]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}
