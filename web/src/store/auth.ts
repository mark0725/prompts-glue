import { create } from "zustand";
import { api } from "@/api";
import { apiError, getToken, setToken } from "@/api/client";
import type { User } from "@/types";

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string;
  requestId?: string;
  initialized: boolean;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => void;
  refresh: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: false,
  error: "",
  initialized: false,
  async login(username, password) {
    set({ loading: true, error: "" });
    try {
      const result = await api.login(username, password);
      setToken(result.access_token);
      set({ user: result.user, loading: false, initialized: true });
      return true;
    } catch (err) {
      const error = apiError(err);
      set({ loading: false, error: error.message, requestId: error.requestId });
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
