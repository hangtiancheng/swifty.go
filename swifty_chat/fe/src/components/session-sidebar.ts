import { LitElement, html, css } from "lit";
import { customElement, state } from "lit/decorators.js";
import { api } from "../service/api";
import useAuthStore from "../store/auth";
import useSessionStore from "../store/session";
import { resolveAvatar } from "../utils/avatar";
import type { SessionItem } from "../types";

@customElement("session-sidebar")
export class SessionSidebar extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
      height: 100%;
    }
  `;

  @state() private userSessions: SessionItem[] = [];
  @state() private groupSessions: SessionItem[] = [];

  private async toggleUsers(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (!checked) return;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getUserSessionList({ owner_id: uid });
    if (res.code === 200 && res.data) {
      const list = ((res.data as SessionItem[]) || []).map((u) => ({
        ...u,
        avatar: resolveAvatar(u.avatar),
      }));
      useSessionStore.getState().setUserSessions(list);
      this.userSessions = list;
    }
  }

  private async toggleGroups(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (!checked) return;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getGroupSessionList({ owner_id: uid });
    if (res.code === 200 && res.data) {
      const list = ((res.data as SessionItem[]) || []).map((g) => ({
        ...g,
        avatar: resolveAvatar(g.avatar),
      }));
      useSessionStore.getState().setGroupSessions(list);
      this.groupSessions = list;
    }
  }

  private chatUser(id: string) {
    this.dispatchEvent(new CustomEvent("chat", { detail: id }));
  }

  private chatGroup(id: string) {
    this.dispatchEvent(new CustomEvent("chat", { detail: id }));
  }

  override render() {
    return html`
      <div class="flex h-full w-full flex-col">
        <div class="p-2">
          <input
            type="text"
            class="input input-bordered input-sm w-full"
            placeholder="Search sessions"
          />
        </div>
        <div class="flex-1 overflow-y-auto">
          <div
            class="collapse-arrow border-base-300 bg-base-200/50 collapse rounded-none border-b"
          >
            <input type="checkbox" @change=${this.toggleUsers} />
            <div class="collapse-title text-base-content text-sm font-medium">
              Users
            </div>
            <div class="collapse-content p-0">
              ${this.userSessions.map(
                (u) => html`
                  <div
                    class="hover:bg-base-200 flex cursor-pointer items-center gap-2 px-3 py-2 transition-colors"
                    @click=${() => this.chatUser(u.user_id!)}
                  >
                    <div class="avatar">
                      <div class="w-8 rounded-full">
                        <img src=${u.avatar} />
                      </div>
                    </div>
                    <span class="text-base-content truncate text-sm"
                      >${u.user_name}</span
                    >
                  </div>
                `,
              )}
            </div>
          </div>
          <div
            class="collapse-arrow border-base-300 bg-base-200/50 collapse rounded-none border-b"
          >
            <input type="checkbox" @change=${this.toggleGroups} />
            <div class="collapse-title text-base-content text-sm font-medium">
              Groups
            </div>
            <div class="collapse-content p-0">
              ${this.groupSessions.map(
                (g) => html`
                  <div
                    class="hover:bg-base-200 flex cursor-pointer items-center gap-2 px-3 py-2 transition-colors"
                    @click=${() => this.chatGroup(g.group_id!)}
                  >
                    <div class="avatar">
                      <div class="w-8 rounded-full">
                        <img src=${g.avatar} />
                      </div>
                    </div>
                    <span class="text-base-content truncate text-sm"
                      >${g.group_name}</span
                    >
                  </div>
                `,
              )}
            </div>
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "session-sidebar": SessionSidebar;
  }
}
