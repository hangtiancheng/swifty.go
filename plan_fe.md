# KamaChat 前端迁移方案: Vue3 -> Lark MVC + TailwindCSS + DaisyUI

## 1. 源项目分析

### 1.1 技术栈

- Vue 3 + Vue Router 4 + Vuex 4
- Element Plus 组件库
- Axios HTTP 客户端
- WebSocket 实时通信
- WebRTC 音视频通话

### 1.2 页面清单

| 路由              | 视图            | 功能                               |
| ----------------- | --------------- | ---------------------------------- |
| /login            | Login.vue       | 账号密码登录                       |
| /smsLogin         | SmsLogin.vue    | 短信验证码登录                     |
| /register         | Register.vue    | 注册(昵称+手机+密码+验证码)        |
| /chat/sessionList | SessionList.vue | 会话列表(用户会话/群聊会话)        |
| /chat/contactlist | ContactList.vue | 联系人列表(好友/我的群/已加入的群) |
| /chat/:id         | ContactChat.vue | 聊天窗口(消息收发/文件/WebRTC)     |
| /chat/owninfo     | OwnInfo.vue     | 个人信息编辑                       |
| /manager          | Manager.vue     | 管理后台(用户/群聊管理)            |

### 1.3 共享组件

- NavigationModal: 左侧导航栏(头像/会话/联系人/个人信息/管理/退出)
- ContactListModal: 联系人列表面板
- Modal / SmallModal: 弹窗容器
- DisableUserModal / DeleteUserModal / SetAdminModal: 管理后台子面板
- DeleteGroupModal / DisableGroupModal: 群聊管理子面板

### 1.4 全局状态 (Vuex)

- backendUrl: 后端服务地址
- wsUrl: WebSocket 服务地址
- userInfo: 当前登录用户信息(sessionStorage 持久化)
- socket: WebSocket 实例

### 1.5 后端 API 清单 (使用 const 或 enum 统一维护, 方便后续更新)

- POST /login -- 密码登录
- POST /register -- 注册
- POST /user/smsLogin -- 验证码登录
- POST /user/sendSmsCode -- 发送验证码
- POST /user/getUserInfo -- 获取用户信息
- POST /user/updateUserInfo -- 更新用户信息
- POST /user/wsLogout -- WebSocket 注销
- POST /contact/getContactInfo -- 获取联系人信息
- POST /contact/getUserList -- 获取好友列表
- POST /contact/applyContact -- 申请添加联系人
- POST /contact/passContactApply -- 同意申请
- POST /contact/refuseContactApply -- 拒绝申请
- POST /contact/getNewContactList -- 获取新好友申请列表
- POST /contact/getAddGroupList -- 获取入群申请列表
- POST /contact/deleteContact -- 删除联系人
- POST /contact/blackContact -- 拉黑联系人
- POST /contact/blackApply -- 拉黑申请
- POST /contact/cancelBlackContact -- 取消拉黑
- POST /contact/loadMyJoinedGroup -- 加载已加入的群
- POST /session/openSession -- 打开/创建会话
- POST /session/getUserSessionList -- 用户会话列表
- POST /session/getGroupSessionList -- 群会话列表
- POST /session/deleteSession -- 删除会话
- POST /session/checkOpenSessionAllowed -- 检查是否允许开启会话
- POST /message/getMessageList -- 获取消息列表
- POST /message/getGroupMessageList -- 获取群消息列表
- POST /message/uploadFile -- 上传文件
- POST /message/uploadAvatar -- 上传头像
- POST /group/createGroup -- 创建群聊
- POST /group/loadMyGroup -- 加载我创建的群
- POST /group/getGroupMemberList -- 获取群成员列表
- POST /group/updateGroupInfo -- 更新群信息
- POST /group/removeGroupMembers -- 移除群成员
- POST /group/leaveGroup -- 退出群聊
- POST /group/dismissGroup -- 解散群聊
- POST /group/checkGroupAddMode -- 检查入群方式
- POST /group/enterGroupDirectly -- 直接入群

---

## 2. 目标架构

### 2.1 技术栈

- Lark MVC (@lark.js/mvc) + TypeScript
- Vite + larkMvcPlugin
- TailwindCSS v4 + DaisyUI 5
- Lark Service 替代 axios
- 原生 WebSocket (保持不变)

