import { useMemo, useState } from "react";
import { useDictStore } from "@/store/dict";
import { Badge, Card } from "@/components/ui";
import { Select } from "@/components/Select";

const sampleRisks = [
  { id: "demo-1", resource: "my-prompt@1.0.0", risk_type: "SENSITIVE_CONTENT", severity: "HIGH", status: "OPEN" },
  { id: "demo-2", resource: "@scope/other", risk_type: "POLICY_VIOLATION", severity: "MEDIUM", status: "REVIEWING" },
];

const sevColor: Record<string, string> = { LOW: "blue", MEDIUM: "yellow", HIGH: "red", CRITICAL: "red" };

export function RisksPage() {
  const label = useDictStore((s) => s.label);
  const dictItems = useDictStore((s) => s.items);
  const [severity, setSeverity] = useState("");
  const [status, setStatus] = useState("");

  const filtered = useMemo(
    () => sampleRisks.filter((r) => (!severity || r.severity === severity) && (!status || r.status === status)),
    [severity, status],
  );

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-foreground">风险标记</h1>

      <Card className="p-2">
        <div className="flex flex-wrap items-center gap-2">
          <Select className="w-32" value={severity} onChange={setSeverity} options={[{ value: "", label: "全部级别" }, ...dictItems("risk_severity").map((i) => ({ value: i.value, label: i.label }))]} />
          <Select className="w-32" value={status} onChange={setStatus} options={[{ value: "", label: "全部状态" }, ...dictItems("risk_status").map((i) => ({ value: i.value, label: i.label }))]} />
        </div>
      </Card>

      <Card>
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
            <tr>
              <th className="px-3 py-2">资源</th>
              <th className="px-3 py-2">风险类型</th>
              <th className="px-3 py-2">严重级别</th>
              <th className="px-3 py-2">状态</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((r) => (
              <tr key={r.id} className="border-b border-border/60 hover:bg-muted/40">
                <td className="px-3 py-2 font-mono text-xs text-primary">{r.resource}</td>
                <td className="px-3 py-2">{label("risk_type", r.risk_type)}</td>
                <td className="px-3 py-2"><Badge color={sevColor[r.severity] || "slate"}>{label("risk_severity", r.severity)}</Badge></td>
                <td className="px-3 py-2"><Badge color={r.status === "RESOLVED" ? "green" : "blue"}>{label("risk_status", r.status)}</Badge></td>
              </tr>
            ))}
            {filtered.length === 0 && <tr><td colSpan={4} className="px-3 py-8 text-center text-muted-foreground">暂无风险标记</td></tr>}
          </tbody>
        </table>
      </Card>
      <p className="text-xs text-muted-foreground">风险标记流程由后端管理 API 提供完整能力，当前为只读视图。</p>
    </div>
  );
}
