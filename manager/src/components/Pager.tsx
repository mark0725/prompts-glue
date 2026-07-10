// 统一分页栏：左侧共N条/第X/Y页/每页条数，右侧翻页。
import { Button } from '@/components/ui'
import { Select } from '@/components/Select'

const DEFAULT_OPTS = [10, 20, 50, 100]

interface PagerProps {
  page: number
  total: number
  pageSize: number
  pageSizeOptions?: number[]
  onPageChange: (p: number) => void
  onPageSizeChange: (s: number) => void
}

export function Pager({ page, total, pageSize, pageSizeOptions, onPageChange, onPageSizeChange }: PagerProps) {
  const pages = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)))
  const opts = pageSizeOptions || DEFAULT_OPTS
  return (
    <div className="flex flex-col gap-2 py-2 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-3">
        <span>共 {total} 条</span>
        <span className="hidden sm:inline">·</span>
        <span>第 {page} / {pages} 页</span>
        <span className="hidden sm:inline">·</span>
        <div className="flex items-center gap-1">
          <span>每页</span>
          <Select
            value={String(pageSize)}
            onChange={(v) => onPageSizeChange(Number(v))}
            className="w-20"
            dropUp
            options={opts.map((o) => ({ value: String(o), label: String(o) }))}
          />
          <span>条</span>
        </div>
      </div>
      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>上一页</Button>
        <Button size="sm" variant="outline" disabled={page >= pages} onClick={() => onPageChange(page + 1)}>下一页</Button>
      </div>
    </div>
  )
}
