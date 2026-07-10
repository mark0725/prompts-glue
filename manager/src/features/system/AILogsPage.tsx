import { useEffect, useState } from "react";
import { api } from "@/api";
import { toast } from "@/store/feedback";
import { Badge, Card, Spinner } from "@/components/ui";
import { Pager } from "@/components/Pager";
import type { GenerationRecord } from "@/types";

export function AILogsPage() {
  const [list, setList] = useState<GenerationRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  const load = (p = page, ps = pageSize) => {
    setLoading(true);
    api.generationRecords(p, ps)
      .then((d) => { setList(d.items ?? []); setTotal(d.total ?? 0); setPage(d.page ?? p); })
      .catch((e) => toast(e.message, "error"))
      .finally(() => setLoading(false));
  };
  useEffect(() => { load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-bold text-foreground">AI日志</h1>
        <p className="text-sm text-muted-foreground">实时生成与复合提示词计算记录，含 token 用量、缓存命中与耗时。</p>
      </div>
      <Card>
        {loading ? <div className="p-10 text-center"><Spinner /></div> : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-3 py-2">时间</th>
                  <th className="px-3 py-2">表达式</th>
                  <th className="px-3 py-2">模型</th>
                  <th className="px-3 py-2 text-right">Token 入/出/合</th>
                  <th className="px-3 py-2">缓存</th>
                  <th className="px-3 py-2 text-right">耗时</th>
                  <th className="px-3 py-2">状态</th>
                </tr>
              </thead>
              <tbody>
                {list.map((r) => (
                  <tr key={r.id} className="border-b border-border/60 hover:bg-muted/40">
                    <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">{(r.created_at || "").slice(0, 19).replace("T", " ")}</td>
                    <td className="max-w-[280px] truncate px-3 py-2 font-mono text-xs" title={r.expression}>{r.expression || "-"}</td>
                    <td className="whitespace-nowrap px-3 py-2 text-xs">{r.model_name || "-"}</td>
                    <td className="whitespace-nowrap px-3 py-2 text-right text-xs">{r.token_input}/{r.token_output}/{r.token_total}</td>
                    <td className="px-3 py-2"><Badge color={r.cache_hit ? "green" : "slate"}>{r.cache_hit ? "命中" : "未命中"}</Badge></td>
                    <td className="whitespace-nowrap px-3 py-2 text-right text-xs">{r.duration_ms != null ? `${r.duration_ms}ms` : "-"}</td>
                    <td className="px-3 py-2">
                      {r.error_code ? <Badge color="red" >失败</Badge> : <Badge color="green">成功</Badge>}
                      {r.error_code ? <span className="ml-1 text-xs text-muted-foreground">{r.error_code}</span> : null}
                    </td>
                  </tr>
                ))}
                {list.length === 0 && <tr><td colSpan={7} className="px-3 py-8 text-center text-muted-foreground">暂无 AI 调用记录</td></tr>}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <Pager page={page} total={total} pageSize={pageSize} onPageChange={(p) => load(p)} onPageSizeChange={(s) => { setPageSize(s); load(1, s); }} />
    </div>
  );
}
