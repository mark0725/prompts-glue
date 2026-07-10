import { useFeedback } from '@/store/feedback'
import { Modal, Button } from '@/components/ui'
import { cn } from '@/lib/cn'

const colors = { success: 'bg-emerald-600', error: 'bg-red-600', info: 'bg-slate-800' }

export function Feedback() {
  const { toasts, remove, confirm, resolveConfirm } = useFeedback()
  return (
    <>
      <div className="fixed right-4 top-4 z-[60] space-y-2">
        {toasts.map((t) => (
          <div key={t.id} onClick={() => remove(t.id)} className={cn('cursor-pointer rounded-md px-4 py-2 text-sm text-white shadow', colors[t.type])}>
            {t.message}
          </div>
        ))}
      </div>
      <Modal open={confirm.open} onClose={() => resolveConfirm(false)} title={confirm.title || '确认操作'}>
        <p className="mb-4 text-sm text-muted-foreground">{confirm.message}</p>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => resolveConfirm(false)}>取消</Button>
          <Button onClick={() => resolveConfirm(true)}>确定</Button>
        </div>
      </Modal>
    </>
  )
}
