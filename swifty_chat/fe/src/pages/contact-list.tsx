import { useNavigate } from "react-router-dom";
import { User } from "lucide-react";
import { NavBarComponent } from "../components/nav-bar.react";
import { ContactSidebarComponent } from "../components/contact-sidebar.react";
import useAuthStore from "../store/auth";
import { performLogout } from "../utils/logout";

export default function ContactList() {
  const navigate = useNavigate();
  const userInfo = useAuthStore((s) => s.userInfo);

  const handleNavBarNavigate = (e: CustomEvent<string>) => {
    navigate(e.detail);
  };

  const handleSidebarNavigate = (e: CustomEvent<string>) => {
    navigate(`/chat/${e.detail}`);
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
          onNavigate={handleNavBarNavigate}
          onLogout={handleLogout}
        />
        <div className="border-base-300 w-55 border-r">
          <ContactSidebarComponent onNavigate={handleSidebarNavigate} />
        </div>
        <div className="text-base-content/30 flex flex-1 flex-col items-center justify-center">
          <span className="mb-4 h-16 w-16">
            <User size={64} strokeWidth={1.5} />
          </span>
          <p className="text-base-content/40">
            Select a contact to start chatting
          </p>
        </div>
      </div>
    </div>
  );
}
