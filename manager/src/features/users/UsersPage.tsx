import { useCallback, useEffect, useState } from "react";
import { Fingerprint, KeyRound, Plus, Power, RefreshCw, Search, Trash2 } from "lucide-react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { confirmDialog, toast } from "@/store/feedback";
import { Badge, Button, Card, Input, Modal, Spinner } from "@/components/ui";
import { Select } from "@/components/Select";
import type { AccessToken, User } from "@/types";

const PAGE_SIZE = 20;

// UserManager 按域（web/manager）管理用户：增删改查、启停、重置密码、令牌管理。
export function UserManager({ realm, title }: { realm: "web" | "manager"; title: string }) {
  const label = useDictStore((s) => s.label);
  const dictItems = useDictStore((s) => s.items);
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    const fetcher = realm === "manager" ? api.managerUsers : api.users;
    fetcher(q || undefined, status || undefined, page, PAGE_SIZE)
      .then((res) => {
        setUsers(res.items ?? []);
        setTotal(res.total ?? 0);
      })
      .catch((e) => toast(e.message, "error"))
      .finally(() => setLoading(false));
  }, [realm, q, status, page]);

  useEffect(() => {
    setPage(1);
  }, [q, status]);
  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-foreground">{title}</h1>
        <div className="flex items-center gap-2">
          {realm === "manager" && (
            <Button variant="outline" size="sm" onClick={() => api.syncLDAP().then((r) => toast(`同步完成：manager ${r.synced_manager} / web ${r.synced_web}${r.skipped ? `，跳过 ${r.skipped} 条` : ""}${r.errors?.length ? `，失败 ${r.errors.length} 条` : ""}`, "success")).catch((e) => toast(e.message, "error"))}>
              <RefreshCw size={14} className="mr-1" /> LDAP 同步
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={load}><RefreshCw size={14} /></Button>
          <CreateUserModal realm={realm} onCreated={load} />
        </div>
      </div>

      <Card className="p-2">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="用户名 / 姓名 / 邮箱" className="h-8 w-56 pl-8 text-xs" />
          </div>
          <Select className="w-28" value={status} onChange={setStatus} options={[{ value: "", label: "全部状态" }, ...dictItems("account_status").map((i) => ({ value: i.value, label: i.label }))]} />
          <div className="ml-auto text-xs text-muted-foreground">共 {total} 条</div>
        </div>
      </Card>

      <UserTable realm={realm} users={users} loading={loading} label={label} onChanged={load} />

      <div className="flex items-center justify-between">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>上一页</Button>
        <span className="text-xs text-muted-foreground">第 {page} 页</span>
        <Button variant="outline" size="sm" disabled={users.length < PAGE_SIZE} onClick={() => setPage((p) => p + 1)}>下一页</Button>
      </div>
    </div>
  );
}

// WebUsersPage 主导航的 Web 用户管理。
export function WebUsersPage() {
  return <UserManager realm="web" title="Web 用户管理" />;
}

// ManagerUsersPage 系统管理下的管理端用户管理。
export function ManagerUsersPage() {
  return <UserManager realm="manager" title="管理端用户管理" />;
}

