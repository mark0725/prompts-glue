import { create } from "zustand";
import { api } from "@/api";
import { apiError } from "@/api/client";
import type { DictItem } from "@/types";

interface DictState {
  dicts: Record<string, DictItem[]>;
  loaded: boolean;
  loading: boolean;
  error: string;
  requestId?: string;
  load: () => Promise<void>;
  label: (group: string, value?: string) => string;
  items: (group: string) => DictItem[];
}

export const useDictStore = create<DictState>((set, get) => ({
  dicts: {},
  loaded: false,
  loading: false,
  error: "",
  async load() {
    if (get().loaded || get().loading) return;
    set({ loading: true, error: "" });
    try {
      const dicts = await api.dictsAll();
      set({ dicts, loaded: true, loading: false });
    } catch (err) {
      const error = apiError(err);
      set({ loading: false, error: error.message, requestId: error.requestId });
    }
  },
  label(group, value) {
    const valueText = value ?? "";
    return get().dicts[group]?.find((item) => item.value === valueText)?.label ?? valueText;
  },
  items(group) {
    return get().dicts[group] ?? [];
  },
}));
