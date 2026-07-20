import { LogOut, MessageSquare, Settings, User, Users } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

interface NavBarProps {
  avatar: string;
  isAdmin: boolean;
  onNavigate: (path: string) => void;
  onLogout: () => void;
}

interface RailItem {
  label: string;
  path: string;
  icon: LucideIcon;
}

const NAV_ITEMS: RailItem[] = [
  { label: "Sessions", path: "/chat/sessions", icon: MessageSquare },
  { label: "Contacts", path: "/chat/contacts", icon: Users },
  { label: "Profile", path: "/chat/profile", icon: User },
];

interface RailButtonProps {
  label: string;
  icon: LucideIcon;
  onClick: () => void;
  destructive?: boolean;
}

function RailButton({
  label,
  icon: Icon,
  onClick,
  destructive,
}: RailButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            aria-label={label}
            onClick={onClick}
            className={cn(
              "transition-all duration-200 hover:scale-105 active:scale-95",
              destructive
                ? "text-destructive hover:bg-destructive/10 hover:text-destructive"
                : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
            )}
          />
        }
      >
        <Icon className="size-5" />
      </TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
  );
}

export function NavBar({ avatar, isAdmin, onNavigate, onLogout }: NavBarProps) {
  return (
    <nav className="border-border bg-muted/50 flex h-full w-16 flex-col items-center border-r py-4">
      <Avatar className="ring-primary/30 ring-offset-card size-10 rounded-full ring-2 ring-offset-2 transition-transform duration-200 hover:scale-105">
        <AvatarImage src={avatar} alt="Your avatar" />
        <AvatarFallback>U</AvatarFallback>
      </Avatar>

      <div className="mt-6 flex flex-col items-center gap-1">
        {NAV_ITEMS.map((item) => (
          <RailButton
            key={item.path}
            label={item.label}
            icon={item.icon}
            onClick={() => onNavigate(item.path)}
          />
        ))}
      </div>

      <div className="flex-1" />

      <div className="flex flex-col items-center gap-1">
        <div className="bg-border mb-2 h-px w-8" aria-hidden="true" />
        {isAdmin && (
          <RailButton
            label="Admin"
            icon={Settings}
            onClick={() => onNavigate("/manager")}
          />
        )}
        <RailButton
          label="Sign Out"
          icon={LogOut}
          onClick={onLogout}
          destructive
        />
      </div>
    </nav>
  );
}
