import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../service/api";
import useAuthStore from "../store/auth";
import useWsStore from "../store/ws";
import { isValidPhone } from "../utils/validate";
import { showToast } from "../utils/toast";
import type { UserInfo } from "../types";

export default function Login() {
  const [telephone, setTelephone] = useState("");
  const [password, setPassword] = useState("");
  const navigate = useNavigate();

  const handleLogin = async () => {
    if (!telephone || !password) {
      showToast("Please fill in all fields", "error");
      return;
    }
    if (!isValidPhone(telephone)) {
      showToast("Invalid phone number", "error");
      return;
    }

    const res = (await api.login({ telephone, password })) as {
      code: number;
      message: string;
      data: UserInfo;
    };
    if (res.code === 200) {
      if (res.data && res.data.status === 1) {
        showToast("This account has been banned", "error");
        return;
      }
      showToast(res.message, "success");
      useAuthStore.getState().setUserInfo(res.data);
      useWsStore.getState().connect(res.data.uuid);
      navigate("/chat/sessions");
    } else {
      showToast(res.message || "Login failed", "error");
    }
  };

  return (
    <div className="bg-base-200 flex min-h-screen items-center justify-center p-4">
      <div className="card border-base-300 bg-base-100 w-full max-w-md border p-8 shadow-xl">
        <h2 className="text-base-content mb-8 text-center text-2xl font-semibold">
          Sign In
        </h2>
        <fieldset className="fieldset space-y-4">
          <label className="label text-base-content/70 text-sm">Phone</label>
          <input
            type="text"
            className="input input-bordered w-full"
            placeholder="Enter your phone number"
            value={telephone}
            onChange={(e) => setTelephone(e.target.value)}
          />

          <label className="label text-base-content/70 text-sm">Password</label>
          <input
            type="password"
            className="input input-bordered w-full"
            placeholder="Enter your password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </fieldset>

        <button
          className="btn btn-accent mt-8 w-full font-normal"
          onClick={handleLogin}
        >
          Sign In
        </button>

        <div className="mt-5 flex justify-end">
          <a
            className="link link-primary text-sm"
            onClick={() => navigate("/register")}
          >
            Register
          </a>
        </div>
      </div>
    </div>
  );
}
