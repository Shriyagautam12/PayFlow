"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { auth } from "./api";
import type { Merchant } from "./types";

interface AuthState {
  token: string | null;
  merchant: Merchant | null;
  login: (token: string, merchant: Merchant) => void;
  logout: () => void;
  isLoading: boolean;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [merchant, setMerchant] = useState<Merchant | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // On mount, try to restore session via refresh token cookie
  useEffect(() => {
    auth
      .refresh()
      .then((res) => {
        setToken(res.access_token);
        setMerchant(res.merchant);
      })
      .catch(() => {
        // No valid session — stay logged out
      })
      .finally(() => setIsLoading(false));
  }, []);

  const login = useCallback((newToken: string, newMerchant: Merchant) => {
    setToken(newToken);
    setMerchant(newMerchant);
  }, []);

  const logout = useCallback(() => {
    if (token) auth.logout(token).catch(() => {});
    setToken(null);
    setMerchant(null);
  }, [token]);

  return (
    <AuthContext.Provider value={{ token, merchant, login, logout, isLoading }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
