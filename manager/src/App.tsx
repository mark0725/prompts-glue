import { useEffect } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { ManagerLayout } from "@/app/ManagerLayout";
import { SystemLayout } from "@/app/SystemLayout";
import { LoginPage } from "@/features/auth/LoginPage";
import { DashboardPage } from "@/features/dashboard/DashboardPage";
import { PackagesPage } from "@/features/packages/PackagesPage";
import { UsersPage } from "@/features/users/UsersPage";
import { DictsPage } from "@/features/dicts/DictsPage";
import { RisksPage } from "@/features/risks/RisksPage";
import { AuditPage } from "@/features/audit/AuditPage";
import { CachePage } from "@/features/cache/CachePage";
import { RolesPage } from "@/features/system/RolesPage";
import { AILogsPage } from "@/features/system/AILogsPage";
import { Feedback } from "@/components/Feedback";
import { useAuthStore } from "@/store/auth";

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, initialized } = useAuthStore();
  if (!initialized) return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">加载中...</div>;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  const refresh = useAuthStore((state) => state.refresh);
  useEffect(() => {
    refresh();
  }, [refresh]);
  return (
    <BrowserRouter>
      <Feedback />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/*"
          element={
            <RequireAuth>
              <ManagerLayout>
                <Routes>
                  <Route path="/" element={<DashboardPage />} />
                  <Route path="/packages" element={<PackagesPage />} />
                  <Route path="/users" element={<UsersPage />} />
                  <Route path="/risks" element={<RisksPage />} />
                  <Route path="/cache" element={<CachePage />} />
                  <Route path="/system" element={<SystemLayout />}>
                    <Route index element={<Navigate to="/system/dicts" replace />} />
                    <Route path="dicts" element={<DictsPage />} />
                    <Route path="roles" element={<RolesPage />} />
                    <Route path="ai-logs" element={<AILogsPage />} />
                    <Route path="audit" element={<AuditPage />} />
                  </Route>
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </ManagerLayout>
            </RequireAuth>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}
