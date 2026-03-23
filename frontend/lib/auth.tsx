'use client';

import { createContext, useContext, useEffect, useState, useCallback } from 'react';
import type { User } from '@/lib/types';

const TOKEN_KEY = 'webhookdb_token';
const USER_KEY  = 'webhookdb_user';

interface AuthState {
  user:  User | null;
  token: string | null;
  loading: boolean;
  login:    (token: string, user: User) => void;
  logout:   () => void;
}

const AuthContext = createContext<AuthState>({
  user: null, token: null, loading: true,
  login: () => {}, logout: () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user,    setUser]    = useState<User | null>(null);
  const [token,   setToken]   = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const t = localStorage.getItem(TOKEN_KEY);
    const u = localStorage.getItem(USER_KEY);
    if (t && u) {
      try {
        setToken(t);
        setUser(JSON.parse(u));
      } catch { /* ignore */ }
    }
    setLoading(false);
  }, []);

  const login = useCallback((t: string, u: User) => {
    localStorage.setItem(TOKEN_KEY, t);
    localStorage.setItem(USER_KEY, JSON.stringify(u));
    setToken(t);
    setUser(u);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    setToken(null);
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, token, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}

/** Returns the stored token synchronously (for axios interceptor). */
export function getStoredToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(TOKEN_KEY);
}