function UserTable({ realm, users, loading, label, onChanged }: {
  realm: "web" | "manager";
  users: User[];
  loading: boolean;
  label: (group: string, value: string) => string;
  onChanged: () => void;
}) {
  return (
    <Card>
      {loading ? <div className="p-10 text-center"><Spinner /></div> : (
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
            <tr>
              <th className="px-3 py-2">用户名</th>
              <th className="px-3 py-2">姓名</th>
              <th className="px-3 py-2">邮箱</th>
              <th className="px-3 py-2">{realm === "manager" ? "角色" : "来源"}</th>
              <th className="px-3 py-2">状态</th>
              <th className="px-3 py-2">最近登录</th>
              <th className="px-3 py-2 text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id} className="border-b border-border/60 hover:bg-muted/40">
                <td className="px-3 py-2 font-mono text-xs">{u.username}</td>
                <td className="px-3 py-2">{u.display_name || "-"}</td>
                <td className="px-3 py-2 text-xs text-muted-foreground">{u.email || "-"}</td>
                <td className="px-3 py-2">
                  {realm === "manager"
                    ? <Badge color="blue">{label("manager_role", u.role || "ADMIN")}</Badge>
                    : <Badge color="slate">{label("account_source", u.source || "LOCAL")}</Badge>}
                </td>
                <td className="px-3 py-2"><Badge color={u.status === "ACTIVE" ? "green" : "yellow"}>{label("account_status", u.status)}</Badge></td>
                <td className="px-3 py-2 text-xs text-muted-foreground">{(u.last_login_at || "").slice(0, 16).replace("T", " ") || "-"}</td>
                <td className="px-3 py-2 text-right">
                  <RowActions user={u} realm={realm} onChanged={onChanged} />
                </td>
              </tr>
            ))}
            {users.length === 0 && <tr><td colSpan={7} className="px-3 py-8 text-center text-muted-foreground">暂无用户</td></tr>}
          </tbody>
        </table>
      )}
    </Card>
  );
}

function RowActions({ user, realm, onChanged }: { user: User; realm: "web" | "manager"; onChanged: () => void }) {
  const scope = realm === "manager" ? "manager-users" : "users";
  const update = realm === "manager" ? api.updateManagerUser : api.updateWebUser;
  const del = realm === "manager" ? api.deleteManagerUser : api.deleteWebUser;
  const reset = realm === "manager" ? api.resetManagerPassword : api.resetWebPassword;
  const [resetOpen, setResetOpen] = useState(false);
  const [pwd, setPwd] = useState("");
  const [tokensOpen, setTokensOpen] = useState(false);

  const toggle = () => {
    const next = user.status === "ACTIVE" ? "DISABLED" : "ACTIVE";
    update(user.id, { status: next })
      .then(() => { toast(`已${next === "ACTIVE" ? "启用" : "禁用"} ${user.username}`, "success"); onChanged(); })
      .catch((e) => toast(e.message, "error"));
  };
  const remove = async () => {
    const ok = await confirmDialog(`确定删除用户 ${user.username}？`, "删除用户");
    if (!ok) return;
    del(user.id)
      .then(() => { toast("已删除", "success"); onChanged(); })
      .catch((e) => toast(e.message, "error"));
  };
  const doReset = () => {
    reset(user.id, pwd)
      .then(() => { toast("密码已重置", "success"); setResetOpen(false); setPwd(""); })
      .catch((e) => toast(e.message, "error"));
  };

  return (
    <div className="flex items-center justify-end gap-1">
      <Button variant="ghost" size="sm" onClick={toggle}><Power size={14} /> {user.status === "ACTIVE" ? "禁用" : "启用"}</Button>
      <Button variant="ghost" size="sm" onClick={() => setResetOpen(true)} disabled={user.source === "LDAP"}><KeyRound size={14} /> 重置密码</Button>
      <Button variant="ghost" size="sm" onClick={() => setTokensOpen(true)}><Fingerprint size={14} /> 令牌</Button>
      <Button variant="ghost" size="sm" className="text-red-600" onClick={remove}><Trash2 size={14} /> 删除</Button>
      <Modal open={resetOpen} onClose={() => setResetOpen(false)} title={`重置 ${user.username} 的密码`}>
        <div className="space-y-3">
          <Input value={pwd} onChange={(e) => setPwd(e.target.value)} placeholder="新密码" type="password" />
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setResetOpen(false)}>取消</Button>
            <Button size="sm" onClick={doReset} disabled={!pwd}>确定</Button>
          </div>
        </div>
      </Modal>
      {tokensOpen && <TokenManager scope={scope} userId={user.id} username={user.username} onClose={() => setTokensOpen(false)} />}
    </div>
  );
}

