import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { LogOut, Search, User as UserIcon, Wand2 } from "lucide-react";
import { useAuthStore } from "@/store/auth";
import { useDictStore } from "@/store/dict";
import { cn } from "@/lib/cn";
import { Logo } from "@/components/Logo";
import { Button } from "@/components/ui";

const navItems = [
  { to: "/", label: "浏览", icon: Search, end: true },
  { to: "/workbench", label: "创建提示词", icon: Wand2, end: false },
];

export function AppLayout({ children }: { children: ReactNode }) {
  const { user, initialized, refresh, logout } = useAuthStore();
  const loadDicts = useDictStore((state) => state.load);
  const navigate = useNavigate();
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement | null>(null);
  const location = useLocation();
  const fullBleed = location.pathname === "/workbench";

  useEffect(() => {
    refresh();
    loadDicts();
  }, [refresh, loadDicts]);

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

  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur">
        <div className={cn("flex h-16 items-center gap-4 px-4", !fullBleed && "mx-auto max-w-6xl")}>
          <Link to="/" className="flex items-center gap-2">
            <Logo className="h-8 w-8" />
            <span className="text-lg font-semibold text-foreground">Prompts Glue</span>
          </Link>

          <nav className="flex items-center gap-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      isActive ? "bg-primary/10 text-primary" : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                    )
                  }
                >
                  <Icon className="h-4 w-4" />
                  <span className="hidden sm:inline">{item.label}</span>
                </NavLink>
              );
            })}
          </nav>

          <div className="ml-auto">
            {initialized ? (
              user ? (
                <div className="relative" ref={userMenuRef}>
                  <button
                    onClick={() => setUserMenuOpen((v) => !v)}
                    className="flex items-center gap-2 rounded-md p-1 pr-2 hover:bg-accent"
                    title={displayName}
                  >
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/15 text-sm font-semibold text-primary ring-1 ring-primary/20">
                      {initial}
                    </span>
                    <span className="hidden text-sm text-foreground sm:inline">{displayName}</span>
                  </button>
                  {userMenuOpen && (
                    <div className="absolute right-0 top-full z-50 mt-1 w-44 rounded-md border bg-popover p-1 text-sm shadow-lg">
                      <Link to="/userspace" onClick={() => setUserMenuOpen(false)} className="flex items-center gap-2 rounded-sm px-2 py-1.5 hover:bg-accent">
                        <UserIcon className="h-4 w-4" /> 个人中心
                      </Link>
                      <button
                        onClick={() => { logout(); setUserMenuOpen(false); navigate("/"); }}
                        className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left hover:bg-accent"
                      >
                        <LogOut className="h-4 w-4" /> 退出登录
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                <Button variant="outline" size="sm" onClick={() => navigate("/login")}>登录</Button>
              )
            ) : null}
          </div>
        </div>
      </header>

      <main className={cn("w-full flex-1", fullBleed ? "px-3 py-3" : "mx-auto max-w-6xl px-4 py-6")}>{children}</main>

      {!fullBleed && (
        <footer className="border-t bg-background">
          <div className="mx-auto flex max-w-6xl flex-col gap-2 px-4 py-6 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
            <span>Prompts Glue · 提示词构建与分发</span>
            <div className="flex items-center gap-4">
              <Link to="/" className="hover:text-foreground">文档</Link>
              <Link to="/" className="hover:text-foreground">CLI 安装</Link>
              <Link to="/" className="hover:text-foreground">关于</Link>
            </div>
          </div>
        </footer>
      )}
    </div>
  );
}
