import { create } from "zustand";
import { api } from "@/api";
import { apiError } from "@/api/client";
import type { DictItem } from "@/types";

interface DictState {
  dicts: Record<string, DictItem[]>;
  loading: boolean;
  loaded: boolean;
  error: string;
  requestId?: string;
  load: () => Promise<void>;
  label: (group: string, value?: string) => string;
  items: (group: string) => DictItem[];
}

function normalize(value?: string): string {
  return value ?? "";
}

export const useDictStore = create<DictState>((set, get) => ({
  dicts: {},
  loading: false,
  loaded: false,
  error: "",
  async load() {
    if (get().loaded || get().loading) {
      return;
    }
    set({ loading: true, error: "" });
    try {
      const dicts = await api.dicts();
      set({ dicts, loaded: true, loading: false });
    } catch (err) {
      const error = apiError(err);
      set({ loading: false, error: error.message, requestId: error.requestId });
    }
  },
  label(group, value) {
    const items = get().dicts[group] ?? [];
    const found = items.find((item) => item.value === normalize(value));
    return found?.label ?? normalize(value);
  },
  items(group) {
    return get().dicts[group] ?? [];
  },
}));
