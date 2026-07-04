import { BASE_URL } from "../config";

export function resolveAvatar(avatar: string): string {
  if (!avatar) return "";
  if (avatar.startsWith("http")) return avatar;
  return BASE_URL + avatar;
}
