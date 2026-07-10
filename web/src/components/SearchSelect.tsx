import { useEffect, useRef, useState } from 'react'
import { ChevronDown, Search, X } from 'lucide-react'
import { cn } from '@/lib/cn'

export interface SearchSelectOption {
  value: string
  label: string
}

interface SearchSelectProps {
  value: string
  onChange: (value: string) => void
  options: SearchSelectOption[]
  loading?: boolean
  placeholder?: string
  disabled?: boolean
  allowClear?: boolean
  className?: string
}

export function SearchSelect({ value, onChange, options, loading, placeholder = '请选择', disabled, allowClear = true, className }: SearchSelectProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const filtered = options.filter((opt) => opt.label.toLowerCase().includes(search.toLowerCase()))
  const selected = options.find((opt) => opt.value === value)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
        setSearch('')
      }
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [])

  useEffect(() => {
    if (open && inputRef.current) inputRef.current.focus()
  }, [open])

  return (
    <div className={cn('relative', className)} ref={containerRef}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setOpen(true)}
        className="flex h-9 w-full items-center justify-between rounded-md border border-input bg-card px-3 py-1 text-sm shadow-sm transition-colors hover:border-ring/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
      >
        <span className={cn('truncate', selected ? 'text-foreground' : 'text-muted-foreground')}>{selected?.label || placeholder}</span>
        <span className="flex items-center gap-1">
          {value && !disabled && allowClear && (
            <span className="inline-flex h-6 w-6 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground" onClick={(e) => { e.stopPropagation(); onChange('') }}>
              <X className="h-4 w-4" />
            </span>
          )}
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        </span>
      </button>
      {open && (
        <div className="absolute left-0 top-full z-[60] mt-1 w-full rounded-md border bg-popover shadow-xl">
          <div className="flex items-center border-b px-2">
            <Search className="h-4 w-4 text-muted-foreground" />
            <input ref={inputRef} type="text" className="h-8 flex-1 bg-transparent px-2 text-sm outline-none" placeholder="搜索..." value={search} onChange={(e) => setSearch(e.target.value)} />
          </div>
          <div className="max-h-60 overflow-y-auto">
            {loading ? (
              <div className="px-3 py-2 text-sm text-muted-foreground">加载中...</div>
            ) : filtered.length === 0 ? (
              <div className="px-3 py-2 text-sm text-muted-foreground">无匹配选项</div>
            ) : (
              filtered.map((opt) => (
                <div key={opt.value} className={cn('cursor-pointer px-3 py-1.5 text-sm transition-colors', opt.value === value ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/80')} onClick={() => { onChange(opt.value); setOpen(false); setSearch('') }}>
                  {opt.label}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
