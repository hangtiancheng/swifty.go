import { defineStore, computed } from "@lark.js/mvc";
import type { UserInfo } from "@/types";
import { resolveAvatar } from "@/utils/avatar";

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

const useAuthStore = defineStore("auth", (s) => {
  const store = s as unknown as AuthState;
  return {
    userInfo: loadUserInfo(),
    isLoggedIn: computed(["userInfo"], () => !!store.userInfo.uuid),
    setUserInfo(info: UserInfo) {
      info.avatar = resolveAvatar(info.avatar);
      store.userInfo = info;
      sessionStorage.setItem("userInfo", JSON.stringify(info));
    },
    clearUserInfo() {
      store.userInfo = { ...emptyUser };
      sessionStorage.removeItem("userInfo");
    },
  };
}) as unknown as {
  (): AuthState;
  (view: unknown): AuthState;
};

export default useAuthStore;
export { emptyUser };
