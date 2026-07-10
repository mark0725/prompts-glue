import { create } from "zustand";
import { api } from "@/api";
import { apiError, getToken, setToken } from "@/api/client";
import type { User } from "@/types";

interface AuthState {
  user: User | null;
  initialized: boolean;
  loading: boolean;
  error: string;
  requestId?: string;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => void;
  refresh: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  initialized: false,
  loading: false,
  error: "",
  async login(username, password) {
    set({ error: "", loading: true });
    try {
      const result = await api.login(username, password);
      setToken(result.access_token);
      set({ user: result.user, initialized: true, loading: false });
      return true;
    } catch (err) {
      const error = apiError(err);
      set({ error: error.message, requestId: error.requestId, loading: false });
      return false;
    }
  },
  logout() {
    setToken("");
    set({ user: null });
  },
  async refresh() {
    if (!getToken()) {
      set({ initialized: true });
      return;
    }
    try {
      const user = await api.me();
      set({ user, initialized: true });
    } catch {
      setToken("");
      set({ user: null, initialized: true });
    }
  },
}));
