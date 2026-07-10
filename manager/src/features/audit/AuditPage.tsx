import { useEffect, useState } from "react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Card, Spinner } from "@/components/ui";
import { Pager } from "@/components/Pager";
import type { AuditLog } from "@/types";

export function AuditPage() {
  const label = useDictStore((s) => s.label);
  const [rows, setRows] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  const load = (p = page, ps = pageSize) => {
    setLoading(true);
    api.auditLogs(p, ps)
      .then((d) => { setRows(d.items ?? []); setTotal(d.total ?? 0); setPage(d.page ?? p); })
      .catch((e) => toast(e.message, "error"))
      .finally(() => setLoading(false));
  };
  useEffect(() => { load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-foreground">审计日志</h1>
      <Card>
        {loading ? <div className="p-10 text-center"><Spinner /></div> : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2">时间</th>
                <th className="px-3 py-2">操作</th>
                <th className="px-3 py-2">资源</th>
                <th className="px-3 py-2">结果</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, i) => (
                <tr key={i} className="border-b border-border/60 hover:bg-muted/40">
                  <td className="px-3 py-2 text-xs text-muted-foreground">{(row.created_at || "").slice(0, 19).replace("T", " ") || "-"}</td>
                  <td className="px-3 py-2">{label("audit_action", row.action) || row.action || "-"}</td>
                  <td className="px-3 py-2">{row.resource_type || "-"}</td>
                  <td className="px-3 py-2"><Badge color={row.result === "SUCCESS" ? "green" : row.result === "DENIED" ? "red" : "yellow"}>{label("operation_result", row.result) || row.result || "-"}</Badge></td>
                </tr>
              ))}
              {rows.length === 0 && <tr><td colSpan={4} className="px-3 py-8 text-center text-muted-foreground">暂无审计记录</td></tr>}
            </tbody>
          </table>
        )}
      </Card>
      <Pager page={page} total={total} pageSize={pageSize} onPageChange={(p) => load(p)} onPageSizeChange={(s) => { setPageSize(s); load(1, s); }} />
    </div>
  );
}
