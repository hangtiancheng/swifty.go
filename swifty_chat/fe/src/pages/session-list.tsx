import { useNavigate } from "react-router-dom";
import { MessageSquare } from "lucide-react";
import { Card } from "@/components/ui/card";
import { NavBar } from "@/components/nav-bar";
import { SessionSidebar } from "@/components/session-sidebar";
import useAuthStore from "@/store/auth";
import { performLogout } from "@/utils/logout";

export default function SessionList() {
  const navigate = useNavigate();
  const userInfo = useAuthStore((s) => s.userInfo);

  const handleLogout = async () => {
    await performLogout();
    navigate("/login");
  };

  return (
    <div className="bg-background flex min-h-screen items-center justify-center p-4">
      <Card className="shadow-primary/5 h-[600px] w-[1000px] flex-row shadow-xl">
        <NavBar
          avatar={userInfo.avatar}
          isAdmin={userInfo.is_admin === 1}
          onNavigate={(path) => navigate(path)}
          onLogout={handleLogout}
        />
        <div className="border-border w-55 border-r">
          <SessionSidebar onChat={(id) => navigate(`/chat/${id}`)} />
        </div>
        <div className="text-muted-foreground/50 flex flex-1 flex-col items-center justify-center">
          <MessageSquare size={64} strokeWidth={1.5} className="mb-4" />
          <p className="text-muted-foreground/70">
            Select a conversation to start chatting
          </p>
        </div>
      </Card>
    </div>
  );
}
