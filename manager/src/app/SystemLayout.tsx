import { NavLink, Outlet, useLocation } from "react-router-dom";
import { ChevronRight, Database, ScrollText, Shield, ShieldCheck, Sparkles } from "lucide-react";
import { cn } from "@/lib/cn";

const items = [
  { path: "/system/manager-users", label: "管理端用户", icon: ShieldCheck },
  { path: "/system/dicts", label: "字典", icon: Database },
  { path: "/system/roles", label: "角色", icon: Shield },
  { path: "/system/ai-logs", label: "AI日志", icon: Sparkles },
  { path: "/system/audit", label: "审计", icon: ScrollText },
];

export function SystemLayout() {
  const location = useLocation();
  return (
    <div className="flex gap-6">
      <aside className="w-52 shrink-0">
        <h2 className="mb-4 text-lg font-semibold text-foreground">系统管理</h2>
        <nav className="space-y-1">
          {items.map((item) => {
            const Icon = item.icon;
            const isActive = location.pathname.startsWith(item.path);
            return (
              <NavLink
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-3 rounded-md border px-3 py-2 text-sm transition-colors active:scale-[0.98]",
                  isActive
                    ? "border-primary/30 bg-primary text-primary-foreground shadow-sm"
                    : "border-transparent text-muted-foreground hover:border-border hover:bg-accent hover:text-accent-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                <span className="flex-1">{item.label}</span>
                {isActive && <ChevronRight className="h-4 w-4" />}
              </NavLink>
            );
          })}
        </nav>
      </aside>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  );
}
