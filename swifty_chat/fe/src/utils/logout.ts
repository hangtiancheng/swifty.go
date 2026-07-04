import { api } from "../service/api";
import useWsStore from "../store/ws";
import useAuthStore from "../store/auth";

/**
 * Tear down auth + websocket session. Callers should navigate
 * to "/login" after this resolves.
 */
export async function performLogout(): Promise<void> {
  const uid = useAuthStore.getState().userInfo.uuid;
  await api.wsLogout({ owner_id: uid });
  useWsStore.getState().disconnect();
  useAuthStore.getState().clearUserInfo();
}