### 2.2 目录结构

```
lark_chat_fe/
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
├── package.json
└── src/
    ├── boot.ts                    # registerViewClass + Framework.boot
    ├── view.ts                    # 项目级 base view
    ├── styles.css                 # tailwind 指令 + 全局样式
    ├── config.ts                  # backendUrl / wsUrl 配置
    ├── service/
    │   ├── index.ts               # AppService (Service.extend)
    │   └── endpoints.ts           # AppService.add([...]) 全部端点注册
    ├── utils/
    │   ├── ws.ts                  # WebSocket 管理器
    │   ├── rtc.ts                 # WebRTC 管理器
    │   ├── toast.ts               # DaisyUI toast 通知
    │   └── validate.ts            # 手机号/邮箱校验
    ├── store/
    │   ├── auth.ts                # 用户认证 store (userInfo, login/logout)
    │   ├── chat.ts                # 聊天 store (messageList, contactInfo, sessionId)
    │   ├── session.ts             # 会话列表 store
    │   └── ws.ts                  # WebSocket store (socket 实例, 连接状态)
    ├── views/
    │   ├── login.ts + login.html
    │   ├── sms-login.ts + sms-login.html
    │   ├── register.ts + register.html
    │   ├── chat.ts + chat.html                  # 聊天主页(含消息区域)
    │   ├── session-list.ts + session-list.html   # 会话列表页
    │   ├── contact-list.ts + contact-list.html   # 联系人列表页
    │   ├── own-info.ts + own-info.html           # 个人信息页
    │   ├── manager.ts + manager.html             # 管理后台
    │   └── not-found.ts + not-found.html         # 404
    └── components/
        ├── nav-bar.ts + nav-bar.html             # 左侧导航栏
        ├── session-sidebar.ts + session-sidebar.html  # 会话侧边栏
        ├── contact-sidebar.ts + contact-sidebar.html  # 联系人侧边栏
        ├── message-bubble.ts + message-bubble.html    # 消息气泡
        ├── modal.ts + modal.html                 # DaisyUI modal 封装
        ├── user-profile.ts + user-profile.html   # 用户信息弹窗
        ├── group-profile.ts + group-profile.html # 群信息弹窗
        ├── video-call.ts + video-call.html       # 音视频通话弹窗
        └── admin/
            ├── user-table.ts + user-table.html   # 用户管理表格
            └── group-table.ts + group-table.html  # 群聊管理表格
```

### 2.3 路由设计

使用 Lark Router history 模式, 路由映射:

```ts
const config: FrameworkConfig = {
  rootId: "app",
  routeMode: "history",
  defaultPath: "/login",
  defaultView: "login",
  routes: {
    "/login": "login",
    "/sms-login": "sms-login",
    "/register": "register",
    "/chat/sessions": "session-list",
    "/chat/contacts": "contact-list",
    "/chat/:id": "chat",
    "/chat/profile": "own-info",
    "/manager": "manager",
  },
  unmatchedView: "not-found",
  error(e: Error) {
    console.error("Lark error:", e);
  },
};
```

路由守卫:

```ts
Router.beforeEach(async (to) => {
  const { userInfo } = useAuthStore.getState();
  const publicPaths = ["/login", "/sms-login", "/register"];
  if (!userInfo.uuid && !publicPaths.includes(to.path)) {
    Router.to("/login", {}, true);
    return false;
  }
  return true;
});
```

### 2.4 状态管理设计

用 Lark 的 zustand-aligned `create()` 替换 Vuex:

auth store:

```ts
interface AuthStore {
  userInfo: UserInfo;
  backendUrl: string;
  wsUrl: string;
  isLoggedIn: boolean; // computed
  setUserInfo: (info: UserInfo) => void;
  clearUserInfo: () => void;
}
```

ws store:

```ts
interface WsStore {
  socket: WebSocket | null;
  status: "disconnected" | "connecting" | "connected";
  connect: (uuid: string) => void;
  disconnect: () => void;
  send: (data: unknown) => void;
}
```

chat store:

