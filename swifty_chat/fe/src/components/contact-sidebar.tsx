import { useEffect, useRef, useState } from "react";
import { ChevronDown, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { api } from "@/service/api";
import useAuthStore from "@/store/auth";
import { showToast } from "@/utils/toast";

type ContactEntry = Record<string, string>;
type GroupEntry = Record<string, string>;
type RequestEntry = Record<string, string>;

interface ContactSidebarProps {
  onNavigate: (contactId: string) => void;
}

export function ContactSidebar({ onNavigate }: ContactSidebarProps) {
  const [friendList, setFriendList] = useState<ContactEntry[]>([]);
  const [myGroupList, setMyGroupList] = useState<GroupEntry[]>([]);
  const [joinedGroupList, setJoinedGroupList] = useState<GroupEntry[]>([]);
  const [requestList, setRequestList] = useState<RequestEntry[]>([]);

  const [friendsOpen, setFriendsOpen] = useState(true);
  const [myGroupsOpen, setMyGroupsOpen] = useState(false);
  const [joinedGroupsOpen, setJoinedGroupsOpen] = useState(false);

  const [applyOpen, setApplyOpen] = useState(false);
  const [createGroupOpen, setCreateGroupOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);

  const [applyId, setApplyId] = useState("");
  const [applyMsg, setApplyMsg] = useState("");
  const [groupName, setGroupName] = useState("");

  const friendsLoaded = useRef(false);
  const myGroupsLoaded = useRef(false);
  const joinedGroupsLoaded = useRef(false);

  const loadFriends = async () => {
    if (friendsLoaded.current) return;
    friendsLoaded.current = true;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getUserList({ owner_id: uid });
    if (res.code === 200 && res.data) {
      setFriendList((res.data as ContactEntry[]) || []);
    }
  };

  const loadMyGroups = async () => {
    if (myGroupsLoaded.current) return;
    myGroupsLoaded.current = true;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.loadMyGroup({ owner_id: uid });
    if (res.code === 200 && res.data) {
      setMyGroupList((res.data as GroupEntry[]) || []);
    }
  };

  const loadJoinedGroups = async () => {
    if (joinedGroupsLoaded.current) return;
    joinedGroupsLoaded.current = true;
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.loadMyJoinedGroup({ owner_id: uid });
    if (res.code === 200 && res.data) {
      setJoinedGroupList((res.data as GroupEntry[]) || []);
    }
  };

  useEffect(() => {
    loadFriends();
  }, []);

  const tryOpenChat = async (contactId: string) => {
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.checkOpenSessionAllowed({
      send_id: uid,
      receive_id: contactId,
    });
    if (res.code === 200 && res.data === true) {
      onNavigate(contactId);
    } else {
      showToast((res.message as string) || "Cannot open session", "warning");
    }
  };

  const unblockUser = async (contactId: string) => {
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
  };

  const showApplyModal = () => {
    setApplyId("");
    setApplyMsg("");
    setApplyOpen(true);
  };

  const submitApply = async () => {
    if (!applyId) {
      showToast("Please enter an ID", "error");
      return;
    }
    const uid = useAuthStore.getState().userInfo.uuid;
    const isGroup = applyId.startsWith("G");

    if (isGroup) {
      const modeRes = await api.checkGroupAddMode({ group_id: applyId });
      if (modeRes.code === 200 && modeRes.data === 0) {
        const res = await api.enterGroupDirectly({
          user_id: uid,
          group_id: applyId,
        });
        if (res.code === 200) {
          showToast("Joined group", "success");
          setApplyOpen(false);
        } else {
          showToast(res.message as string, "error");
        }
      } else {
        const res = await api.applyContact({
          user_id: uid,
          contact_id: applyId,
          contact_type: 1,
          message: applyMsg,
        });
        if (res.code === 200) {
          showToast("Application sent", "success");
          setApplyOpen(false);
        } else {
          showToast(res.message as string, "error");
        }
      }
    } else {
      const res = await api.applyContact({
        user_id: uid,
        contact_id: applyId,
        contact_type: 0,
        message: applyMsg,
      });
      if (res.code === 200) {
        showToast("Application sent", "success");
        setApplyOpen(false);
      } else {
        showToast(res.message as string, "error");
      }
    }
  };

  const showCreateGroupModal = () => {
    setGroupName("");
    setCreateGroupOpen(true);
  };

  const submitCreateGroup = async () => {
    if (!groupName) {
      showToast("Please enter a group name", "error");
      return;
    }
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.createGroup({
      name: groupName,
      owner_id: uid,
      avatar: "",
    });
    if (res.code === 200) {
      showToast("Group created", "success");
      setCreateGroupOpen(false);
    } else {
      showToast(res.message as string, "error");
    }
  };

  const showNewContactModal = async () => {
    const uid = useAuthStore.getState().userInfo.uuid;
    const res = await api.getNewContactList({ user_id: uid });
    const list = (res.data as RequestEntry[] | null) || [];
    if (list.length === 0) {
      showToast("No pending friend requests", "info");
      return;
    }
    setRequestList(list);
    setRequestsOpen(true);
  };

  const removeRequest = (id: string) => {
    setRequestList((prev) => prev.filter((r) => r.apply_id !== id));
  };

  const approveRequest = async (id: string) => {
    const res = await api.passContactApply({ apply_id: id });
    if (res.code === 200) {
      showToast("Approved", "success");
      removeRequest(id);
    } else {
      showToast(res.message as string, "error");
    }
  };

  const refuseRequest = async (id: string) => {
    const res = await api.refuseContactApply({ apply_id: id });
    if (res.code === 200) {
      showToast("Refused", "success");
      removeRequest(id);
    } else {
      showToast(res.message as string, "error");
    }
  };

  const blockRequest = async (id: string) => {
    const res = await api.blackApply({ apply_id: id });
    if (res.code === 200) {
      showToast("Blocked", "success");
      removeRequest(id);
    } else {
      showToast(res.message as string, "error");
    }
  };

  const sectionTrigger = (title: string, open: boolean, count: number) => (
    <CollapsibleTrigger className="border-border bg-muted/30 hover:bg-accent/50 flex w-full items-center justify-between border-b px-3 py-2.5 text-sm font-medium transition-colors">
      <span>
        {title}
        {count > 0 && (
          <span className="text-muted-foreground ml-2 text-xs font-normal">
            {count}
          </span>
        )}
      </span>
      <ChevronDown
        className={`text-muted-foreground size-4 transition-transform duration-200 ${open ? "rotate-180" : ""}`}
      />
    </CollapsibleTrigger>
  );

  return (
    <div className="flex h-full w-full flex-col">
      <div className="flex items-center gap-1 p-2">
        <Input
          type="text"
          className="h-8 flex-1 text-sm"
          placeholder="Search contacts"
        />
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="outline"
                size="icon"
                className="size-8 shrink-0 rounded-md"
                aria-label="Add contact or group"
              />
            }
          >
            <Plus className="size-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuItem className="text-sm" onClick={showApplyModal}>
              Add Contact / Group
            </DropdownMenuItem>
            <DropdownMenuItem
              className="text-sm"
              onClick={showCreateGroupModal}
            >
              Create Group
            </DropdownMenuItem>
            <DropdownMenuItem className="text-sm" onClick={showNewContactModal}>
              Friend Requests
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="flex-1 overflow-y-auto">
        <Collapsible open={friendsOpen} onOpenChange={setFriendsOpen}>
          {sectionTrigger("Friends", friendsOpen, friendList.length)}
          <CollapsibleContent>
            {friendList.map((user) => (
              <div
                key={user.user_id}
                className="group hover:bg-accent/60 flex cursor-pointer items-center justify-between px-3 py-2 transition-colors duration-150"
              >
                <span
                  className="flex-1 truncate text-sm"
                  onClick={() => tryOpenChat(user.user_id)}
                >
                  {user.nickname}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-muted-foreground h-6 px-2 text-xs opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={() => unblockUser(user.user_id)}
                >
                  Unblock
                </Button>
              </div>
            ))}
          </CollapsibleContent>
        </Collapsible>

        <Collapsible
          open={myGroupsOpen}
          onOpenChange={(open) => {
            setMyGroupsOpen(open);
            if (open) loadMyGroups();
          }}
        >
          {sectionTrigger("My Groups", myGroupsOpen, myGroupList.length)}
          <CollapsibleContent>
            {myGroupList.map((group) => (
              <div
                key={group.group_id}
                className="hover:bg-accent/60 flex cursor-pointer items-center gap-2 px-3 py-2 transition-colors duration-150"
                onClick={() => tryOpenChat(group.group_id)}
              >
                <span className="truncate text-sm">{group.name}</span>
              </div>
            ))}
          </CollapsibleContent>
        </Collapsible>

        <Collapsible
          open={joinedGroupsOpen}
          onOpenChange={(open) => {
            setJoinedGroupsOpen(open);
            if (open) loadJoinedGroups();
          }}
        >
          {sectionTrigger(
            "Joined Groups",
            joinedGroupsOpen,
            joinedGroupList.length,
          )}
          <CollapsibleContent>
            {joinedGroupList.map((group) => (
              <div
                key={group.group_id}
                className="hover:bg-accent/60 flex cursor-pointer items-center gap-2 px-3 py-2 transition-colors duration-150"
                onClick={() => tryOpenChat(group.group_id)}
              >
                <span className="truncate text-sm">{group.name}</span>
              </div>
            ))}
          </CollapsibleContent>
        </Collapsible>
      </div>

      {/* Apply Contact / Group Dialog */}
      <Dialog open={applyOpen} onOpenChange={setApplyOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Add Contact / Group</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="apply-id">User / Group ID</Label>
              <Input
                id="apply-id"
                type="text"
                placeholder="Enter ID"
                value={applyId}
                onChange={(e) => setApplyId(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="apply-msg">Message</Label>
              <Textarea
                id="apply-msg"
                rows={2}
                placeholder="Optional"
                maxLength={100}
                value={applyMsg}
                onChange={(e) => setApplyMsg(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button size="sm" onClick={submitApply}>
              Submit
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setApplyOpen(false)}
            >
              Cancel
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Group Dialog */}
      <Dialog open={createGroupOpen} onOpenChange={setCreateGroupOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Create Group</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="group-name">Group Name</Label>
              <Input
                id="group-name"
                type="text"
                placeholder="Required"
                value={groupName}
                onChange={(e) => setGroupName(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button size="sm" onClick={submitCreateGroup}>
              Create
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setCreateGroupOpen(false)}
            >
              Cancel
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Friend Requests Dialog */}
      <Dialog open={requestsOpen} onOpenChange={setRequestsOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Friend Requests</DialogTitle>
          </DialogHeader>
          {requestList.length === 0 ? (
            <p className="text-muted-foreground py-4 text-center text-sm">
              No pending requests
            </p>
          ) : (
            <div className="flex max-h-60 flex-col gap-2 overflow-y-auto">
              {requestList.map((req) => (
                <div
                  key={req.apply_id}
                  className="border-border flex items-center justify-between border-b py-2"
                >
                  <div className="flex items-center gap-2">
                    <span className="text-sm">{req.contact_name}</span>
                    {req.message && (
                      <span className="text-muted-foreground text-xs">
                        ({req.message})
                      </span>
                    )}
                  </div>
                  <div className="flex gap-1">
                    <Button
                      size="sm"
                      className="h-7 px-2 text-xs"
                      onClick={() => approveRequest(req.apply_id)}
                    >
                      Approve
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-muted-foreground h-7 px-2 text-xs"
                      onClick={() => refuseRequest(req.apply_id)}
                    >
                      Refuse
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive h-7 px-2 text-xs"
                      onClick={() => blockRequest(req.apply_id)}
                    >
                      Block
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
          <DialogFooter>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setRequestsOpen(false)}
            >
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
