// 自定义下拉框（非原生 select），用于表单/筛选中的字典选项。
import { useEffect, useRef, useState } from 'react'
import { ChevronDown, Check } from 'lucide-react'
import { cn } from '@/lib/cn'

export interface SelectOption {
  value: string
  label: string
}

interface SelectProps {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  className?: string
  dropUp?: boolean
}

export function Select({ value, onChange, options, placeholder = '请选择', disabled, className, dropUp }: SelectProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const selected = options.find((o) => o.value === value)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [])

  return (
    <div className={cn('relative', className)} ref={ref}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setOpen((o) => !o)}
        className="flex h-9 w-full items-center justify-between rounded-md border border-input bg-card px-3 py-1 text-sm shadow-sm transition-colors hover:border-ring/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
      >
        <span className={cn('truncate', selected ? 'text-foreground' : 'text-muted-foreground')}>{selected?.label || placeholder}</span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
      </button>
      {open && (
        <div className={cn('absolute left-0 z-[60] max-h-60 w-full overflow-y-auto rounded-md border bg-popover py-1 shadow-xl', dropUp ? 'bottom-full mb-1' : 'top-full mt-1')}>
          {options.length === 0 ? (
            <div className="px-3 py-2 text-sm text-muted-foreground">无选项</div>
          ) : (
            options.map((o) => (
              <div
                key={o.value}
                className={cn('flex cursor-pointer items-center justify-between px-3 py-1.5 text-sm transition-colors', o.value === value ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/80')}
                onClick={() => { onChange(o.value); setOpen(false) }}
              >
                <span className="truncate">{o.label}</span>
                {o.value === value && <Check className="h-3.5 w-3.5 shrink-0" />}
              </div>
            ))
          )}
        </div>
      )}
    </div>
  )
}
