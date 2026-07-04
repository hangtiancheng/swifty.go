import { create } from "zustand";
import type { UserInfo } from "../types";
import { resolveAvatar } from "../utils/avatar";

export interface AuthState {
  userInfo: UserInfo;
  isLoggedIn: boolean;
  setUserInfo: (info: UserInfo) => void;
  clearUserInfo: () => void;
}

const emptyUser: UserInfo = {
  uuid: "",
  nickname: "",
  telephone: "",
  email: "",
  avatar: "",
  gender: 0,
  birthday: "",
  signature: "",
  status: 0,
  is_admin: 0,
  created_at: "",
};

function loadUserInfo(): UserInfo {
  try {
    const raw = sessionStorage.getItem("userInfo");
    return raw ? JSON.parse(raw) : { ...emptyUser };
  } catch {
    return { ...emptyUser };
  }
}

const initialUser = loadUserInfo();

const useAuthStore = create<AuthState>((set) => ({
  userInfo: initialUser,
  isLoggedIn: !!initialUser.uuid,
  setUserInfo(info: UserInfo) {
    info.avatar = resolveAvatar(info.avatar);
    sessionStorage.setItem("userInfo", JSON.stringify(info));
    set({ userInfo: info, isLoggedIn: !!info.uuid });
  },
  clearUserInfo() {
    sessionStorage.removeItem("userInfo");
    set({ userInfo: { ...emptyUser }, isLoggedIn: false });
  },
}));

export default useAuthStore;
export { emptyUser };
