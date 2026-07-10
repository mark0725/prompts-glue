import { create } from 'zustand'

export type ToastType = 'success' | 'error' | 'info'
export interface Toast {
  id: number
  message: string
  type: ToastType
}

interface FeedbackState {
  toasts: Toast[]
  push: (message: string, type?: ToastType) => void
  remove: (id: number) => void
  confirm: { open: boolean; title: string; message: string; resolve?: (v: boolean) => void }
  confirmDialog: (message: string, title?: string) => Promise<boolean>
  resolveConfirm: (v: boolean) => void
}

let idc = 0

export const useFeedback = create<FeedbackState>((set, get) => ({
  toasts: [],
  push: (message, type = 'info') => {
    const id = ++idc
    set((s) => ({ toasts: [...s.toasts, { id, message, type }] }))
    setTimeout(() => get().remove(id), 3000)
  },
  remove: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
  confirm: { open: false, title: '', message: '' },
  confirmDialog: (message, title = '确认操作') =>
    new Promise<boolean>((resolve) => set({ confirm: { open: true, title, message, resolve } })),
  resolveConfirm: (v) => {
    const r = get().confirm.resolve
    set({ confirm: { open: false, title: '', message: '' } })
    r?.(v)
  },
}))

export const toast = (m: string, t: ToastType = 'info') => useFeedback.getState().push(m, t)
export const confirmDialog = (m: string, title?: string) => useFeedback.getState().confirmDialog(m, title)