```ts
interface ChatStore {
  contactInfo: ContactInfo | null;
  sessionId: string;
  messageList: Message[];
  setContact: (info: ContactInfo) => void;
  setSessionId: (id: string) => void;
  addMessage: (msg: Message) => void;
  setMessageList: (list: Message[]) => void;
  clearChat: () => void;
}
```

session store:

```ts
interface SessionStore {
  userSessions: SessionItem[];
  groupSessions: SessionItem[];
  loadUserSessions: () => Promise<void>;
  loadGroupSessions: () => Promise<void>;
}
```

### 2.5 API 层设计

使用 Lark Service 统一管理所有后端请求, 利用其 LFU 缓存和请求去重能力:

```ts
// src/service/index.ts
import { Service } from "@lark.js/mvc";
import { BASE_URL } from "../config";

const AppService = Service.extend(
  (payload, callback) => {
    const url = payload.get<string>("url");
    const method = payload.get<string>("method") || "POST";
    const data = payload.get("data");
    const isFormData = data instanceof FormData;

    fetch(url, {
      method,
      headers: isFormData ? undefined : { "Content-Type": "application/json" },
      body: isFormData ? data : data ? JSON.stringify(data) : undefined,
    })
      .then((r) => r.json())
      .then((json) => {
        payload.set(json);
        callback();
      })
      .catch((err) => {
        payload.set({ error: err.message });
        callback();
      });
  },
  30, // cacheMax
  5,  // cacheBuffer
);

export default AppService;
```

按业务域注册端点 (src/service/endpoints.ts):

```ts
import AppService from "./index";
import { BASE_URL } from "../config";

AppService.add([
  // -- 认证 --
  { name: "login", url: BASE_URL + "/login" },
  { name: "register", url: BASE_URL + "/register" },
  { name: "smsLogin", url: BASE_URL + "/user/smsLogin" },
  { name: "sendSmsCode", url: BASE_URL + "/user/sendSmsCode" },
  { name: "getUserInfo", url: BASE_URL + "/user/getUserInfo", cache: 30_000 },
  { name: "updateUserInfo", url: BASE_URL + "/user/updateUserInfo" },
  { name: "wsLogout", url: BASE_URL + "/user/wsLogout" },

  // -- 联系人 --
  { name: "getContactInfo", url: BASE_URL + "/contact/getContactInfo", cache: 15_000 },
  { name: "getUserList", url: BASE_URL + "/contact/getUserList" },
  { name: "applyContact", url: BASE_URL + "/contact/applyContact" },
  { name: "passContactApply", url: BASE_URL + "/contact/passContactApply" },
  { name: "refuseContactApply", url: BASE_URL + "/contact/refuseContactApply" },
  { name: "getNewContactList", url: BASE_URL + "/contact/getNewContactList" },
  { name: "getAddGroupList", url: BASE_URL + "/contact/getAddGroupList" },
  { name: "deleteContact", url: BASE_URL + "/contact/deleteContact", cleanKeys: "getUserList" },
  { name: "blackContact", url: BASE_URL + "/contact/blackContact" },
  { name: "blackApply", url: BASE_URL + "/contact/blackApply" },
  { name: "cancelBlackContact", url: BASE_URL + "/contact/cancelBlackContact" },
  { name: "loadMyJoinedGroup", url: BASE_URL + "/contact/loadMyJoinedGroup" },

  // -- 会话 --
  { name: "openSession", url: BASE_URL + "/session/openSession" },
  { name: "getUserSessionList", url: BASE_URL + "/session/getUserSessionList" },
  { name: "getGroupSessionList", url: BASE_URL + "/session/getGroupSessionList" },
  { name: "deleteSession", url: BASE_URL + "/session/deleteSession", cleanKeys: "getUserSessionList,getGroupSessionList" },
  { name: "checkOpenSessionAllowed", url: BASE_URL + "/session/checkOpenSessionAllowed" },

  // -- 消息 --
  { name: "getMessageList", url: BASE_URL + "/message/getMessageList" },
  { name: "getGroupMessageList", url: BASE_URL + "/message/getGroupMessageList" },
  { name: "uploadFile", url: BASE_URL + "/message/uploadFile" },
  { name: "uploadAvatar", url: BASE_URL + "/message/uploadAvatar" },

  // -- 群聊 --
  { name: "createGroup", url: BASE_URL + "/group/createGroup" },
  { name: "loadMyGroup", url: BASE_URL + "/group/loadMyGroup" },
  { name: "getGroupMemberList", url: BASE_URL + "/group/getGroupMemberList" },
  { name: "updateGroupInfo", url: BASE_URL + "/group/updateGroupInfo" },
  { name: "removeGroupMembers", url: BASE_URL + "/group/removeGroupMembers" },
  { name: "leaveGroup", url: BASE_URL + "/group/leaveGroup" },
  { name: "dismissGroup", url: BASE_URL + "/group/dismissGroup" },
  { name: "checkGroupAddMode", url: BASE_URL + "/group/checkGroupAddMode" },
  { name: "enterGroupDirectly", url: BASE_URL + "/group/enterGroupDirectly" },
]);
```