// TokenManager 管理端查看并吊销某用户的访问令牌（含登录令牌）。
function TokenManager({ scope, userId, username, onClose }: { scope: string; userId: string; username: string; onClose: () => void }) {
  const [items, setItems] = useState<AccessToken[]>([]);
  const [loading, setLoading] = useState(true);

  const load = () => {
    setLoading(true);
    api.userTokens(scope as "users" | "manager-users", userId).then(setItems).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  };
  useEffect(load, []);

  const revoke = async (t: AccessToken) => {
    const ok = await confirmDialog(`确定吊销 ${username} 的令牌「${t.name || "-"}」？`, "吊销令牌");
    if (!ok) return;
    api.revokeUserToken(scope as "users" | "manager-users", userId, t.id).then(() => { toast("已吊销", "success"); load(); }).catch((e) => toast(e.message, "error"));
  };

  return (
    <Modal open onClose={onClose} title={`${username} 的访问令牌`}>
      <div className="space-y-3">
        {loading ? <div className="p-6 text-center"><Spinner /></div> : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
              <tr><th className="px-3 py-2">名称</th><th className="px-3 py-2">创建</th><th className="px-3 py-2">最近使用</th><th className="px-3 py-2">过期</th><th className="px-3 py-2 text-right">操作</th></tr>
            </thead>
            <tbody>
              {items.map((t) => (
                <tr key={t.id} className="border-b border-border/60">
                  <td className="px-3 py-2 font-mono text-xs">{t.name || "-"}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{(t.created_at || "").slice(0, 16).replace("T", " ") || "-"}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{(t.last_used_at || "").slice(0, 16).replace("T", " ") || "—"}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{t.expires_at ? (t.expires_at).slice(0, 16).replace("T", " ") : "永不过期"}</td>
                  <td className="px-3 py-2 text-right"><Button size="sm" variant="ghost" className="text-red-600" onClick={() => revoke(t)}>吊销</Button></td>
                </tr>
              ))}
              {items.length === 0 && <tr><td colSpan={5} className="px-3 py-6 text-center text-muted-foreground">暂无令牌</td></tr>}
            </tbody>
          </table>
        )}
      </div>
    </Modal>
  );
}

function CreateUserModal({ realm, onCreated }: { realm: "web" | "manager"; onCreated: () => void }) {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ username: "", password: "", email: "", display_name: "", role: "ADMIN" });
  const dictItems = useDictStore((s) => s.items);
  const reset = () => setForm({ username: "", password: "", email: "", display_name: "", role: "ADMIN" });

  const submit = () => {
    if (!form.username || !form.password) { toast("用户名与密码必填", "error"); return; }
    const body = realm === "manager"
      ? api.createManagerUser({ username: form.username, password: form.password, email: form.email || undefined, display_name: form.display_name || undefined, role: form.role })
      : api.createWebUser({ username: form.username, password: form.password, email: form.email || undefined, display_name: form.display_name || undefined });
    body
      .then(() => { toast("已创建", "success"); setOpen(false); reset(); onCreated(); })
      .catch((e) => toast(e.message, "error"));
  };

  return (
    <>
      <Button size="sm" onClick={() => setOpen(true)}><Plus size={14} className="mr-1" />新建{realm === "manager" ? "管理端" : "Web"}用户</Button>
      <Modal open={open} onClose={() => setOpen(false)} title={`新建${realm === "manager" ? "管理端" : "Web"}用户`}>
        <div className="space-y-3">
          <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} placeholder="用户名" />
          <Input value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} placeholder="密码" type="password" />
          <Input value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} placeholder="姓名（可选）" />
          <Input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} placeholder="邮箱（可选）" />
          {realm === "manager" && (
            <Select value={form.role} onChange={(role) => setForm({ ...form, role })} options={dictItems("manager_role").map((i) => ({ value: i.value, label: i.label }))} />
          )}
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setOpen(false)}>取消</Button>
            <Button size="sm" onClick={submit}>创建</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
