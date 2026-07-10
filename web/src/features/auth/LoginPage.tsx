import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuthStore } from "@/store/auth";
import { Button, Card, Input } from "@/components/ui";
import { Logo } from "@/components/Logo";

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { login, loading, error } = useAuthStore();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const from = (location.state as { from?: string } | null)?.from;

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const ok = await login(username, password);
    if (ok) navigate(from ?? "/", { replace: true });
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/40">
      <Card className="w-96 p-6">
        <div className="mb-5 flex items-center gap-2">
          <Logo className="h-9 w-9" />
          <div>
            <h1 className="text-lg font-bold text-foreground">Prompts Glue</h1>
            <p className="text-xs text-muted-foreground">用户端门户</p>
          </div>
        </div>
        <form onSubmit={submit} className="space-y-3">
          <Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="用户名" autoComplete="username" />
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="密码" autoComplete="current-password" />
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
          <Button type="submit" className="w-full" disabled={loading}>{loading ? "登录中…" : "登录"}</Button>
        </form>
      </Card>
    </div>
  );
}
