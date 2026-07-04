import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../service/api";
import useAuthStore from "../store/auth";
import useWsStore from "../store/ws";
import { isValidPhone } from "../utils/validate";
import { showToast } from "../utils/toast";
import type { UserInfo } from "../types";

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
    <div className="bg-base-200 flex min-h-screen items-center justify-center p-4">
      <div className="card card-border border-base-300 bg-base-100 w-full max-w-md p-8 shadow-xl">
        <h2 className="text-base-content mb-8 text-center text-2xl font-semibold">
          Register
        </h2>
        <fieldset className="fieldset space-y-4">
          <label className="label text-base-content/70 text-sm">Nickname</label>
          <input
            type="text"
            className="input input-bordered w-full"
            placeholder="3-10 characters"
            value={nickname}
            onChange={(e) => setNickname(e.target.value)}
          />

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

          <label className="label text-base-content/70 text-sm">
            Confirm Password
          </label>
          <input
            type="password"
            className="input input-bordered w-full"
            placeholder="Re-enter your password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
          />
        </fieldset>

        <button
          className="btn btn-accent mt-8 w-full font-normal"
          onClick={handleRegister}
        >
          Register
        </button>

        <div className="mt-5 flex justify-end">
          <a
            className="link link-primary cursor-pointer text-sm"
            onClick={() => navigate("/login")}
          >
            Sign In
          </a>
        </div>
      </div>
    </div>
  );
}
