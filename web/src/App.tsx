import { BrowserRouter, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AppLayout } from "@/app/AppLayout";
import { SearchPage } from "@/features/registry/SearchPage";
import { PackageDetailPage } from "@/features/registry/PackageDetailPage";
import { WorkbenchPage } from "@/features/workbench/WorkbenchPage";
import { UserspacePage } from "@/features/userspace/UserspacePage";
import { LoginPage } from "@/features/auth/LoginPage";
import { Feedback } from "@/components/Feedback";
import { useAuthStore } from "@/store/auth";

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, initialized } = useAuthStore();
  const location = useLocation();
  if (!initialized) return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">加载中...</div>;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  return <>{children}</>;
}

function WithLayout({ children }: { children: React.ReactNode }) {
  return <AppLayout>{children}</AppLayout>;
}

export default function App() {
  return (
    <BrowserRouter>
      <Feedback />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<WithLayout><SearchPage /></WithLayout>} />
        <Route path="/packages/:name" element={<WithLayout><PackageDetailPage /></WithLayout>} />
        <Route path="/workbench" element={<WithLayout><WorkbenchPage /></WithLayout>} />
        <Route path="/userspace" element={<WithLayout><RequireAuth><UserspacePage /></RequireAuth></WithLayout>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
