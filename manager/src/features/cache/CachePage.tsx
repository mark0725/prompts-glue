import { useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast, confirmDialog } from "@/store/feedback";
import { Badge, Button, Card, Spinner } from "@/components/ui";
import { Pager } from "@/components/Pager";
import type { CacheEntry } from "@/types";

export function CachePage() {
  const label = useDictStore((s) => s.label);
  const [entries, setEntries] = useState<CacheEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  const load = (p = page, ps = pageSize) => {
    setLoading(true);
    api.cacheList(p, ps).then((d) => { setEntries(d.items ?? []); setTotal(d.total ?? 0); setPage(d.page ?? p); }).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  };
  useEffect(() => { load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const clear = async (mode: "all" | "expired") => {
    const msg = mode === "all" ? "将删除所有计算缓存，下次生成需重新计算。" : "将只删除已过期的缓存项。";
    if (!(await confirmDialog(msg, mode === "all" ? "清理全部缓存" : "清理过期缓存"))) return;
    try {
      await api.clearCache(mode === "expired");
      toast("缓存已清理", "success");
      load(1);
    } catch (e: any) { toast(e.message, "error"); }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-foreground">计算缓存</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => clear("expired")}>清理过期</Button>
          <Button variant="danger" size="sm" onClick={() => clear("all")}><Trash2 size={14} /> 清理全部</Button>
        </div>
      </div>

      <Card>
        {loading ? <div className="p-10 text-center"><Spinner /></div> : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2">缓存键</th>
                <th className="px-3 py-2">策略</th>
                <th className="px-3 py-2 text-right">命中</th>
                <th className="px-3 py-2">过期时间</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr key={e.id} className="border-b border-border/60 hover:bg-muted/40">
                  <td className="px-3 py-2 font-mono text-xs">{e.cache_key.slice(0, 16)}…</td>
                  <td className="px-3 py-2"><Badge color="blue">{label("cache_strategy", e.cache_strategy)}</Badge></td>
                  <td className="px-3 py-2 text-right">{e.hit_count}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{(e.expires_at || "").slice(0, 16).replace("T", " ") || "长期"}</td>
                </tr>
              ))}
              {entries.length === 0 && <tr><td colSpan={4} className="px-3 py-8 text-center text-muted-foreground">暂无缓存</td></tr>}
            </tbody>
          </table>
        )}
      </Card>

      <Pager page={page} total={total} pageSize={pageSize} onPageChange={(p) => load(p)} onPageSizeChange={(s) => { setPageSize(s); load(1, s); }} />
    </div>
  );
}
