import { useNavigate } from "react-router-dom";
import { User } from "lucide-react";
import { Card } from "@/components/ui/card";
import { NavBar } from "@/components/nav-bar";
import { ContactSidebar } from "@/components/contact-sidebar";
import useAuthStore from "@/store/auth";
import { performLogout } from "@/utils/logout";

export default function ContactList() {
  const navigate = useNavigate();
  const userInfo = useAuthStore((s) => s.userInfo);

  const handleLogout = async () => {
    await performLogout();
    navigate("/login");
  };

  return (
    <div className="bg-background flex min-h-screen items-center justify-center p-4">
      <Card className="shadow-primary/5 flex h-[600px] w-[1000px] flex-row overflow-hidden shadow-xl">
        <NavBar
          avatar={userInfo.avatar}
          isAdmin={userInfo.is_admin === 1}
          onNavigate={(path) => navigate(path)}
          onLogout={handleLogout}
        />
        <div className="border-border w-55 border-r">
          <ContactSidebar onNavigate={(id) => navigate(`/chat/${id}`)} />
        </div>
        <div className="text-muted-foreground/50 flex flex-1 flex-col items-center justify-center">
          <User size={64} strokeWidth={1.5} className="mb-4" />
          <p className="text-muted-foreground/70">
            Select a contact to start chatting
          </p>
        </div>
      </Card>
    </div>
  );
}
