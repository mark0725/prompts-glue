import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "@/store/auth";
import { Button, Card, Input } from "@/components/ui";

export function LoginPage() {
  const navigate = useNavigate();
  const { login, loading, error } = useAuthStore();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const ok = await login(username, password);
    if (ok) navigate("/");
  }

  return (
    <div className="flex h-screen items-center justify-center bg-muted/40">
      <Card className="w-96 p-6">
        <h1 className="text-xl font-bold text-foreground">prompts-glue</h1>
        <p className="mb-5 text-sm text-muted-foreground">管理界面</p>
        <form onSubmit={submit} className="space-y-3">
          <Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="用户名" autoComplete="username" />
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="密码" autoComplete="current-password" />
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "登录中..." : "登录"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
