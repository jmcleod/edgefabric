import { createContext, useContext, useEffect, useState, useCallback, useRef, ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiLogin, apiVerifyTotpWithToken, apiGet, apiLogout } from '@/lib/api';
import { transformUser } from '@/lib/transforms';
import type { User } from '@/types';
import type { ApiUser } from '@/types/api';

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<{ totpRequired: boolean }>;
  verifyTotp: (code: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Fetch current user profile using the HttpOnly session cookie.
  const fetchMe = useCallback(async () => {
    try {
      const apiUser = await apiGet<ApiUser>('/api/v1/auth/me');
      setUser(transformUser(apiUser));
    } catch {
      // No valid session cookie — user is not authenticated.
      setUser(null);
    }
  }, []);

  // On mount: check for existing session (cookie-based)
  useEffect(() => {
    fetchMe().finally(() => setIsLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Holds the MFA-pending token in memory (not localStorage, not cookie) until TOTP is verified.
  const pendingTokenRef = useRef<string | null>(null);

  const login = useCallback(async (email: string, password: string) => {
    const resp = await apiLogin(email, password);

    if (resp.totp_required) {
      // Store MFA-pending token only in memory — the backend already sent it
      // in the JSON body (NOT as a cookie). We'll use it for the TOTP verify call.
      pendingTokenRef.current = resp.token;
    } else {
      // Full session — the server set an HttpOnly cookie. Fetch user profile.
      const apiUser = await apiGet<ApiUser>('/api/v1/auth/me');
      setUser(transformUser(apiUser));
    }

    return { totpRequired: resp.totp_required };
  }, []);

  const verifyTotp = useCallback(async (code: string) => {
    const pending = pendingTokenRef.current;
    if (!pending) {
      throw new Error('No pending MFA token — call login() first');
    }

    // Use the pending token explicitly for the TOTP verify call.
    // On success the server sets the HttpOnly session cookie.
    await apiVerifyTotpWithToken(code, pending);
    pendingTokenRef.current = null;

    // Fetch user profile after TOTP verification.
    const apiUser = await apiGet<ApiUser>('/api/v1/auth/me');
    setUser(transformUser(apiUser));
  }, []);

  const logout = useCallback(async () => {
    await apiLogout(); // Clears the session cookie server-side.
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, isLoading, login, verifyTotp, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

/** Wrapper component that redirects to /login if not authenticated. */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, isLoading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isLoading && !user) {
      navigate('/login', { replace: true });
    }
  }, [user, isLoading, navigate]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    );
  }

  if (!user) {
    return null;
  }

  return <>{children}</>;
}
