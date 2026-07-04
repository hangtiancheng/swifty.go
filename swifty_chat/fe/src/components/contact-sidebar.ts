import { LitElement, html, css } from "lit";
import { customElement, query, state } from "lit/decorators.js";
import { api } from "../service/api";
import useAuthStore from "../store/auth";
import { showToast } from "../utils/toast";

type ContactEntry = Record<string, string>;
type GroupEntry = Record<string, string>;
type RequestEntry = Record<string, string>;

@customElement("contact-sidebar")
export class ContactSidebar extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
      height: 100%;
    }
  `;

  @state() private friendList: ContactEntry[] = [];
  @state() private myGroupList: GroupEntry[] = [];
  @state() private joinedGroupList: GroupEntry[] = [];
  @state() private requestList: RequestEntry[] = [];

  // Modal input buffers (reactive so future conditional rendering works)
  @state() private applyId = "";
  @state() private applyMsg = "";
  @state() private groupName = "";

  // Modal element refs — scoped query via data attribute, no global id collision
  @query("dialog[data-modal='apply']")
  private _applyModal!: HTMLDialogElement;
  @query("dialog[data-modal='create-group']")
  private _createGroupModal!: HTMLDialogElement;
  @query("dialog[data-modal='friend-requests']")
  private _friendRequestsModal!: HTMLDialogElement;

  private async toggleFriends(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (!checked) return;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getUserList({ owner_id: uid });
    if (res.code === 200 && res.data) {
      this.friendList = (res.data as ContactEntry[]) || [];
    }
  }

  private async toggleMyGroups(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (!checked) return;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.loadMyGroup({ owner_id: uid });
    if (res.code === 200 && res.data) {
      this.myGroupList = (res.data as GroupEntry[]) || [];
    }
  }

  private async toggleJoinedGroups(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (!checked) return;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.loadMyJoinedGroup({ owner_id: uid });
    if (res.code === 200 && res.data) {
      this.joinedGroupList = (res.data as GroupEntry[]) || [];
    }
  }

  private async tryOpenChat(contactId: string) {
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.checkOpenSessionAllowed({
      send_id: uid,
      receive_id: contactId,
    });
    if (res.code === 200 && res.data === true) {
      this.dispatchEvent(new CustomEvent("navigate", { detail: contactId }));
    } else {
      showToast((res.message as string) || "Cannot open session", "warning");
    }
  }

  private showApplyModal() {
    this.applyId = "";
    this.applyMsg = "";
    this._applyModal?.showModal();
  }

  private closeApplyModal() {
    this._applyModal?.close();
  }

  private onApplyIdInput(e: Event) {
    this.applyId = (e.target as HTMLInputElement).value;
  }

  private onApplyMsgInput(e: Event) {
    this.applyMsg = (e.target as HTMLTextAreaElement).value;
  }

  private async submitApply() {
    if (!this.applyId) {
      showToast("Please enter an ID", "error");
      return;
    }
    const uid = useAuthStore.getState().userInfo.uuid;
    const isGroup = this.applyId.startsWith("G");

    if (isGroup) {
      const modeRes = await api.checkGroupAddMode({ group_id: this.applyId });
      if (modeRes.code === 200 && modeRes.data === 0) {
        const res = await api.enterGroupDirectly({
          user_id: uid,
          group_id: this.applyId,
        });
        if (res.code === 200) {
          showToast("Joined group", "success");
          this.closeApplyModal();
        } else {
          showToast(res.message as string, "error");
        }
      } else {
        const res = await api.applyContact({
          user_id: uid,
          contact_id: this.applyId,
          contact_type: 1,
          message: this.applyMsg,
        });
        if (res.code === 200) {
          showToast("Application sent", "success");
          this.closeApplyModal();
        } else {
          showToast(res.message as string, "error");
        }
      }
    } else {
      const res = await api.applyContact({
        user_id: uid,
        contact_id: this.applyId,
        contact_type: 0,
        message: this.applyMsg,
      });
      if (res.code === 200) {
        showToast("Application sent", "success");
        this.closeApplyModal();
      } else {
        showToast(res.message as string, "error");
      }
    }
  }

  private showCreateGroupModal() {
    this.groupName = "";
    this._createGroupModal?.showModal();
  }

  private closeCreateGroupModal() {
    this._createGroupModal?.close();
  }

  private onGroupNameInput(e: Event) {
    this.groupName = (e.target as HTMLInputElement).value;
  }

  private async submitCreateGroup() {
    if (!this.groupName) {
      showToast("Please enter a group name", "error");
      return;
    }
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.createGroup({
      name: this.groupName,
      owner_id: uid,
      avatar: "",
    });
    if (res.code === 200) {
      showToast("Group created", "success");
      this.closeCreateGroupModal();
    } else {
      showToast(res.message as string, "error");
    }
  }

  private async showNewContactModal() {
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getNewContactList({ user_id: uid });
    const list = (res.data as RequestEntry[] | null) || [];
    if (list.length === 0) {
      showToast("No pending friend requests", "info");
      return;
    }
    this.requestList = list;
    this._friendRequestsModal?.showModal();
  }

  private closeRequestsModal() {
    this._friendRequestsModal?.close();
  }

  private removeRequest(id: string) {
    this.requestList = this.requestList.filter((r) => r.apply_id !== id);
  }

  private async approveRequest(id: string) {
    const res = await api.passContactApply({ apply_id: id });
    if (res.code === 200) {
      showToast("Approved", "success");
      this.removeRequest(id);
    } else {
      showToast(res.message as string, "error");
    }
  }

  private async refuseRequest(id: string) {
    const res = await api.refuseContactApply({ apply_id: id });
    if (res.code === 200) {
      showToast("Refused", "success");
      this.removeRequest(id);
    } else {
      showToast(res.message as string, "error");
    }
  }

  private async blockRequest(id: string) {
    const res = await api.blackApply({ apply_id: id });
    if (res.code === 200) {
      showToast("Blocked", "success");
      this.removeRequest(id);
    } else {
      showToast(res.message as string, "error");
    }
  }

  private async unblockUser(contactId: string) {
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.cancelBlackContact({
      user_id: uid,
      contact_id: contactId,
    });
    if (res.code === 200) {
      showToast("Contact unblocked", "success");
    } else {
      showToast((res.message as string) || "Failed to unblock", "error");
    }
  }

  override render() {
    return html`
      <div class="flex h-full w-full flex-col">
        <div class="flex gap-1 p-2">
          <input
            type="text"
            class="input input-bordered input-sm flex-1"
            placeholder="Search contacts"
          />
          <div class="dropdown dropdown-end">
            <div
              tabindex="0"
              role="button"
              class="btn btn-soft btn-accent btn-sm btn-square font-normal"
            >
              +
            </div>
            <ul
              tabindex="0"
              class="dropdown-content menu rounded-box border-base-300 bg-base-100 z-10 w-44 border p-2 shadow-lg"
            >
              <li>
                <a
                  class="text-base-content hover:bg-base-200 text-sm"
                  @click=${this.showApplyModal}
                  >Add Contact / Group</a
                >
              </li>
              <li>
                <a
                  class="text-base-content hover:bg-base-200 text-sm"
                  @click=${this.showCreateGroupModal}
                  >Create Group</a
                >
              </li>
              <li>
                <a
                  class="text-base-content hover:bg-base-200 text-sm"
                  @click=${this.showNewContactModal}
                  >Friend Requests</a
                >
              </li>
            </ul>
          </div>
        </div>
        <div class="flex-1 overflow-y-auto">
          <div
            class="collapse-arrow border-base-300 bg-base-200/50 collapse rounded-none border-b"
          >
            <input type="checkbox" @change=${this.toggleFriends} />
            <div class="collapse-title text-base-content text-sm font-medium">
              Friends
            </div>
            <div class="collapse-content p-0">
              ${this.friendList.map(
                (user) => html`
                  <div
                    class="group hover:bg-base-200 flex cursor-pointer items-center justify-between px-3 py-2 transition-colors"
                  >
                    <span
                      class="text-base-content flex-1 truncate text-sm"
                      @click=${() => this.tryOpenChat(user.user_id)}
                      >${user.nickname}</span
                    >
                    <button
                      class="btn btn-ghost btn-xs text-base-content/40 font-normal opacity-0 transition-opacity group-hover:opacity-100"
                      @click=${() => this.unblockUser(user.user_id)}
                    >
                      Unblock
                    </button>
                  </div>
                `,
              )}
            </div>
          </div>
          <div
            class="collapse-arrow border-base-300 bg-base-200/50 collapse rounded-none border-b"
          >
            <input type="checkbox" @change=${this.toggleMyGroups} />
            <div class="collapse-title text-base-content text-sm font-medium">
              My Groups
            </div>
            <div class="collapse-content p-0">
              ${this.myGroupList.map(
                (group) => html`
                  <div
                    class="hover:bg-base-200 flex cursor-pointer items-center gap-2 px-3 py-2 transition-colors"
                    @click=${() => this.tryOpenChat(group.group_id)}
                  >
                    <span class="text-base-content truncate text-sm"
                      >${group.name}</span
                    >
                  </div>
                `,
              )}
            </div>
          </div>
          <div
            class="collapse-arrow border-base-300 bg-base-200/50 collapse rounded-none border-b"
          >
            <input type="checkbox" @change=${this.toggleJoinedGroups} />
            <div class="collapse-title text-base-content text-sm font-medium">
              Joined Groups
            </div>
            <div class="collapse-content p-0">
              ${this.joinedGroupList.map(
                (group) => html`
                  <div
                    class="hover:bg-base-200 flex cursor-pointer items-center gap-2 px-3 py-2 transition-colors"
                    @click=${() => this.tryOpenChat(group.group_id)}
                  >
                    <span class="text-base-content truncate text-sm"
                      >${group.name}</span
                    >
                  </div>
                `,
              )}
            </div>
          </div>
        </div>

        <!-- Apply Contact / Group Modal -->
        <dialog class="modal" data-modal="apply">
          <div class="modal-box border-base-300 rounded-box w-80 border">
            <h3 class="text-base-content mb-4 text-base font-semibold">
              Add Contact / Group
            </h3>
            <fieldset class="fieldset space-y-3">
              <label class="label text-base-content/70 text-sm"
                >User / Group ID</label
              >
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="Enter ID"
                @input=${this.onApplyIdInput}
              />
              <label class="label text-base-content/70 text-sm">Message</label>
              <textarea
                class="textarea textarea-bordered textarea-sm w-full"
                rows="2"
                placeholder="Optional"
                maxlength="100"
                @input=${this.onApplyMsgInput}
              ></textarea>
            </fieldset>
            <div class="modal-action">
              <button
                class="btn btn-sm btn-accent font-normal"
                @click=${this.submitApply}
              >
                Submit
              </button>
              <button
                class="btn btn-sm btn-ghost font-normal"
                @click=${this.closeApplyModal}
              >
                Cancel
              </button>
            </div>
          </div>
          <form method="dialog" class="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>

        <!-- Create Group Modal -->
        <dialog class="modal" data-modal="create-group">
          <div class="modal-box border-base-300 rounded-box w-80 border">
            <h3 class="text-base-content mb-4 text-base font-semibold">
              Create Group
            </h3>
            <fieldset class="fieldset space-y-3">
              <label class="label text-base-content/70 text-sm"
                >Group Name</label
              >
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="Required"
                @input=${this.onGroupNameInput}
              />
            </fieldset>
            <div class="modal-action">
              <button
                class="btn btn-sm btn-accent font-normal"
                @click=${this.submitCreateGroup}
              >
                Create
              </button>
              <button
                class="btn btn-sm btn-ghost font-normal"
                @click=${this.closeCreateGroupModal}
              >
                Cancel
              </button>
            </div>
          </div>
          <form method="dialog" class="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>

        <!-- Friend Requests Modal -->
        <dialog class="modal" data-modal="friend-requests">
          <div class="modal-box border-base-300 rounded-box w-96 border">
            <h3 class="text-base-content mb-4 text-base font-semibold">
              Friend Requests
            </h3>
            ${
              this.requestList.length === 0
                ? html`<p class="text-base-content/40 py-4 text-center text-sm">
                    No pending requests
                  </p>`
                : html`<div class="max-h-60 space-y-2 overflow-y-auto">
                    ${this.requestList.map(
                      (req) => html`
                        <div
                          class="border-base-200 flex items-center justify-between border-b py-2"
                        >
                          <div class="flex items-center gap-2">
                            <span class="text-base-content text-sm"
                              >${req.contact_name}</span
                            >
                            ${
                              req.message
                                ? html`<span
                                    class="text-base-content/40 text-xs"
                                    >(${req.message})</span
                                  >`
                                : null
                            }
                          </div>
                          <div class="flex gap-1">
                            <button
                              class="btn btn-xs btn-accent font-normal"
                              @click=${() => this.approveRequest(req.apply_id)}
                            >
                              Approve
                            </button>
                            <button
                              class="btn btn-xs btn-ghost text-base-content/60 font-normal"
                              @click=${() => this.refuseRequest(req.apply_id)}
                            >
                              Refuse
                            </button>
                            <button
                              class="btn btn-xs btn-ghost text-error font-normal"
                              @click=${() => this.blockRequest(req.apply_id)}
                            >
                              Block
                            </button>
                          </div>
                        </div>
                      `,
                    )}
                  </div>`
            }
            <div class="modal-action">
              <button
                class="btn btn-sm btn-ghost font-normal"
                @click=${this.closeRequestsModal}
              >
                Close
              </button>
            </div>
          </div>
          <form method="dialog" class="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "contact-sidebar": ContactSidebar;
  }
}