在 View 中使用:

```ts
init() {
  const service = new AppService();
  this.capture("svc", service, true);
  this.svc = service;
},

loadSessions() {
  this.svc.all(
    { name: "getUserSessionList", data: { owner_id: this.userId } },
    (errors, payload) => {
      if (!errors[0]) {
        this.updater.set({ userSessions: payload.get("data") }).digest();
      }
    },
  );
},
```

写操作使用 `service.save()` 跳过缓存:

```ts
sendLogin(telephone: string, password: string) {
  this.svc.save(
    { name: "login", data: { telephone, password } },
    (errors, payload) => {
      if (!errors[0] && payload.get("code") === 200) {
        useAuthStore.getState().setUserInfo(payload.get("data"));
        Router.to("/chat/sessions");
      }
    },
  );
},
```

### 2.6 UI 组件映射 (Element Plus -> DaisyUI)

| Element Plus                          | DaisyUI 替代                                            |
| ------------------------------------- | ------------------------------------------------------- |
| el-button                             | `<button class="btn btn-primary">`                      |
| el-input                              | `<input class="input input-bordered">`                  |
| el-form / el-form-item                | `<fieldset class="fieldset">` + `<label class="label">` |
| el-menu / el-sub-menu                 | `<ul class="menu">` + collapse 手动实现                 |
| el-message / el-notification          | `<div class="toast">` + toast.ts 工具                   |
| el-dialog                             | `<dialog class="modal">`                                |
| el-dropdown                           | `<div class="dropdown">`                                |
| el-descriptions                       | 手写 table 或 card 布局                                 |
| el-upload                             | 原生 `<input type="file">` + fetch 上传                 |
| el-scrollbar                          | `<div class="overflow-y-auto">`                         |
| el-tooltip                            | `<div class="tooltip">`                                 |
| el-icon                               | 直接使用 SVG 或 DaisyUI 内置                            |
| el-image                              | `<img>` 标签                                            |
| el-container/aside/header/main/footer | tailwind flex 布局                                      |
| el-radio-group                        | `<div>` + `<input type="radio" class="radio">`          |
| el-message-box (confirm)              | `<dialog class="modal">` 确认弹窗                       |

---

## 3. 迁移步骤

### 第一步: 项目脚手架

1. 清理 lark_chat_fe 中的 React 模板代码
2. 安装依赖: `@lark.js/mvc`, `tailwindcss`, `daisyui`
3. 配置 vite.config.ts (larkMvcPlugin + @ 别名)
4. 配置 tailwind.config.ts (引入 daisyui 插件)
5. 编写 index.html (引用 /src/boot.ts)
6. 编写 src/styles.css (tailwind 指令)

### 第二步: 基础设施

1. 编写 src/config.ts (服务地址配置)
2. 编写 src/service/index.ts (AppService, 基于 Service.extend)
3. 编写 src/service/endpoints.ts (AppService.add, 注册全部后端端点)
4. 编写 src/utils/validate.ts (手机号/邮箱校验)
5. 编写 src/utils/toast.ts (DaisyUI toast 通知)
6. 编写 src/view.ts (base view, 含 navigate 方法)

### 第三步: Store 层

1. 编写 src/store/auth.ts (认证 store)
2. 编写 src/store/ws.ts (WebSocket store)
3. 编写 src/store/chat.ts (聊天 store)
4. 编写 src/store/session.ts (会话 store)

### 第四步: 认证页面

