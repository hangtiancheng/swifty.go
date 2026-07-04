import { useNavigate } from "react-router-dom";
import { MessageSquare } from "lucide-react";
import { NavBarComponent } from "../components/nav-bar.react";
import { SessionSidebarComponent } from "../components/session-sidebar.react";
import useAuthStore from "../store/auth";
import { performLogout } from "../utils/logout";

export default function SessionList() {
  const navigate = useNavigate();
  const userInfo = useAuthStore((s) => s.userInfo);

  const handleNavigate = (e: CustomEvent<string>) => {
    navigate(e.detail);
  };

  const handleLogout = async () => {
    await performLogout();
    navigate("/login");
  };

  return (
    <div className="bg-base-200 flex min-h-screen items-center justify-center p-4">
      <div className="card card-border border-base-300 bg-base-100 flex h-150 w-250 flex-row overflow-hidden shadow-xl">
        <NavBarComponent
          avatar={userInfo.avatar}
          isAdmin={userInfo.is_admin === 1}
          onNavigate={handleNavigate}
          onLogout={handleLogout}
        />
        <div className="border-base-300 w-55 border-r">
          <SessionSidebarComponent
            onChat={(e: CustomEvent<string>) => navigate(`/chat/${e.detail}`)}
          />
        </div>
        <div className="text-base-content/30 flex flex-1 flex-col items-center justify-center">
          <span className="text-base-content/30 mb-4 h-16 w-16">
            <MessageSquare size={64} strokeWidth={1.5} />
          </span>
          <p className="text-base-content/40">
            Select a conversation to start chatting
          </p>
        </div>
      </div>
    </div>
  );
}
