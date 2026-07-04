import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Paperclip, Video } from "lucide-react";
import { NavBarComponent } from "../components/nav-bar.react";
import { SessionSidebarComponent } from "../components/session-sidebar.react";
import { MessageBubbleComponent } from "../components/message-bubble.react";
import { VideoCallComponent } from "../components/video-call.react";
import type { VideoCall } from "../components/video-call";
import { api } from "../service/api";
import useAuthStore from "../store/auth";
import useChatStore from "../store/chat";
import useWsStore from "../store/ws";
import { resolveAvatar } from "../utils/avatar";
import { showToast } from "../utils/toast";
import { performLogout } from "../utils/logout";
import { getFileSize } from "../utils/format";
import { BASE_URL } from "../config";
import type { ContactInfo, Message } from "../types";

type MemberRow = Record<string, string>;
type JoinRequestRow = Record<string, string>;

export default function Chat() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const userInfo = useAuthStore((s) => s.userInfo);
  const contactInfo = useChatStore((s) => s.contactInfo);
  const messageList = useChatStore((s) => s.messageList);

  const [chatMessage, setChatMessage] = useState("");
  const [memberList, setMemberList] = useState<MemberRow[]>([]);
  const [joinRequestList, setJoinRequestList] = useState<JoinRequestRow[]>([]);
  const [selectedMembers, setSelectedMembers] = useState<string[]>([]);

  // Edit group modal buffers
  const [editGroupName, setEditGroupName] = useState("");
  const [editGroupNotice, setEditGroupNotice] = useState("");
  const [editGroupAddMode, setEditGroupAddMode] = useState(-1);
  const [groupAvatarFile, setGroupAvatarFile] = useState<File | null>(null);

  const videoCallRef = useRef<VideoCall | null>(null);

  // Derived contact fields
  const contactId = contactInfo?.contact_id ?? "";
  const contactName = contactInfo?.contact_name ?? "";
  const contactAvatar = contactInfo?.contact_avatar ?? "";
  const isUserContact = contactId.startsWith("U");
  const isGroupContact = contactId !== "" && !isUserContact;
  const isGroupOwner = contactInfo?.contact_owner_id === userInfo.uuid;
  const contactGenderText = isUserContact
    ? contactInfo?.contact_gender === 0
      ? "Male"
      : "Female"
    : "";
  const contactPhone = contactInfo?.contact_phone ?? "";
  const contactEmail = contactInfo?.contact_email ?? "";
  const contactBirthday = contactInfo?.contact_birthday ?? "";
  const contactSignature = contactInfo?.contact_signature ?? "";
  const groupMemberCnt = isGroupContact
    ? (contactInfo?.contact_member_cnt ?? 0)
    : 0;
  const groupOwnerId = contactInfo?.contact_owner_id ?? "";
  const groupAddModeText = isGroupContact
    ? contactInfo?.contact_add_mode === 0
      ? "Direct Join"
      : "Owner Approval"
    : "";
  const groupNotice = contactInfo?.contact_notice ?? "";

  const scrollToBottom = () => {
    const el = document.getElementById("chat-messages");
    if (el) el.scrollTop = el.scrollHeight;
  };

  const loadMessages = async (cid: string) => {
    const isUser = cid.startsWith("U");
    const res = isUser
      ? await api.getMessageList({
          send_id: userInfo.uuid,
          receive_id: cid,
        })
      : await api.getGroupMessageList({ group_id: cid });
    if (res.code === 200 && res.data) {
      const list = ((res.data as Message[]) || []).map((m) => ({
        ...m,
        send_avatar: resolveAvatar(m.send_avatar),
      }));
      useChatStore.getState().setMessageList(list);
    }
  };

  const loadChat = async (cid: string) => {
    const res = await api.getContactInfo({
      user_id: userInfo.uuid,
      contact_id: cid,
    });
    if (res.code !== 200 || !res.data) return;
    const info = res.data as ContactInfo;
    info.contact_avatar = resolveAvatar(info.contact_avatar);
    useChatStore.getState().setContact(info);

    const sessionRes = await api.openSession({
      send_id: userInfo.uuid,
      receive_id: cid,
    });
    if (sessionRes.code === 200) {
      useChatStore.getState().setSessionId(sessionRes.data as string);
      await loadMessages(cid);
    }
  };

  // Load chat on id change
  useEffect(() => {
    useChatStore.getState().clearChat();
    if (id) {
      loadChat(id);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  // Auto scroll to bottom on new messages
  useEffect(() => {
    scrollToBottom();
  }, [messageList]);

  // Subscribe to websocket messages
  useEffect(() => {
    const handleWsMessage = (event: MessageEvent) => {
      const raw = event.data;
      if (typeof raw !== "string" || raw.length === 0 || raw.charAt(0) !== "{")
        return;
      let message: Message;
      try {
        message = JSON.parse(raw) as Message;
      } catch {
        return;
      }
      const auth = useAuthStore.getState();
      const chat = useChatStore.getState();

      if (message.type === 3) {
        try {
          const avData = JSON.parse(message.av_data || "{}") as Record<
            string,
            unknown
          >;
          videoCallRef.current?.handleSignal(avData);
        } catch {
          /* ignore malformed signal */
        }
        return;
      }

      const isRelevant =
        (message.receive_id.startsWith("G") &&
          message.receive_id === chat.contactInfo?.contact_id) ||
        (message.receive_id.startsWith("U") &&
          message.receive_id === auth.userInfo.uuid) ||
        message.send_id === auth.userInfo.uuid;

      if (isRelevant) {
        message.send_avatar = resolveAvatar(message.send_avatar);
        useChatStore.getState().addMessage(message);
      }
    };
    useWsStore.getState().setOnMessage(handleWsMessage);
    return () => {
      useWsStore.getState().setOnMessage(() => {});
    };
  }, []);

  const handleNavBarNavigate = (e: CustomEvent<string>) => navigate(e.detail);
  const handleSidebarChat = (e: CustomEvent<string>) =>
    navigate(`/chat/${e.detail}`);
  const handleLogout = async () => {
    await performLogout();
    navigate("/login");
  };

  const sendMessage = () => {
    if (!chatMessage.trim() || !contactInfo) return;
    const msg: Message = {
      session_id: useChatStore.getState().sessionId,
      type: 0,
      content: chatMessage,
      url: "",
      send_id: userInfo.uuid,
      send_name: userInfo.nickname,
      send_avatar: userInfo.avatar,
      receive_id: contactInfo.contact_id,
      file_size: getFileSize(0),
      file_name: "",
      file_type: "",
      created_at: new Date().toISOString(),
    };
    useWsStore.getState().send(msg);
    setChatMessage("");
  };

  const onFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const formData = new FormData();
    formData.append("file", file);
    const res = await api.uploadFile(formData);
    const data = res.data as { url?: string; file_name?: string } | null;
    const uploadedUrl = data?.url || "";
    const uploadedName = data?.file_name || file.name;
    if (!uploadedUrl) {
      showToast("File upload failed", "error");
      return;
    }
    showToast("File uploaded successfully", "success");
    if (!contactInfo) return;
    const msg: Message = {
      session_id: useChatStore.getState().sessionId,
      type: 2,
      content: "",
      url: BASE_URL + uploadedUrl,
      send_id: userInfo.uuid,
      send_name: userInfo.nickname,
      send_avatar: userInfo.avatar,
      receive_id: contactInfo.contact_id,
      file_size: getFileSize(file.size),
      file_name: uploadedName,
      file_type: file.type,
      created_at: new Date().toISOString(),
    };
    useWsStore.getState().send(msg);
    e.target.value = "";
  };

  const openVideoCall = () => {
    videoCallRef.current?.show();
  };

  const deleteSession = async () => {
    const sessionId = useChatStore.getState().sessionId;
    await api.deleteSession({ owner_id: userInfo.uuid, session_id: sessionId });
    navigate("/chat/sessions");
  };

  const deleteContact = async () => {
    if (!contactInfo) return;
    await api.deleteContact({
      user_id: userInfo.uuid,
      contact_id: contactInfo.contact_id,
    });
    showToast("Contact removed", "success");
    navigate("/chat/sessions");
  };

  const blackContact = async () => {
    if (!contactInfo) return;
    await api.blackContact({
      user_id: userInfo.uuid,
      contact_id: contactInfo.contact_id,
    });
    showToast("Contact blocked", "success");
    navigate("/chat/sessions");
  };

  const dismissGroup = async () => {
    if (!contactInfo) return;
    const res = await api.dismissGroup({ group_id: contactInfo.contact_id });
    if (res.code === 200) {
      showToast("Group disbanded", "success");
      navigate("/chat/sessions");
    } else {
      showToast(res.message, "error");
    }
  };

  const leaveGroup = async () => {
    if (!contactInfo) return;
    const res = await api.leaveGroup({
      user_id: userInfo.uuid,
      group_id: contactInfo.contact_id,
    });
    if (res.code === 200) {
      showToast("Left group", "success");
      navigate("/chat/sessions");
    } else {
      showToast(res.message, "error");
    }
  };

  const showEditGroupModal = () => {
    setEditGroupName("");
    setEditGroupNotice("");
    setEditGroupAddMode(-1);
    setGroupAvatarFile(null);
    (
      document.getElementById("edit-group-modal") as HTMLDialogElement
    )?.showModal();
  };

  const saveGroupInfo = async () => {
    if (
      !editGroupName &&
      !editGroupNotice &&
      editGroupAddMode === -1 &&
      !groupAvatarFile
    ) {
      showToast("Please modify at least one field", "warning");
      return;
    }
    if (
      editGroupName &&
      (editGroupName.length < 3 || editGroupName.length > 10)
    ) {
      showToast("Group name must be 3-10 characters", "error");
      return;
    }
    if (!contactInfo) return;
    if (groupAvatarFile) {
      const formData = new FormData();
      formData.append("file", groupAvatarFile);
      await api.uploadAvatar(formData);
    }
    const data: Record<string, unknown> = { uuid: contactInfo.contact_id };
    if (editGroupName) data.name = editGroupName;
    if (editGroupNotice) data.notice = editGroupNotice;
    if (editGroupAddMode !== -1) data.add_mode = editGroupAddMode;
    if (groupAvatarFile)
      data.avatar = "/static/avatars/" + groupAvatarFile.name;
    const res = await api.updateGroupInfo(data);
    if (res.code === 200) {
      showToast("Group updated", "success");
      (
        document.getElementById("edit-group-modal") as HTMLDialogElement
      )?.close();
      loadChat(contactInfo.contact_id);
    } else {
      showToast(res.message, "error");
    }
  };

  const showRemoveMembersModal = async () => {
    if (!contactInfo) return;
    setSelectedMembers([]);
    const res = await api.getGroupMemberList({
      group_id: contactInfo.contact_id,
    });
    const list = ((res.data as MemberRow[]) || []).map((m) => ({
      ...m,
      avatar: resolveAvatar(m.avatar),
    }));
    setMemberList(list);
    (
      document.getElementById("remove-members-modal") as HTMLDialogElement
    )?.showModal();
  };

  const toggleMember = (mid: string, checked: boolean) => {
    setSelectedMembers((prev) =>
      checked ? [...prev, mid] : prev.filter((x) => x !== mid),
    );
  };

  const removeSelectedMembers = async () => {
    if (selectedMembers.length === 0) {
      showToast("Please select members to remove", "warning");
      return;
    }
    if (!contactInfo) return;
    const res = await api.removeGroupMembers({
      group_id: contactInfo.contact_id,
      member_ids: selectedMembers,
    });
    if (res.code === 200) {
      showToast("Members removed", "success");
      const remaining = memberList.filter(
        (m) => !selectedMembers.includes(m.user_id),
      );
      setMemberList(remaining);
      setSelectedMembers([]);
    } else {
      showToast(res.message, "error");
    }
  };

  const showJoinRequestsModal = async () => {
    const res = await api.getAddGroupList({ user_id: userInfo.uuid });
    const list = ((res.data as JoinRequestRow[]) || []).filter(
      (r) => r.contact_type === "1",
    );
    if (list.length === 0) {
      showToast("No pending join requests", "info");
      return;
    }
    setJoinRequestList(list);
    (
      document.getElementById("join-requests-modal") as HTMLDialogElement
    )?.showModal();
  };

  const approveJoinRequest = async (applyId: string) => {
    const res = await api.passContactApply({ apply_id: applyId });
    if (res.code === 200) {
      showToast("Approved", "success");
      setJoinRequestList((prev) => prev.filter((r) => r.apply_id !== applyId));
    } else {
      showToast(res.message, "error");
    }
  };

  const rejectJoinRequest = async (applyId: string) => {
    const res = await api.refuseContactApply({ apply_id: applyId });
    if (res.code === 200) {
      showToast("Rejected", "success");
      setJoinRequestList((prev) => prev.filter((r) => r.apply_id !== applyId));
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
          <SessionSidebarComponent onChat={handleSidebarChat} />
        </div>
        <div className="flex flex-1 flex-col">
          <div className="border-base-300 bg-base-200/50 flex h-14 items-center justify-between border-b px-4">
            <div className="flex items-center gap-3">
              {contactAvatar && (
                <div className="avatar">
                  <div className="ring-base-300 ring-offset-base-100 w-10 rounded-full ring ring-offset-1">
                    <img src={contactAvatar} />
                  </div>
                </div>
              )}
              <h2 className="text-base-content text-base font-semibold">
                {contactName}
              </h2>
            </div>
            <div className="dropdown dropdown-end">
              <div
                tabIndex={0}
                role="button"
                className="btn btn-ghost btn-sm btn-square text-base-content/70 font-normal"
              >
                <span className="text-lg leading-none">⋮</span>
              </div>
              <ul
                tabIndex={0}
                className="dropdown-content menu rounded-box border-base-300 bg-base-100 z-10 w-44 border p-2 shadow-lg"
              >
                {isUserContact && (
                  <li>
                    <a
                      className="text-base-content hover:bg-base-200 text-sm"
                      onClick={() =>
                        (
                          document.getElementById(
                            "user-info-modal",
                          ) as HTMLDialogElement
                        )?.showModal()
                      }
                    >
                      User Info
                    </a>
                  </li>
                )}
                {isGroupContact && (
                  <li>
                    <a
                      className="text-base-content hover:bg-base-200 text-sm"
                      onClick={() =>
                        (
                          document.getElementById(
                            "group-info-modal",
                          ) as HTMLDialogElement
                        )?.showModal()
                      }
                    >
                      Group Info
                    </a>
                  </li>
                )}
                {isGroupContact && isGroupOwner && (
                  <>
                    <li>
                      <a
                        className="text-base-content hover:bg-base-200 text-sm"
                        onClick={showEditGroupModal}
                      >
                        Edit Group
                      </a>
                    </li>
                    <li>
                      <a
                        className="text-base-content hover:bg-base-200 text-sm"
                        onClick={showRemoveMembersModal}
                      >
                        Remove Members
                      </a>
                    </li>
                    <li>
                      <a
                        className="text-base-content hover:bg-base-200 text-sm"
                        onClick={showJoinRequestsModal}
                      >
                        Join Requests
                      </a>
                    </li>
                  </>
                )}
                <li>
                  <a
                    className="text-base-content hover:bg-base-200 text-sm"
                    onClick={deleteSession}
                  >
                    Delete Session
                  </a>
                </li>
                {isUserContact && (
                  <>
                    <li>
                      <a
                        className="text-base-content hover:bg-base-200 text-sm"
                        onClick={deleteContact}
                      >
                        Remove Contact
                      </a>
                    </li>
                    <li>
                      <a
                        className="text-error hover:bg-error/10 text-sm"
                        onClick={blackContact}
                      >
                        Block Contact
                      </a>
                    </li>
                  </>
                )}
                {isGroupContact && isGroupOwner && (
                  <li>
                    <a
                      className="text-error hover:bg-error/10 text-sm"
                      onClick={dismissGroup}
                    >
                      Disband Group
                    </a>
                  </li>
                )}
                {isGroupContact && !isGroupOwner && (
                  <li>
                    <a
                      className="text-base-content hover:bg-base-200 text-sm"
                      onClick={leaveGroup}
                    >
                      Leave Group
                    </a>
                  </li>
                )}
              </ul>
            </div>
          </div>

          <div
            className="bg-base-200 flex-1 overflow-y-auto p-4"
            id="chat-messages"
          >
            <MessageBubbleComponent
              messageList={messageList}
              currentUserId={userInfo.uuid}
              currentUserAvatar={userInfo.avatar}
              currentUserName={userInfo.nickname}
            />
          </div>

          <div className="border-base-300 bg-base-200 flex h-10 items-center justify-between gap-1 border-t px-2">
            <div className="flex items-center gap-1">
              <label className="btn btn-ghost btn-sm btn-square text-base-content/70 hover:bg-base-300 cursor-pointer font-normal">
                <input type="file" className="hidden" onChange={onFileSelect} />
                <Paperclip size={16} />
              </label>
            </div>
            <div className="tooltip tooltip-left" data-tip="Video Call">
              <button
                className="btn btn-ghost btn-sm btn-square text-base-content/70 hover:bg-base-300 font-normal"
                onClick={openVideoCall}
              >
                <Video size={16} />
              </button>
            </div>
          </div>

          <VideoCallComponent ref={videoCallRef} />

          <div className="border-base-300 flex h-45 border-t">
            <textarea
              className="textarea textarea-ghost bg-base-100 placeholder-base-content/40 flex-1 resize-none p-3 text-sm"
              placeholder="Type a message..."
              maxLength={500}
              value={chatMessage}
              onChange={(e) => setChatMessage(e.target.value)}
            />
            <div className="flex w-17 flex-col-reverse p-2">
              <button
                className="btn btn-accent h-10 text-sm font-normal"
                onClick={sendMessage}
              >
                Send
              </button>
            </div>
          </div>
        </div>

        {/* User Info Modal */}
        <dialog id="user-info-modal" className="modal">
          <div className="modal-box border-base-300 rounded-box w-96 border">
            <h3 className="text-base-content mb-4 text-base font-semibold">
              User Profile
            </h3>
            <div className="space-y-2 text-sm">
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">ID</span>
                <span className="text-base-content">{contactId}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Name</span>
                <span className="text-base-content">{contactName}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Gender</span>
                <span className="text-base-content">{contactGenderText}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Phone</span>
                <span className="text-base-content">{contactPhone}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Email</span>
                <span className="text-base-content">{contactEmail}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Birthday</span>
                <span className="text-base-content">{contactBirthday}</span>
              </div>
              <div className="py-1">
                <span className="text-base-content/60">Signature</span>
                <p className="text-base-content mt-1">{contactSignature}</p>
              </div>
            </div>
            <div className="modal-action">
              <button
                className="btn btn-sm btn-ghost font-normal"
                onClick={() =>
                  (
                    document.getElementById(
                      "user-info-modal",
                    ) as HTMLDialogElement
                  )?.close()
                }
              >
                Close
              </button>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>

        {/* Group Info Modal */}
        <dialog id="group-info-modal" className="modal">
          <div className="modal-box border-base-300 rounded-box w-96 border">
            <h3 className="text-base-content mb-4 text-base font-semibold">
              Group Info
            </h3>
            <div className="space-y-2 text-sm">
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">ID</span>
                <span className="text-base-content">{contactId}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Name</span>
                <span className="text-base-content">{contactName}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Members</span>
                <span className="text-base-content">{groupMemberCnt}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Owner</span>
                <span className="text-base-content">{groupOwnerId}</span>
              </div>
              <div className="border-base-200 flex justify-between border-b py-1">
                <span className="text-base-content/60">Join Mode</span>
                <span className="text-base-content">{groupAddModeText}</span>
              </div>
              <div className="py-1">
                <span className="text-base-content/60">Notice</span>
                <p className="text-base-content mt-1">{groupNotice}</p>
              </div>
            </div>
            <div className="modal-action">
              <button
                className="btn btn-sm btn-ghost font-normal"
                onClick={() =>
                  (
                    document.getElementById(
                      "group-info-modal",
                    ) as HTMLDialogElement
                  )?.close()
                }
              >
                Close
              </button>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>

        {/* Edit Group Modal */}
        <dialog id="edit-group-modal" className="modal">
          <div className="modal-box border-base-300 rounded-box w-96 border">
            <h3 className="text-base-content mb-4 text-base font-semibold">
              Edit Group
            </h3>
            <fieldset className="fieldset space-y-3">
              <label className="label text-base-content/70 text-sm">
                Group Name
              </label>
              <input
                type="text"
                className="input input-bordered input-sm w-full"
                placeholder="3-10 characters"
                value={editGroupName}
                onChange={(e) => setEditGroupName(e.target.value)}
              />
              <label className="label text-base-content/70 text-sm">
                Notice
              </label>
              <textarea
                className="textarea textarea-bordered textarea-sm w-full"
                rows={3}
                placeholder="Optional"
                maxLength={500}
                value={editGroupNotice}
                onChange={(e) => setEditGroupNotice(e.target.value)}
              />
              <label className="label text-base-content/70 text-sm">
                Join Mode
              </label>
              <div className="flex gap-4">
                <label className="label cursor-pointer gap-2">
                  <input
                    type="radio"
                    name="addMode"
                    className="radio radio-sm radio-primary"
                    value={0}
                    checked={editGroupAddMode === 0}
                    onChange={() => setEditGroupAddMode(0)}
                  />
                  <span className="text-sm">Direct Join</span>
                </label>
                <label className="label cursor-pointer gap-2">
                  <input
                    type="radio"
                    name="addMode"
                    className="radio radio-sm radio-primary"
                    value={1}
                    checked={editGroupAddMode === 1}
                    onChange={() => setEditGroupAddMode(1)}
                  />
                  <span className="text-sm">Owner Approval</span>
                </label>
              </div>
              <label className="label text-base-content/70 text-sm">
                Avatar
              </label>
              <input
                type="file"
                className="file-input file-input-bordered file-input-sm w-full"
                accept="image/*"
                onChange={(e) =>
                  setGroupAvatarFile(e.target.files?.[0] ?? null)
                }
              />
            </fieldset>
            <div className="modal-action">
              <button
                className="btn btn-sm btn-accent font-normal"
                onClick={saveGroupInfo}
              >
                Save
              </button>
              <button
                className="btn btn-sm btn-ghost font-normal"
                onClick={() =>
                  (
                    document.getElementById(
                      "edit-group-modal",
                    ) as HTMLDialogElement
                  )?.close()
                }
              >
                Cancel
              </button>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>

        {/* Remove Members Modal */}
        <dialog id="remove-members-modal" className="modal">
          <div className="modal-box border-base-300 rounded-box w-96 border">
            <h3 className="text-base-content mb-4 text-base font-semibold">
              Remove Group Members
            </h3>
            {memberList.length === 0 && (
              <p className="text-base-content/40 py-4 text-center text-sm">
                No members found
              </p>
            )}
            <div className="max-h-60 space-y-1 overflow-y-auto">
              {memberList.map((m) => (
                <label
                  key={m.user_id}
                  className="border-base-200 hover:bg-base-200/50 flex cursor-pointer items-center justify-between border-b px-2 py-2"
                >
                  <div className="flex items-center gap-2">
                    <div className="avatar">
                      <div className="w-8 rounded-full">
                        <img src={m.avatar} />
                      </div>
                    </div>
                    <span className="text-base-content text-sm">
                      {m.nickname}
                    </span>
                  </div>
                  <input
                    type="checkbox"
                    className="checkbox checkbox-sm checkbox-primary"
                    checked={selectedMembers.includes(m.user_id)}
                    onChange={(e) => toggleMember(m.user_id, e.target.checked)}
                  />
                </label>
              ))}
            </div>
            <div className="modal-action">
              <button
                className="btn btn-sm btn-error font-normal"
                onClick={removeSelectedMembers}
              >
                Remove Selected
              </button>
              <button
                className="btn btn-sm btn-ghost font-normal"
                onClick={() =>
                  (
                    document.getElementById(
                      "remove-members-modal",
                    ) as HTMLDialogElement
                  )?.close()
                }
              >
                Cancel
              </button>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>

        {/* Join Requests Modal */}
        <dialog id="join-requests-modal" className="modal">
          <div className="modal-box border-base-300 rounded-box w-96 border">
            <h3 className="text-base-content mb-4 text-base font-semibold">
              Group Join Requests
            </h3>
            {joinRequestList.length === 0 && (
              <p className="text-base-content/40 py-4 text-center text-sm">
                No pending requests
              </p>
            )}
            <div className="max-h-60 space-y-2 overflow-y-auto">
              {joinRequestList.map((req) => (
                <div
                  key={req.apply_id}
                  className="border-base-200 flex items-center justify-between border-b py-2"
                >
                  <div className="flex items-center gap-2">
                    <span className="text-base-content text-sm">
                      {req.contact_name}
                    </span>
                    {req.message && (
                      <span className="text-base-content/40 text-xs">
                        ({req.message})
                      </span>
                    )}
                  </div>
                  <div className="flex gap-1">
                    <button
                      className="btn btn-xs btn-accent font-normal"
                      onClick={() => approveJoinRequest(req.apply_id)}
                    >
                      Approve
                    </button>
                    <button
                      className="btn btn-xs btn-ghost text-base-content/60 font-normal"
                      onClick={() => rejectJoinRequest(req.apply_id)}
                    >
                      Reject
                    </button>
                  </div>
                </div>
              ))}
            </div>
            <div className="modal-action">
              <button
                className="btn btn-sm btn-ghost font-normal"
                onClick={() =>
                  (
                    document.getElementById(
                      "join-requests-modal",
                    ) as HTMLDialogElement
                  )?.close()
                }
              >
                Close
              </button>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>
      </div>
    </div>
  );
}
