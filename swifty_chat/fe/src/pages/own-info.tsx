import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { NavBarComponent } from "../components/nav-bar.react";
import { ContactSidebarComponent } from "../components/contact-sidebar.react";
import useAuthStore from "../store/auth";
import { api } from "../service/api";
import { showToast } from "../utils/toast";
import { isValidEmail } from "../utils/validate";
import { performLogout } from "../utils/logout";
import type { UserInfo } from "../types";

export default function OwnInfo() {
  const navigate = useNavigate();
  const userInfo = useAuthStore((s) => s.userInfo);

  const [editNick, setEditNick] = useState("");
  const [editEmail, setEditEmail] = useState("");
  const [editBirthday, setEditBirthday] = useState("");
  const [editSig, setEditSig] = useState("");
  const [avatarFile, setAvatarFile] = useState<File | null>(null);

  const handleNavBarNavigate = (e: CustomEvent<string>) => navigate(e.detail);
  const handleSidebarNavigate = (e: CustomEvent<string>) =>
    navigate(`/chat/${e.detail}`);
  const handleLogout = async () => {
    await performLogout();
    navigate("/login");
  };

  const closeEditModal = () => {
    (document.getElementById("edit-modal") as HTMLDialogElement)?.close();
    setEditNick("");
    setEditEmail("");
    setEditBirthday("");
    setEditSig("");
    setAvatarFile(null);
  };

  const saveProfile = async () => {
    if (!editNick && !editEmail && !editBirthday && !editSig && !avatarFile) {
      showToast("Please modify at least one field", "warning");
      return;
    }
    if (editNick && (editNick.length < 3 || editNick.length > 10)) {
      showToast("Nickname must be 3-10 characters", "error");
      return;
    }
    if (editEmail && !isValidEmail(editEmail)) {
      showToast("Invalid email address", "error");
      return;
    }
    const data: Record<string, unknown> = { uuid: userInfo.uuid };
    if (editNick) data.nickname = editNick;
    if (editEmail) data.email = editEmail;
    if (editBirthday) data.birthday = editBirthday;
    if (editSig) data.signature = editSig;
    if (avatarFile) {
      const formData = new FormData();
      formData.append("file", avatarFile);
      await api.uploadAvatar(formData);
      data.avatar = "/static/avatars/" + avatarFile.name;
    }
    const res = await api.updateUserInfo(data);
    if (res.code === 200) {
      showToast(res.message, "success");
      const updated: UserInfo = { ...userInfo };
      if (editNick) updated.nickname = editNick;
      if (editEmail) updated.email = editEmail;
      if (editBirthday) updated.birthday = editBirthday;
      if (editSig) updated.signature = editSig;
      if (avatarFile) updated.avatar = "/static/avatars/" + avatarFile.name;
      useAuthStore.getState().setUserInfo(updated);
      closeEditModal();
    } else {
      showToast(res.message, "error");
    }
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
        <div className="relative flex flex-1 flex-col items-center justify-center p-8">
          <h2 className="text-base-content mb-6 text-xl font-semibold">
            My Profile
          </h2>
          <div className="text-base-content space-y-2 text-sm">
            <p>User ID: {userInfo.uuid}</p>
            <p>Nickname: {userInfo.nickname}</p>
            <p>Phone: {userInfo.telephone}</p>
            <p>Email: {userInfo.email}</p>
            <p>Gender: {userInfo.gender === 0 ? "Male" : "Female"}</p>
            <p>Birthday: {userInfo.birthday}</p>
            <p>Signature: {userInfo.signature}</p>
            <p>Joined: {userInfo.created_at}</p>
            <div className="flex items-center gap-2">
              <span>Avatar:</span>
              {userInfo.avatar && (
                <div className="avatar">
                  <div className="ring-primary/30 ring-offset-base-100 w-10 rounded-full ring ring-offset-1">
                    <img src={userInfo.avatar} />
                  </div>
                </div>
              )}
            </div>
          </div>
          <button
            className="btn btn-accent btn-sm absolute right-6 bottom-6 font-normal"
            onClick={() =>
              (
                document.getElementById("edit-modal") as HTMLDialogElement
              )?.showModal()
            }
          >
            Edit
          </button>

          <dialog id="edit-modal" className="modal">
            <div className="modal-box border-base-300 rounded-box border">
              <h3 className="text-base-content mb-4 text-lg font-semibold">
                Edit Profile
              </h3>
              <fieldset className="fieldset space-y-3">
                <label className="label text-base-content/70 text-sm">
                  Nickname
                </label>
                <input
                  type="text"
                  className="input input-bordered input-sm w-full"
                  placeholder="Optional, 3-10 characters"
                  value={editNick}
                  onChange={(e) => setEditNick(e.target.value)}
                />
                <label className="label text-base-content/70 text-sm">
                  Email
                </label>
                <input
                  type="text"
                  className="input input-bordered input-sm w-full"
                  placeholder="Optional"
                  value={editEmail}
                  onChange={(e) => setEditEmail(e.target.value)}
                />
                <label className="label text-base-content/70 text-sm">
                  Birthday
                </label>
                <input
                  type="text"
                  className="input input-bordered input-sm w-full"
                  placeholder="Optional, e.g. 2024.1.1"
                  value={editBirthday}
                  onChange={(e) => setEditBirthday(e.target.value)}
                />
                <label className="label text-base-content/70 text-sm">
                  Signature
                </label>
                <input
                  type="text"
                  className="input input-bordered input-sm w-full"
                  placeholder="Optional"
                  value={editSig}
                  onChange={(e) => setEditSig(e.target.value)}
                />
                <label className="label text-base-content/70 text-sm">
                  Avatar
                </label>
                <input
                  type="file"
                  className="file-input file-input-bordered file-input-sm w-full"
                  onChange={(e) => setAvatarFile(e.target.files?.[0] ?? null)}
                />
              </fieldset>
              <div className="modal-action">
                <button
                  className="btn btn-sm btn-accent font-normal"
                  onClick={saveProfile}
                >
                  Save
                </button>
                <button
                  className="btn btn-sm btn-ghost text-base-content/60 font-normal"
                  onClick={closeEditModal}
                >
                  Cancel
                </button>
              </div>
            </div>
            <form method="dialog" className="modal-backdrop">
              <button>close</button>
            </form>
          </dialog>
        </div>
      </div>
    </div>
  );
}
