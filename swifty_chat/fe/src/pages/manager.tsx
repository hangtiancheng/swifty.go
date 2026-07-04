import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Shield } from "lucide-react";
import { api } from "../service/api";
import useAuthStore from "../store/auth";
import { showToast } from "../utils/toast";

type Panel =
  | "none"
  | "disable-user"
  | "delete-user"
  | "set-admin"
  | "disable-group"
  | "delete-group";
type UserRow = Record<string, string>;
type GroupRow = Record<string, string>;

export default function Manager() {
  const navigate = useNavigate();
  const [currentPanel, setCurrentPanel] = useState<Panel>("none");
  const [userList, setUserList] = useState<UserRow[]>([]);
  const [groupList, setGroupList] = useState<GroupRow[]>([]);
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([]);
  const [selectedGroupIds, setSelectedGroupIds] = useState<string[]>([]);

  const loadUserList = async () => {
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getUserInfoList({ owner_id: uid });
    setUserList((res.data as UserRow[]) || []);
    setSelectedUserIds([]);
  };

  const loadGroupList = async () => {
    const res = await api.getGroupInfoList({});
    setGroupList((res.data as GroupRow[]) || []);
    setSelectedGroupIds([]);
  };

  const showPanel = (panel: Panel) => {
    setCurrentPanel(panel);
    setSelectedUserIds([]);
    setSelectedGroupIds([]);
    if (
      panel === "disable-user" ||
      panel === "delete-user" ||
      panel === "set-admin"
    ) {
      loadUserList();
    } else if (panel === "disable-group" || panel === "delete-group") {
      loadGroupList();
    }
  };

  const toggleUser = (uuid: string, checked: boolean) => {
    setSelectedUserIds((prev) =>
      checked ? [...prev, uuid] : prev.filter((id) => id !== uuid),
    );
  };

  const toggleAllUsers = (checked: boolean) => {
    setSelectedUserIds(checked ? userList.map((u) => u.uuid) : []);
  };

  const toggleGroup = (uuid: string, checked: boolean) => {
    setSelectedGroupIds((prev) =>
      checked ? [...prev, uuid] : prev.filter((id) => id !== uuid),
    );
  };

  const toggleAllGroups = (checked: boolean) => {
    setSelectedGroupIds(checked ? groupList.map((g) => g.group_id) : []);
  };

  const requireSelection = (ids: string[], msg: string): boolean => {
    if (ids.length === 0) {
      showToast(msg, "warning");
      return false;
    }
    return true;
  };

  const enableSelectedUsers = async () => {
    if (!requireSelection(selectedUserIds, "No users selected")) return;
    await api.ableUsers({ uuid_list: selectedUserIds });
    showToast("Users enabled", "success");
    loadUserList();
  };

  const disableSelectedUsers = async () => {
    if (!requireSelection(selectedUserIds, "No users selected")) return;
    await api.disableUsers({ uuid_list: selectedUserIds });
    showToast("Users disabled", "success");
    loadUserList();
  };

  const deleteSelectedUsers = async () => {
    if (!requireSelection(selectedUserIds, "No users selected")) return;
    await api.deleteUsers({ uuid_list: selectedUserIds });
    showToast("Users deleted", "success");
    loadUserList();
  };

  const setAdminSelected = async (isAdmin: number) => {
    if (!requireSelection(selectedUserIds, "No users selected")) return;
    await api.setAdmin({ uuid_list: selectedUserIds, is_admin: isAdmin });
    showToast(isAdmin ? "Admin granted" : "Admin revoked", "success");
    loadUserList();
  };

  const enableSelectedGroups = async () => {
    if (!requireSelection(selectedGroupIds, "No groups selected")) return;
    await api.setGroupsStatus({ uuid_list: selectedGroupIds, status: 0 });
    showToast("Groups enabled", "success");
    loadGroupList();
  };

  const disableSelectedGroups = async () => {
    if (!requireSelection(selectedGroupIds, "No groups selected")) return;
    await api.setGroupsStatus({ uuid_list: selectedGroupIds, status: 1 });
    showToast("Groups disabled", "success");
    loadGroupList();
  };

  const deleteSelectedGroups = async () => {
    if (!requireSelection(selectedGroupIds, "No groups selected")) return;
    await api.deleteGroups({ uuid_list: selectedGroupIds });
    showToast("Groups deleted", "success");
    loadGroupList();
  };

  const backToChat = () => navigate("/chat/sessions");

  const isUserPanel =
    currentPanel === "disable-user" ||
    currentPanel === "delete-user" ||
    currentPanel === "set-admin";
  const isGroupPanel =
    currentPanel === "disable-group" || currentPanel === "delete-group";
  const allUsersChecked =
    userList.length > 0 && selectedUserIds.length === userList.length;
  const allGroupsChecked =
    groupList.length > 0 && selectedGroupIds.length === groupList.length;

  return (
    <div className="bg-base-200 flex min-h-screen items-center justify-center p-4">
      <div className="card card-border border-base-300 bg-base-100 flex h-150 w-250 flex-col overflow-hidden shadow-xl">
        <div className="border-base-300 bg-base-200 flex h-14 items-center justify-between border-b px-6">
          <div className="flex items-center gap-3">
            <span className="text-primary h-6 w-6">
              <Shield size={24} />
            </span>
            <span className="text-base-content text-lg font-semibold">
              Admin Panel
            </span>
          </div>
          <button
            className="btn btn-sm btn-ghost text-base-content/70 hover:bg-base-300 font-normal"
            onClick={backToChat}
          >
            Back
          </button>
        </div>
        <div className="flex flex-1 overflow-hidden">
          <div className="border-base-300 bg-base-200/50 w-48 border-r">
            <ul className="menu text-sm">
              <li className="menu-title text-primary">Users</li>
              <li>
                <a
                  className="text-base-content hover:bg-base-200"
                  onClick={() => showPanel("disable-user")}
                >
                  Enable / Disable
                </a>
              </li>
              <li>
                <a
                  className="text-base-content hover:bg-base-200"
                  onClick={() => showPanel("delete-user")}
                >
                  Delete
                </a>
              </li>
              <li>
                <a
                  className="text-base-content hover:bg-base-200"
                  onClick={() => showPanel("set-admin")}
                >
                  Set as Admin
                </a>
              </li>
              <li className="menu-title text-primary">Groups</li>
              <li>
                <a
                  className="text-base-content hover:bg-base-200"
                  onClick={() => showPanel("disable-group")}
                >
                  Enable / Disable
                </a>
              </li>
              <li>
                <a
                  className="text-base-content hover:bg-base-200"
                  onClick={() => showPanel("delete-group")}
                >
                  Delete / Disband
                </a>
              </li>
            </ul>
          </div>
          <div className="flex-1 overflow-y-auto">
            {currentPanel === "none" && (
              <div className="flex h-full items-center justify-center">
                <p className="text-base-content/40">
                  Select an option from the left menu
                </p>
              </div>
            )}

            {isUserPanel && (
              <div className="p-4">
                <div className="overflow-x-auto">
                  <table className="table-sm table">
                    <thead>
                      <tr className="text-base-content">
                        <th>
                          <input
                            type="checkbox"
                            className="checkbox checkbox-xs"
                            checked={allUsersChecked}
                            onChange={(e) => toggleAllUsers(e.target.checked)}
                          />
                        </th>
                        <th>UUID</th>
                        <th>Nickname</th>
                        <th>Phone</th>
                        <th>Admin</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {userList.map((user) => (
                        <tr key={user.uuid} className="hover:bg-base-200">
                          <td>
                            <input
                              type="checkbox"
                              className="checkbox checkbox-xs"
                              checked={selectedUserIds.includes(user.uuid)}
                              onChange={(e) =>
                                toggleUser(user.uuid, e.target.checked)
                              }
                            />
                          </td>
                          <td className="text-xs">{user.uuid}</td>
                          <td>{user.nickname}</td>
                          <td>{user.telephone}</td>
                          <td>
                            {user.is_admin === "1" ? (
                              <span className="badge badge-sm badge-success">
                                Yes
                              </span>
                            ) : (
                              <span className="badge badge-sm">No</span>
                            )}
                          </td>
                          <td>
                            {user.status === "1" ? (
                              <span className="badge badge-sm badge-error">
                                Disabled
                              </span>
                            ) : (
                              <span className="badge badge-sm badge-success">
                                Active
                              </span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="mt-4 flex justify-end gap-2">
                  {currentPanel === "disable-user" && (
                    <>
                      <button
                        className="btn btn-sm btn-accent font-normal"
                        onClick={enableSelectedUsers}
                      >
                        Enable
                      </button>
                      <button
                        className="btn btn-sm btn-error font-normal"
                        onClick={disableSelectedUsers}
                      >
                        Disable
                      </button>
                    </>
                  )}
                  {currentPanel === "delete-user" && (
                    <button
                      className="btn btn-sm btn-error font-normal"
                      onClick={deleteSelectedUsers}
                    >
                      Delete
                    </button>
                  )}
                  {currentPanel === "set-admin" && (
                    <>
                      <button
                        className="btn btn-sm btn-accent font-normal"
                        onClick={() => setAdminSelected(1)}
                      >
                        Grant Admin
                      </button>
                      <button
                        className="btn btn-sm btn-ghost text-base-content/60 font-normal"
                        onClick={() => setAdminSelected(0)}
                      >
                        Revoke Admin
                      </button>
                    </>
                  )}
                </div>
              </div>
            )}

            {isGroupPanel && (
              <div className="p-4">
                <div className="overflow-x-auto">
                  <table className="table-sm table">
                    <thead>
                      <tr className="text-base-content">
                        <th>
                          <input
                            type="checkbox"
                            className="checkbox checkbox-xs"
                            checked={allGroupsChecked}
                            onChange={(e) => toggleAllGroups(e.target.checked)}
                          />
                        </th>
                        <th>Group ID</th>
                        <th>Name</th>
                        <th>Owner</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {groupList.map((group) => (
                        <tr key={group.group_id} className="hover:bg-base-200">
                          <td>
                            <input
                              type="checkbox"
                              className="checkbox checkbox-xs"
                              checked={selectedGroupIds.includes(
                                group.group_id,
                              )}
                              onChange={(e) =>
                                toggleGroup(group.group_id, e.target.checked)
                              }
                            />
                          </td>
                          <td className="text-xs">{group.group_id}</td>
                          <td>{group.name}</td>
                          <td className="text-xs">{group.owner_id}</td>
                          <td>
                            {group.status === "1" ? (
                              <span className="badge badge-sm badge-error">
                                Disabled
                              </span>
                            ) : (
                              <span className="badge badge-sm badge-success">
                                Active
                              </span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="mt-4 flex justify-end gap-2">
                  {currentPanel === "disable-group" && (
                    <>
                      <button
                        className="btn btn-sm btn-accent font-normal"
                        onClick={enableSelectedGroups}
                      >
                        Enable
                      </button>
                      <button
                        className="btn btn-sm btn-error font-normal"
                        onClick={disableSelectedGroups}
                      >
                        Disable
                      </button>
                    </>
                  )}
                  {currentPanel === "delete-group" && (
                    <button
                      className="btn btn-sm btn-error font-normal"
                      onClick={deleteSelectedGroups}
                    >
                      Delete
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
