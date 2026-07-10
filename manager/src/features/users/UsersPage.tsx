import { useEffect, useMemo, useState } from "react";
import { RefreshCw, Search } from "lucide-react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Button, Card, Input, Spinner } from "@/components/ui";
import { Select } from "@/components/Select";
import type { User } from "@/types";

export function UsersPage() {
  const label = useDictStore((s) => s.label);
  const dictItems = useDictStore((s) => s.items);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");

  const load = () => {
    setLoading(true);
    api.users().then(setUsers).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  };
  useEffect(load, []);

  const filtered = useMemo(
    () => users.filter((u) =>
      (u.username + (u.display_name || "") + (u.email || "")).toLowerCase().includes(q.toLowerCase()) &&
      (!status || u.status === status),
    ),
    [users, q, status],
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-foreground">用户管理</h1>
        <Button variant="outline" size="sm" onClick={load}><RefreshCw size={14} /></Button>
      </div>

      <Card className="p-2">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="用户名 / 姓名 / 邮箱" className="h-8 w-56 pl-8 text-xs" />
          </div>
          <Select className="w-28" value={status} onChange={setStatus} options={[{ value: "", label: "全部状态" }, ...dictItems("account_status").map((i) => ({ value: i.value, label: i.label }))]} />
        </div>
      </Card>

      <Card>
        {loading ? <div className="p-10 text-center"><Spinner /></div> : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2">用户名</th>
                <th className="px-3 py-2">姓名</th>
                <th className="px-3 py-2">邮箱</th>
                <th className="px-3 py-2">角色</th>
                <th className="px-3 py-2">状态</th>
                <th className="px-3 py-2">最近登录</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((u) => (
                <tr key={u.id} className="border-b border-border/60 hover:bg-muted/40">
                  <td className="px-3 py-2 font-mono text-xs">{u.username}</td>
                  <td className="px-3 py-2">{u.display_name || "-"}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{u.email || "-"}</td>
                  <td className="px-3 py-2">{u.role}</td>
                  <td className="px-3 py-2"><Badge color={u.status === "ACTIVE" ? "green" : "yellow"}>{label("account_status", u.status)}</Badge></td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{(u.last_login_at || "").slice(0, 16).replace("T", " ") || "-"}</td>
                </tr>
              ))}
              {filtered.length === 0 && <tr><td colSpan={6} className="px-3 py-8 text-center text-muted-foreground">暂无用户</td></tr>}
            </tbody>
          </table>
        )}
      </Card>
      <div className="text-xs text-muted-foreground">共 {filtered.length} 条</div>
    </div>
  );
}