1. 编写 login view (login.ts + login.html)
2. 编写 sms-login view (sms-login.ts + sms-login.html)
3. 编写 register view (register.ts + register.html)
4. 编写 not-found view

### 第五步: 共享组件

1. 编写 nav-bar 组件 (左侧图标导航)
2. 编写 modal 组件 (DaisyUI dialog 封装)
3. 编写 session-sidebar 组件 (会话侧边栏, 用户/群聊折叠列表)
4. 编写 contact-sidebar 组件 (联系人侧边栏)
5. 编写 message-bubble 组件 (消息气泡, 区分左右/文本/文件)

### 第六步: 聊天核心页面

1. 编写 session-list view (会话列表 + 空聊天区域)
2. 编写 contact-list view (联系人列表 + 空聊天区域)
3. 编写 chat view (聊天窗口, 含消息列表/工具栏/输入区/发送)
4. 接入 WebSocket 消息收发
5. 接入文件上传/下载

### 第七步: 个人信息与群聊

1. 编写 own-info view (个人信息展示 + 编辑弹窗)
2. 编写 user-profile 组件 (查看他人信息)
3. 编写 group-profile 组件 (查看/编辑群信息)

### 第八步: 管理后台

1. 编写 manager view (管理后台框架)
2. 编写 user-table 组件 (启用/禁用/删除/设置管理员)
3. 编写 group-table 组件 (启用/禁用/删除群聊)

### 第九步: WebRTC 音视频

1. 编写 src/utils/rtc.ts (WebRTC 管理器)
2. 编写 video-call 组件 (通话 UI)
3. 接入信令交换 (通过 WebSocket)

### 第十步: boot.ts 注册与集成

1. 注册所有 view 和 component
2. 配置路由和路由守卫
3. 编写 Framework.boot 配置
4. 在 boot 阶段初始化 WebSocket (如已登录)

---

## 4. 关键技术决策

### 4.1 DaisyUI 主题

使用 DaisyUI 的 `valentine` 主题作为基础, 与源项目的粉色调风格一致:

```html
<html data-theme="valentine"></html>
```

### 4.2 WebSocket 生命周期

- 登录成功后通过 ws store 的 connect action 建立连接
- App 层(boot.ts)检测 sessionStorage 中的 userInfo, 如存在则自动重连
- socket.onmessage 统一派发到 chat store 的 addMessage

### 4.3 路由参数传递

聊天页面需要动态路由 `/chat/:id`, Lark Router 不直接支持 `:param` 占位符, 改用 query 参数:

```
路由: /chat
参数: ?id=U_xxx 或 ?id=G_xxx
```

使用 `useUrlState` 或 `observeLocation` 监听 id 参数变化, 在 assign() 中加载对应联系人和消息。

### 4.4 背景图片

源项目使用 `chat_server_background.jpg` 作为登录/聊天页背景。将该图片复制到 `src/assets/` 目录, 在模板中通过 CSS background-image 引用。

### 4.5 文件上传

源项目使用 Element Plus 的 el-upload 组件。迁移后使用原生 `<input type="file">` 触发选择, 通过 AppService.save 上传 FormData:

```ts
"$fileInput<change>"(e: Event) {
  const input = e.eventTarget as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  const formData = new FormData();
  formData.append("file", file);
  this.svc.save(
    { name: "uploadFile", data: formData },
    (errors, payload) => {
      if (!errors[0]) {
        // 发送文件消息
      }
    },
  );
},
```

AppService 的 syncFn 已处理 FormData 类型: 检测到 `data instanceof FormData` 时不设置 Content-Type header, 让浏览器自动生成 multipart boundary。

### 4.6 消息气泡的 ldk 优化

聊天消息列表可能很长。对每条消息气泡使用 `ldk` 属性标记, 避免每次 digest 时重复 diff 已有消息:

```html
{{forOf messageList as msg idx}}
<div ldk="msg-{{=msg.session_id}}-{{=idx}}">
  <!-- 消息内容 -->
</div>
{{/forOf}}
```

---

## 5. 依赖安装

```bash
cd lark_chat_fe
pnpm add @lark.js/mvc
pnpm add -D tailwindcss @tailwindcss/vite daisyui
```

清除 React 相关依赖:

```bash
pnpm remove @vitejs/plugin-react @types/react @types/react-dom
```
