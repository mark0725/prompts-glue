import { useEffect, useRef, useState, type ReactNode } from "react";
import { NavLink, useLocation, useNavigate } from "react-router-dom";
import { LayoutDashboard, ListChecks, LogOut, Package, PanelLeftClose, Settings, ShieldAlert, Users } from "lucide-react";
import { useAuthStore } from "@/store/auth";
import { useDictStore } from "@/store/dict";
import { cn } from "@/lib/cn";
import { Logo } from "@/components/Logo";

const navItems = [
  { to: "/", label: "仪表盘", icon: LayoutDashboard, end: true },
  { to: "/packages", label: "提示词包", icon: Package, end: false },
  { to: "/users", label: "Web 用户", icon: Users, end: false },
  { to: "/risks", label: "风险", icon: ShieldAlert, end: false },
  { to: "/cache", label: "缓存", icon: ListChecks, end: false },
];

const navItemClass =
  "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:scale-[0.98]";
const navInactive = "text-muted-foreground";

export function ManagerLayout({ children }: { children: ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();
  const loadDicts = useDictStore((state) => state.load);
  const [collapsed, setCollapsed] = useState(true);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    loadDicts();
  }, [loadDicts]);

  useEffect(() => {
    if (!userMenuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) setUserMenuOpen(false);
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [userMenuOpen]);

  const displayName = user?.display_name || user?.username || "当前用户";
  const initial = (user?.display_name || user?.username || "U").slice(0, 1).toUpperCase();
  const doLogout = () => {
    logout();
    navigate("/login");
  };
  const isActive = (path: string) => (path === "/" ? location.pathname === "/" : location.pathname.startsWith(path));

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className={cn("flex h-full shrink-0 flex-col border-r bg-card transition-all duration-200", collapsed ? "w-16" : "w-60")}>
        <div className={cn("flex items-center border-b p-4", collapsed ? "justify-center" : "justify-between")}>
          <button onClick={() => setCollapsed((c) => !c)} className="flex items-center gap-2" title={collapsed ? "展开菜单" : "收起菜单"}>
            <Logo className="h-8 w-8 shrink-0" />
            {!collapsed && <span className="text-lg font-semibold text-foreground">Prompts Glue</span>}
          </button>
          {!collapsed && (
            <button
              onClick={() => setCollapsed(true)}
              className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
              title="收起菜单"
            >
              <PanelLeftClose className="h-4 w-4" />
            </button>
          )}
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto p-2">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                title={item.label}
                className={cn(
                  navItemClass,
                  isActive(item.to) ? "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:text-primary-foreground" : navInactive,
                  collapsed && "justify-center",
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                {!collapsed && item.label}
              </NavLink>
            );
          })}
        </nav>

        <div className="mt-auto space-y-1 border-t p-2">
          <NavLink
            to="/system"
            title="系统管理"
            className={cn(
              navItemClass,
              "w-full",
              isActive("/system") ? "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:text-primary-foreground" : navInactive,
              collapsed && "justify-center",
            )}
          >
            <Settings className="h-4 w-4 shrink-0" />
            {!collapsed && "系统管理"}
          </NavLink>
          <div className="group relative" ref={userMenuRef}>
            <button
              onClick={() => setUserMenuOpen((v) => !v)}
              className={cn(navItemClass, "w-full", collapsed && "justify-center")}
              title={displayName}
            >
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/15 text-sm font-semibold text-primary ring-1 ring-primary/20">
                {initial}
              </span>
              {!collapsed && <span className="flex-1 truncate text-left text-sm">{displayName}</span>}
            </button>
            {userMenuOpen && (
              <div className={cn("absolute z-50 w-56 rounded-md border bg-popover p-1 text-sm shadow-lg", collapsed ? "left-full bottom-0 ml-2" : "left-0 right-0 bottom-full mb-1")}>
                <div className="border-b px-3 py-2">
                  <div className="truncate font-medium text-foreground">{displayName}</div>
                  {user?.username && <div className="truncate text-xs text-muted-foreground">@{user.username}</div>}
                </div>
                <button onClick={doLogout} className="mt-1 flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left hover:bg-accent">
                  <LogOut className="h-3.5 w-3.5" /> 退出登录
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>

      <main className="flex flex-1 flex-col overflow-hidden">
        <div className="min-h-0 flex-1 overflow-y-auto p-4 lg:p-6">{children}</div>
      </main>
    </div>
  );
}
