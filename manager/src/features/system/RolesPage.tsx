import { useEffect, useMemo, useState } from "react";
import { Pencil, Plus, Shield, Trash2, UserMinus, UserPlus } from "lucide-react";
import { api } from "@/api";
import { confirmDialog, toast } from "@/store/feedback";
import { Button, Card, Input, Modal, Spinner } from "@/components/ui";
import { SearchSelect } from "@/components/SearchSelect";
import { cn } from "@/lib/cn";
import type { Permission, Role, RoleMember, User } from "@/types";

const groupLabel: Record<string, string> = {
  user: "用户", role: "角色", dict: "字典", config: "配置", risk: "风险",
  audit: "审计", cache: "缓存", ldap: "LDAP", generation: "生成记录",
};

export function RolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [tab, setTab] = useState<"members" | "perms">("members");
  const [edit, setEdit] = useState<Role | {} | null>(null);

  const loadRoles = () => {
    setLoading(true);
    api.roles().then((list) => {
      setRoles(list);
      if (list.length && !list.find((r) => r.id === selectedId)) setSelectedId(list[0].id);
    }).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  };
  useEffect(loadRoles, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <div className="p-10 text-center"><Spinner /></div>;

  const selected = roles.find((r) => r.id === selectedId) ?? null;
  const delRole = async (r: Role) => {
    if (r.system) { toast("内置角色不可删除", "error"); return; }
    if (!(await confirmDialog(`确认删除角色「${r.name}」？`, "删除角色"))) return;
    api.deleteRole(r.id).then(() => { toast("已删除", "success"); setSelectedId(null); loadRoles(); }).catch((e) => toast(e.message, "error"));
  };

  return (
    <div className="flex h-full gap-4">
      <Card className="w-64 shrink-0 overflow-y-auto p-2">
        <div className="mb-2 flex items-center justify-between px-1">
          <h2 className="text-sm font-semibold text-foreground">角色</h2>
          <Button size="sm" variant="ghost" onClick={() => setEdit({})}><Plus size={14} /></Button>
        </div>
        <div className="space-y-1">
          {roles.map((r) => (
            <button key={r.id} onClick={() => setSelectedId(r.id)} className={cn("flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm transition-colors", r.id === selectedId ? "bg-primary text-primary-foreground" : "hover:bg-accent")}>
              <span className="flex items-center gap-2"><Shield size={14} /> {r.name}</span>
              <span className={cn("text-xs", r.id === selectedId ? "text-primary-foreground/80" : "text-muted-foreground")}>{r.code}</span>
            </button>
          ))}
          {roles.length === 0 && <div className="px-3 py-6 text-center text-sm text-muted-foreground">暂无角色</div>}
        </div>
      </Card>

      <div className="flex min-w-0 flex-1 flex-col">
        {!selected ? (
          <Card className="flex flex-1 items-center justify-center text-sm text-muted-foreground">请选择左侧角色</Card>
        ) : (
          <>
            <div className="mb-3 flex items-center justify-between">
              <div>
                <h1 className="text-lg font-bold text-foreground">{selected.name}</h1>
                <div className="text-xs text-muted-foreground">{selected.code} · {selected.enabled ? "启用" : "停用"}{selected.system ? " · 内置" : ""}</div>
              </div>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={() => setEdit(selected)}><Pencil size={14} /> 编辑</Button>
                <Button size="sm" variant="outline" className="text-red-600" onClick={() => delRole(selected)}><Trash2 size={14} /> 删除</Button>
              </div>
            </div>
            <div className="mb-3 flex gap-1 border-b">
              {([["members", "成员"], ["perms", "权限"]] as const).map(([k, l]) => (
                <button key={k} onClick={() => setTab(k)} className={cn("border-b-2 px-4 py-2 text-sm font-medium transition-colors", tab === k ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground")}>{l}</button>
              ))}
            </div>
            <Card className="flex min-h-0 flex-1 flex-col overflow-hidden">
              {tab === "members" ? <MembersPanel roleId={selected.id} /> : <PermsPanel roleId={selected.id} />}
            </Card>
          </>
        )}
      </div>

      {edit && <RoleModal role={edit} onClose={() => setEdit(null)} onDone={() => { setEdit(null); loadRoles(); }} />}
    </div>
  );
}

// MembersPanel 角色成员管理。
function MembersPanel({ roleId }: { roleId: string }) {
  const [members, setMembers] = useState<RoleMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [addOpen, setAddOpen] = useState(false);
  const [allUsers, setAllUsers] = useState<User[]>([]);
  const [addUid, setAddUid] = useState("");

  const load = () => {
    setLoading(true);
    api.roleMembers(roleId).then(setMembers).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  };
  useEffect(load, [roleId]);

  const openAdd = () => {
    api.managerUsers(undefined, undefined, 1, 500).then((r) => setAllUsers(r.items ?? [])).catch((e) => toast(e.message, "error"));
    setAddUid("");
    setAddOpen(true);
  };
  const addMember = () => {
    if (!addUid) return;
    const ids = [...members.map((m) => m.id), addUid];
    api.setRoleMembers(roleId, ids).then(() => { toast("已添加", "success"); setAddOpen(false); load(); }).catch((e) => toast(e.message, "error"));
  };
  const removeMember = async (m: RoleMember) => {
    if (!(await confirmDialog(`确认从角色移除「${m.name}」？`, "移除成员"))) return;
    const ids = members.map((x) => x.id).filter((id) => id !== m.id);
    api.setRoleMembers(roleId, ids).then(() => { toast("已移除", "success"); load(); }).catch((e) => toast(e.message, "error"));
  };
  const addOptions = allUsers.filter((u) => !members.some((m) => m.id === u.id)).map((u) => ({ value: u.id, label: `${u.display_name || u.username}（${u.username}）` }));

  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between border-b p-2">
        <span className="text-sm text-muted-foreground">共 {members.length} 名成员</span>
        <Button size="sm" onClick={openAdd}><UserPlus size={14} /> 添加成员</Button>
      </div>
      {loading ? <div className="p-6 text-center"><Spinner /></div> : (
        <div className="divide-y">
          {members.map((m) => (
            <div key={m.id} className="flex items-center justify-between px-3 py-2 text-sm">
              <div><span className="font-medium">{m.name}</span> <span className="text-xs text-muted-foreground">@{m.username}</span></div>
              <Button size="sm" variant="ghost" className="text-red-600" onClick={() => removeMember(m)}><UserMinus size={14} /> 移除</Button>
            </div>
          ))}
          {members.length === 0 && <div className="px-3 py-6 text-center text-sm text-muted-foreground">暂无成员</div>}
        </div>
      )}
      <Modal open={addOpen} onClose={() => setAddOpen(false)} title="添加成员">
        <div className="space-y-3">
          <SearchSelect value={addUid} onChange={setAddUid} options={addOptions} placeholder="搜索用户名/姓名" />
          {addOptions.length === 0 && <div className="text-sm text-muted-foreground">无可添加用户（全部已是成员）</div>}
          <div className="flex justify-end gap-2"><Button variant="outline" size="sm" onClick={() => setAddOpen(false)}>取消</Button><Button size="sm" onClick={addMember} disabled={!addUid}>添加</Button></div>
        </div>
      </Modal>
    </div>
  );
}

// PermsPanel 角色权限管理（按 code 前缀分组勾选）。
function PermsPanel({ roleId }: { roleId: string }) {
  const [allPerms, setAllPerms] = useState<Permission[]>([]);
  const [sel, setSel] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([api.rbacRole(roleId), api.permissions()])
      .then(([detail, perms]) => {
        setAllPerms(perms);
        setSel(new Set(detail.permission_ids ?? []));
      })
      .catch((e) => toast(e.message, "error"))
      .finally(() => setLoading(false));
  }, [roleId]);

  const groups = useMemo(() => {
    const m: Record<string, Permission[]> = {};
    for (const p of allPerms) (m[p.group] ||= []).push(p);
    return Object.entries(m);
  }, [allPerms]);

  const groupAll = (g: Permission[]) => g.every((p) => sel.has(p.id));
  const toggleGroup = (g: Permission[]) => {
    const n = new Set(sel);
    if (groupAll(g)) g.forEach((p) => n.delete(p.id)); else g.forEach((p) => n.add(p.id));
    setSel(n);
  };
  const togglePerm = (id: string) => { const n = new Set(sel); if (n.has(id)) n.delete(id); else n.add(id); setSel(n); };
  const allSelected = allPerms.length > 0 && allPerms.every((p) => sel.has(p.id));
  const toggleAll = () => setSel(allSelected ? new Set() : new Set(allPerms.map((p) => p.id)));
  const save = () => {
    api.setRolePermissions(roleId, Array.from(sel)).then(() => toast("权限已保存", "success")).catch((e) => toast(e.message, "error"));
  };

  if (loading) return <div className="p-6 text-center"><Spinner /></div>;
  return (
    <div className="flex flex-col overflow-hidden">
      <div className="flex items-center justify-between border-b p-2">
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={allSelected} onChange={toggleAll} /> 全选</label>
        <Button size="sm" onClick={save}>保存权限</Button>
      </div>
      <div className="flex-1 overflow-y-auto p-3">
        {groups.map(([g, perms]) => (
          <div key={g} className="mb-4">
            <div className="mb-1 flex items-center gap-2">
              <input type="checkbox" checked={groupAll(perms)} onChange={() => toggleGroup(perms)} />
              <span className="text-sm font-semibold text-foreground">{groupLabel[g] || g}</span>
              <span className="text-xs text-muted-foreground">({perms.filter((p) => sel.has(p.id)).length}/{perms.length})</span>
            </div>
            <div className="grid grid-cols-2 gap-1 pl-6 lg:grid-cols-3">
              {perms.map((p) => (
                <label key={p.id} className="flex items-center gap-2 rounded px-2 py-1 text-sm hover:bg-accent">
                  <input type="checkbox" checked={sel.has(p.id)} onChange={() => togglePerm(p.id)} />
                  <span>{p.name}</span>
                  <span className="font-mono text-xs text-muted-foreground">{p.code}</span>
                </label>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function RoleModal({ role, onClose, onDone }: { role: Role | {}; onClose: () => void; onDone: () => void }) {
  const r = role as Role;
  const isEdit = !!r.id;
  const [f, setF] = useState<any>({
    code: r.code || "",
    name: r.name || "",
    description: r.description || "",
    enabled: r.enabled !== undefined ? r.enabled : true,
  });
  const set = (k: string, v: any) => setF({ ...f, [k]: v });
  const submit = () => {
    if (isEdit) {
      api.updateRole(r.id, { name: f.name, description: f.description, enabled: f.enabled }).then(() => { toast("已更新", "success"); onDone(); }).catch((e) => toast(e.message, "error"));
    } else {
      api.createRole({ code: f.code, name: f.name, description: f.description, enabled: f.enabled }).then(() => { toast("已创建", "success"); onDone(); }).catch((e) => toast(e.message, "error"));
    }
  };
  return (
    <Modal open onClose={onClose} title={isEdit ? "编辑角色" : "新建角色"}>
      <div className="grid grid-cols-2 gap-3">
        <L label="编码"><Input value={f.code} disabled={isEdit || r.code === "admin"} onChange={(e) => set("code", e.target.value)} /></L>
        <L label="名称"><Input value={f.name} onChange={(e) => set("name", e.target.value)} /></L>
        <div className="col-span-2"><L label="说明"><Input value={f.description} onChange={(e) => set("description", e.target.value)} /></L></div>
        <label className="col-span-2 flex items-center gap-2 text-sm"><input type="checkbox" checked={f.enabled} disabled={(role as Role).code === "admin"} onChange={(e) => set("enabled", e.target.checked)} /> 启用</label>
      </div>
      <div className="mt-4 flex justify-end gap-2"><Button variant="outline" onClick={onClose}>取消</Button><Button onClick={submit}>{isEdit ? "保存" : "创建"}</Button></div>
    </Modal>
  );
}

function L({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="flex flex-col gap-1 text-xs text-muted-foreground">{label}{children}</label>;
}
