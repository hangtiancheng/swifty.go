import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/service/api";
import useAuthStore from "@/store/auth";
import useWsStore from "@/store/ws";
import { isValidPhone } from "@/utils/validate";
import { showToast } from "@/utils/toast";
import type { UserInfo } from "@/types";

export default function Register() {
  const [nickname, setNickname] = useState("");
  const [telephone, setTelephone] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const navigate = useNavigate();

  const handleRegister = async () => {
    if (!nickname || !telephone || !password || !confirmPassword) {
      showToast("Please fill in all fields", "error");
      return;
    }
    if (nickname.length < 3 || nickname.length > 10) {
      showToast("Nickname must be 3-10 characters", "error");
      return;
    }
    if (!isValidPhone(telephone)) {
      showToast("Invalid phone number", "error");
      return;
    }
    if (password !== confirmPassword) {
      showToast("Passwords do not match", "error");
      return;
    }
    const res = await api.register({ nickname, telephone, password });
    if (res.code === 200) {
      const data = res.data as UserInfo;
      showToast(res.message, "success");
      useAuthStore.getState().setUserInfo(data);
      useWsStore.getState().connect(data.uuid);
      navigate("/chat/sessions");
    } else {
      showToast(res.message || "Registration failed", "error");
    }
  };

  return (
    <div className="bg-background relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      {/* Ambient soft-pink layers */}
      <div
        aria-hidden
        className="bg-primary/10 pointer-events-none absolute -top-24 -right-32 h-96 w-96 animate-pulse rounded-full blur-3xl [animation-duration:8s] motion-reduce:animate-none"
      />
      <div
        aria-hidden
        className="bg-primary/5 pointer-events-none absolute -bottom-40 -left-24 h-[28rem] w-[28rem] animate-pulse rounded-full blur-3xl [animation-delay:2s] [animation-duration:10s] motion-reduce:animate-none"
      />
      <div
        aria-hidden
        className="bg-primary/[0.07] pointer-events-none absolute bottom-1/4 left-1/3 h-64 w-64 rounded-full blur-3xl"
      />

      <Card className="animate-in fade-in zoom-in-95 border-border shadow-primary/5 w-full max-w-md shadow-xl duration-300">
        <CardHeader>
          <CardTitle className="text-2xl font-semibold tracking-tight">
            Register
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            Create your Swifty Chat account
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="register-nickname">Nickname</Label>
            <Input
              id="register-nickname"
              type="text"
              placeholder="3-10 characters"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="register-phone">Phone</Label>
            <Input
              id="register-phone"
              type="text"
              placeholder="Enter your phone number"
              value={telephone}
              onChange={(e) => setTelephone(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="register-password">Password</Label>
            <Input
              id="register-password"
              type="password"
              placeholder="Enter your password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="register-confirm-password">Confirm Password</Label>
            <Input
              id="register-confirm-password"
              type="password"
              placeholder="Re-enter your password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
          </div>
        </CardContent>
        <CardFooter className="flex-col gap-3">
          <Button className="w-full" onClick={handleRegister}>
            Register
          </Button>
          <div className="flex w-full justify-end">
            <a
              className="text-primary cursor-pointer text-sm hover:underline"
              onClick={() => navigate("/login")}
            >
              Sign In
            </a>
          </div>
        </CardFooter>
      </Card>
    </div>
  );
}
