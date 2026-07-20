# daisyUI -> shadcn/ui 迁移计划

项目: swifty-chat-fe
技术栈: React 19 + Vite 8 + Tailwind CSS v4 + daisyUI 5 + Lit 3 + Zustand + react-router-dom 7
目标: 彻底移除 daisyUI 和 Lit, 全面迁移到 shadcn/ui, 采用浅粉色主题

---

## 一、现状分析

### 1.1 daisyUI 使用清单

共 16 个源文件使用了 daisyUI 类名, 涉及以下 daisyUI 组件:

| daisyUI 组件                                                          | 使用位置                                         | 对应 shadcn 组件                   |
| --------------------------------------------------------------------- | ------------------------------------------------ | ---------------------------------- |
| btn (ghost/accent/error/sm/xs/square/soft/outline)                    | 全部页面 + Lit 组件                              | Button                             |
| card / card-border                                                    | 所有页面容器                                     | Card                               |
| input / input-bordered / input-sm                                     | login, register, chat, own-info, sidebars        | Input                              |
| textarea / textarea-ghost / textarea-bordered                         | chat, own-info, contact-sidebar                  | Textarea                           |
| modal / modal-box / modal-action / modal-backdrop                     | chat (5个), own-info, contact-sidebar (3个)      | Dialog                             |
| dropdown / dropdown-end / dropdown-content                            | chat, contact-sidebar                            | DropdownMenu                       |
| menu / menu-title                                                     | chat, manager                                    | 自定义导航 / NavigationMenu        |
| badge (sm/xs/success/warning/error/ghost/outline)                     | dashboard, manager, message-bubble               | Badge                              |
| avatar + ring                                                         | nav-bar, chat, message-bubble, own-info          | Avatar                             |
| tooltip / tooltip-right / tooltip-left                                | nav-bar, chat                                    | Tooltip                            |
| collapse / collapse-arrow / collapse-title / collapse-content         | session-sidebar, contact-sidebar                 | Collapsible                        |
| chat / chat-start / chat-end / chat-bubble / chat-image / chat-header | message-bubble                                   | MessageScroller + Message + Bubble |
| checkbox / checkbox-sm / checkbox-xs / checkbox-primary               | chat, manager                                    | Checkbox                           |
| radio / radio-sm / radio-primary                                      | chat (edit group)                                | RadioGroup                         |
| file-input / file-input-bordered / file-input-sm                      | chat, own-info                                   | Button + hidden input              |
| table / table-sm                                                      | manager                                          | Table                              |
| fieldset / label                                                      | login, register, chat, own-info, contact-sidebar | FieldGroup + Field + FieldLabel    |
| link / link-primary                                                   | login, register                                  | 自定义链接样式                     |

### 1.2 daisyUI 色彩 token 使用

- base-100 / base-200 / base-300 / base-content (背景层级 + 文字)
- primary / primary-content
- accent / accent-content
- neutral
- error / success / warning / info
- 各种透明度变体 (base-content/60, base-content/40, error/10 等)

### 1.3 Lit Web Components (需先转为 React)

| Lit 组件        | 文件                              | 功能                              |
| --------------- | --------------------------------- | --------------------------------- |
| nav-bar         | src/components/nav-bar.ts         | 左侧图标导航栏                    |
| session-sidebar | src/components/session-sidebar.ts | 会话列表侧边栏 (折叠面板)         |
| message-bubble  | src/components/message-bubble.ts  | 聊天气泡 (文本 + 文件)            |
| contact-sidebar | src/components/contact-sidebar.ts | 联系人侧边栏 (折叠面板 + 3个弹窗) |
| video-call      | src/components/video-call.ts      | 视频通话弹窗 (WebRTC)             |

每个 Lit 组件都有对应的 @lit/react 包装文件 (*.react.ts), 迁移后一并删除。

---

## 二、迁移步骤 (共 7 个阶段)

### 阶段 1: 初始化 shadcn/ui

1.1 运行 shadcn init:

```bash
pnpm dlx shadcn@latest init
```

选择:

- style: nova (或根据偏好选择)
- base: radix
- 确认 CSS 文件为 src/index.css
- 确认 import alias 为 @/

1.2 安装所需 shadcn 组件:

```bash
pnpm dlx shadcn@latest add \
  button card input textarea dialog dropdown-menu \
  badge avatar tooltip collapsible checkbox radio-group \
  table label separator scroll-area sonner \
  message-scroller message bubble attachment marker
```

1.3 安装额外依赖:

```bash
pnpm add sonner class-variance-authority clsx tailwind-merge
```

1.4 确认 src/lib/utils.ts 已生成 (包含 cn 函数)。

### 阶段 2: 浅粉色主题配置

2.1 重写 src/index.css, 移除所有 daisyUI 相关内容:

```css
@import "tailwindcss";

/* --- 移除以下 daisyUI 内容 --- */
/* @plugin "daisyui" { ... } */
/* @plugin "daisyui/theme" { ... } */
/* .input:focus 等 daisyUI 覆盖样式 */

/* --- shadcn/ui 主题变量 --- */
:root {
  --radius: 0.625rem;

  /* 浅粉色主题 - 柔和温暖的粉色调 */
  --background: oklch(0.97 0.008 350);
  --foreground: oklch(0.25 0.02 350);

  --card: oklch(0.99 0.004 350);
  --card-foreground: oklch(0.25 0.02 350);

  --popover: oklch(0.99 0.004 350);
  --popover-foreground: oklch(0.25 0.02 350);

  /* 主色: 柔玫瑰粉 */
  --primary: oklch(0.65 0.15 350);
  --primary-foreground: oklch(0.98 0.01 350);

  /* 次要色: 淡粉灰 */
  --secondary: oklch(0.93 0.02 350);
  --secondary-foreground: oklch(0.3 0.03 350);

  --muted: oklch(0.94 0.015 350);
  --muted-foreground: oklch(0.5 0.02 350);

  /* 强调色: 稍深的粉 */
  --accent: oklch(0.9 0.04 350);
  --accent-foreground: oklch(0.25 0.03 350);

  --destructive: oklch(0.58 0.22 25);
  --destructive-foreground: oklch(0.98 0.01 25);

  --border: oklch(0.9 0.02 350);
  --input: oklch(0.88 0.025 350);
  --ring: oklch(0.65 0.15 350);

  /* 图表色 */
  --chart-1: oklch(0.65 0.15 350);
  --chart-2: oklch(0.7 0.12 330);
  --chart-3: oklch(0.75 0.1 310);
  --chart-4: oklch(0.68 0.14 20);
  --chart-5: oklch(0.72 0.11 40);

  /* 侧边栏 */
  --sidebar: oklch(0.96 0.012 350);
  --sidebar-foreground: oklch(0.25 0.02 350);
  --sidebar-primary: oklch(0.65 0.15 350);
  --sidebar-primary-foreground: oklch(0.98 0.01 350);
  --sidebar-accent: oklch(0.92 0.03 350);
  --sidebar-accent-foreground: oklch(0.25 0.03 350);
  --sidebar-border: oklch(0.9 0.02 350);
  --sidebar-ring: oklch(0.65 0.15 350);

  /* 成功/警告/信息 (自定义扩展) */
  --success: oklch(0.62 0.2 145);
  --success-foreground: oklch(0.98 0.01 145);
  --warning: oklch(0.78 0.18 80);
  --warning-foreground: oklch(0.25 0.05 80);
  --info: oklch(0.65 0.15 230);
  --info-foreground: oklch(0.98 0.01 230);
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  --color-sidebar: var(--sidebar);
  --color-sidebar-foreground: var(--sidebar-foreground);
  --color-sidebar-primary: var(--sidebar-primary);
  --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);
  --color-sidebar-accent: var(--sidebar-accent);
  --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
  --color-sidebar-border: var(--sidebar-border);
  --color-sidebar-ring: var(--sidebar-ring);
  --color-success: var(--success);
  --color-success-foreground: var(--success-foreground);
  --color-warning: var(--warning);
  --color-warning-foreground: var(--warning-foreground);
  --color-info: var(--info);
  --color-info-foreground: var(--info-foreground);

  --font-sans:
    Iosevka, Maple Mono, Menlo, Cascadia Code, Sarasa Gothic SC, PingFang SC,
    Microsoft YaHei, sans-serif;
  --font-mono:
    Iosevka, Maple Mono, Menlo, Cascadia Code, Sarasa Gothic SC, PingFang SC,
    Microsoft YaHei, monospace;

  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
  }
}
```

2.2 更新 index.html:

```html
<!-- 移除 data-theme="swifty-light" -->
<html lang="en">
  <!-- 移除 body 上的 bg-base-200 -->
  <body></body>
</html>
```

### 阶段 3: Lit 组件转 React

这是最关键的一步。5 个 Lit 组件需要完全重写为 React 函数组件, 同时用 shadcn 组件替换 daisyUI 类名。

3.1 nav-bar.ts -> src/components/nav-bar.tsx

- Lit LitElement -> React 函数组件
- @property -> React props 接口
- CustomEvent dispatch -> 回调 props
- daisyUI 替换:
  - tooltip tooltip-right -> shadcn Tooltip + TooltipTrigger + TooltipContent
  - btn btn-ghost btn-sm btn-square -> Button variant="ghost" size="icon"
  - avatar + ring -> shadcn Avatar + AvatarImage + AvatarFallback
  - text-error -> text-destructive
  - bg-base-200 -> bg-muted
  - border-base-300 -> border-border
- 删除 iconToSvg + unsafeHTML, 直接使用 lucide-react 图标
- 删除 nav-bar.react.ts 包装文件

3.2 session-sidebar.ts -> src/components/session-sidebar.tsx

- @state -> useState
- api 调用逻辑保持不变
- daisyUI 替换:
  - input input-bordered input-sm -> shadcn Input
  - collapse collapse-arrow -> shadcn Collapsible + CollapsibleTrigger + CollapsibleContent
  - avatar -> shadcn Avatar
  - hover:bg-base-200 -> hover:bg-accent
- 删除 session-sidebar.react.ts

3.3 message-bubble.ts -> src/components/message-bubble.tsx

- 这是改动最大的组件
- daisyUI chat 组件 -> shadcn 聊天组件:
  - chat chat-start/chat-end -> Message align="start"/"end"
  - chat-image avatar -> MessageAvatar + Avatar
  - chat-header -> MessageHeader
  - chat-bubble / chat-bubble-primary -> Bubble variant
  - badge badge-sm badge-ghost -> Badge variant="secondary"
  - btn btn-sm btn-soft btn-accent -> Button variant="outline" size="sm"
- 使用 MessageScroller 替代手动滚动:
  - MessageScrollerProvider autoScroll
  - MessageScroller + MessageScrollerViewport + MessageScrollerContent
  - MessageScrollerItem
  - MessageScrollerButton (跳到底部)
- 文件消息使用 Attachment 组件:
  - Attachment state="done"
  - AttachmentMedia variant="icon"
  - AttachmentContent + AttachmentTitle + AttachmentDescription
  - AttachmentActions + AttachmentAction (下载按钮)
- 删除 message-bubble.react.ts
- chat.tsx 中的 scrollToBottom 逻辑和手动滚动容器可以移除

3.4 contact-sidebar.ts -> src/components/contact-sidebar.tsx

- @state -> useState
- @query -> useRef
- daisyUI 替换:
  - input input-bordered -> Input
  - dropdown dropdown-end -> DropdownMenu + DropdownMenuTrigger + DropdownMenuContent + DropdownMenuItem
  - btn btn-soft btn-accent btn-sm btn-square -> Button variant="outline" size="icon"
  - collapse collapse-arrow -> Collapsible
  - modal -> Dialog + DialogContent + DialogHeader + DialogTitle + DialogFooter
  - fieldset / label -> FieldGroup + Field + FieldLabel
  - textarea textarea-bordered -> Textarea
  - btn btn-sm btn-accent -> Button (default variant)
  - btn btn-sm btn-ghost -> Button variant="ghost"
  - btn btn-xs -> Button size="sm"
- 删除 contact-sidebar.react.ts

3.5 video-call.ts -> src/components/video-call.tsx

- @state -> useState
- Lit connectedCallback/disconnectedCallback -> useEffect
- 暴露 show() / handleSignal() -> useImperativeHandle + forwardRef
- daisyUI 替换:
  - card card-border -> Card + CardContent + CardHeader + CardTitle
  - badge badge-sm badge-ghost -> Badge variant="secondary"
  - btn 各变体 -> Button 对应变体
  - bg-neutral -> bg-muted
  - rounded-box -> rounded-lg
- 删除 video-call.react.ts

### 阶段 4: React 页面迁移

逐页替换 daisyUI 类名为 shadcn 组件 + 语义化 token。

4.1 色彩 token 映射表 (全局适用):

| daisyUI token        | shadcn/Tailwind 替换      |
| -------------------- | ------------------------- |
| bg-base-100          | bg-card                   |
| bg-base-200          | bg-background 或 bg-muted |
| bg-base-300          | bg-accent                 |
| text-base-content    | text-foreground           |
| text-base-content/60 | text-muted-foreground     |
| text-base-content/40 | text-muted-foreground/70  |
| border-base-300      | border-border             |
| border-base-200      | border-border             |
| text-primary         | text-primary              |
| bg-primary           | bg-primary                |
| text-error           | text-destructive          |
| hover:bg-error/10    | hover:bg-destructive/10   |
| hover:bg-base-200    | hover:bg-accent           |
| hover:bg-base-300    | hover:bg-accent           |
| ring-primary/30      | ring-primary/30           |
| ring-base-300        | ring-border               |
| ring-offset-base-100 | ring-offset-card          |

4.2 login.tsx:

- card border-base-300 bg-base-100 -> Card + CardHeader + CardTitle + CardContent + CardFooter
- fieldset / label -> FieldGroup + Field + FieldLabel
- input input-bordered -> Input
- btn btn-accent -> Button (default)
- link link-primary -> 带 text-primary 的 button 或 Link

4.3 register.tsx:

- 与 login.tsx 相同的映射

4.4 chat.tsx (改动最大):

- 外层容器: card card-border -> Card
- 顶部栏: 保持自定义布局, 替换色彩 token
- dropdown -> DropdownMenu 全套组件
- avatar -> Avatar
- tooltip -> Tooltip
- 5 个 dialog modal -> 5 个 Dialog 组件:
  - user-info-modal -> Dialog (用户信息)
  - group-info-modal -> Dialog (群组信息)
  - edit-group-modal -> Dialog + FieldGroup + Input + Textarea + RadioGroup
  - remove-members-modal -> Dialog + Checkbox 列表
  - join-requests-modal -> Dialog + 按钮列表
- textarea textarea-ghost -> Textarea
- btn btn-accent -> Button
- checkbox checkbox-sm checkbox-primary -> Checkbox
- radio radio-sm radio-primary -> RadioGroup + RadioGroupItem
- file-input -> Button variant="outline" + hidden input
- 移除所有 document.getElementById(...).showModal() / .close() 调用, 改用 React state 控制 Dialog open

4.5 session-list.tsx / contact-list.tsx:

- card card-border -> Card
- 色彩 token 替换
- 图标区域保持不变

4.6 own-info.tsx:

- card -> Card
- avatar -> Avatar
- dialog modal -> Dialog
- fieldset -> FieldGroup + Field
- input / file-input -> Input / Button + hidden input
- btn 变体 -> Button 变体

4.7 manager.tsx:

- card -> Card
- menu / menu-title -> 自定义侧边导航 (div + Button variant="ghost")
- table table-sm -> Table + TableHeader + TableBody + TableRow + TableCell + TableHead
- checkbox checkbox-xs -> Checkbox
- badge badge-sm badge-success/error -> Badge 对应变体
- btn 变体 -> Button 变体

4.8 dashboard.tsx:

- card -> Card
- badge 各变体 -> Badge
- btn 变体 -> Button
- 虚拟列表逻辑保持不变

4.9 not-found.tsx:

- btn btn-accent -> Button
- text-primary/20 -> text-primary/20 (保留)
- 色彩 token 替换

4.10 app.tsx:

- RootErrorBoundary: bg-base-200 -> bg-background, text-error -> text-destructive

### 阶段 5: Toast 系统迁移

5.1 用 sonner 替换自定义 toast:

```bash
pnpm add sonner
```

5.2 在 App 组件中添加 Toaster:

```tsx
import { Toaster } from "@/components/ui/sonner";

// 在 RouterProvider 旁边
<Toaster position="top-right" richColors />;
```

5.3 重写 src/utils/toast.ts:

```ts
import { toast } from "sonner";

export function showToast(
  message: string,
  type: "info" | "success" | "warning" | "error" = "info",
) {
  switch (type) {
    case "success":
      toast.success(message);
      break;
    case "error":
      toast.error(message);
      break;
    case "warning":
      toast.warning(message);
      break;
    default:
      toast.info(message);
      break;
  }
}
```

### 阶段 6: 清理与移除

6.1 删除 Lit 相关文件:

```
src/components/nav-bar.ts
src/components/nav-bar.react.ts
src/components/session-sidebar.ts
src/components/session-sidebar.react.ts
src/components/message-bubble.ts
src/components/message-bubble.react.ts
src/components/contact-sidebar.ts
src/components/contact-sidebar.react.ts
src/components/video-call.ts
src/components/video-call.react.ts
src/utils/icon.ts          (iconToSvg 不再需要)
```

6.2 卸载不再需要的依赖:

```bash
pnpm remove daisyui lit @lit/react
```

6.3 清理 index.css:

- 移除 @plugin "daisyui" 块
- 移除 @plugin "daisyui/theme" 块
- 移除 .input:focus / .textarea:focus 覆盖样式

6.4 清理 index.html:

- 移除 data-theme="swifty-light"
- 移除 body 上的 class="bg-base-200"

6.5 清理 tsconfig.app.json:

- 移除 experimentalDecorators (Lit 装饰器不再需要)
- 恢复 useDefineForClassFields 为默认值

### 阶段 7: 验证与调优

7.1 构建验证:

```bash
pnpm typecheck
pnpm build
```

7.2 全局搜索残留 daisyUI 类名:

```bash
grep -rn "btn\b\|card\b\|modal\b\|badge\b\|avatar\b\|tooltip\b\|collapse\b\|chat\b\|input-bordered\|textarea-\|file-input\|base-100\|base-200\|base-300\|base-content\|link-primary\|fieldset\b\|menu-title\|dropdown\b\|checkbox\b\|radio\b" src/
```

7.3 视觉走查清单:

- [ ] 登录页: 卡片居中, 粉色主题, 输入框聚焦有粉色 ring
- [ ] 注册页: 与登录页风格一致
- [ ] 会话列表: 左侧导航栏图标 + tooltip, 折叠面板展开/收起流畅
- [ ] 联系人列表: 同上, 下拉菜单正常
- [ ] 聊天页: 气泡对齐正确, 文件附件显示正常, 所有弹窗可打开/关闭
- [ ] 个人信息: 头像显示, 编辑弹窗表单完整
- [ ] 管理面板: 表格渲染, 复选框, 状态 badge 颜色正确
- [ ] 缓存仪表盘: 虚拟列表滚动流畅, 状态 badge 正确
- [ ] 404 页: 简洁居中
- [ ] Toast 通知: 右上角弹出, 颜色区分

---

## 三、文件变更总览

### 新增文件

| 文件                               | 说明                                   |
| ---------------------------------- | -------------------------------------- |
| components.json                    | shadcn 配置                            |
| src/lib/utils.ts                   | cn() 工具函数 (shadcn 自动生成)        |
| src/components/ui/*.tsx            | shadcn 组件 (~20 个文件, CLI 自动生成) |
| src/components/nav-bar.tsx         | React 导航栏 (替代 Lit)                |
| src/components/session-sidebar.tsx | React 会话侧边栏                       |
| src/components/message-bubble.tsx  | React 消息气泡                         |
| src/components/contact-sidebar.tsx | React 联系人侧边栏                     |
| src/components/video-call.tsx      | React 视频通话                         |

### 修改文件

| 文件                       | 改动范围                                                    |
| -------------------------- | ----------------------------------------------------------- |
| package.json               | 移除 daisyui/lit/@lit/react, 新增 shadcn 依赖               |
| index.html                 | 移除 data-theme, body class                                 |
| src/index.css              | 完全重写: daisyUI 主题 -> shadcn CSS 变量                   |
| src/app.tsx                | 色彩 token 替换, 添加 Toaster                               |
| src/pages/login.tsx        | daisyUI -> shadcn 组件                                      |
| src/pages/register.tsx     | 同上                                                        |
| src/pages/chat.tsx         | 大量改动: 5 个 modal -> Dialog, dropdown -> DropdownMenu 等 |
| src/pages/session-list.tsx | Card + token 替换                                           |
| src/pages/contact-list.tsx | 同上                                                        |
| src/pages/own-info.tsx     | Dialog + 表单组件替换                                       |
| src/pages/manager.tsx      | Table + Checkbox + Badge 替换                               |
| src/pages/dashboard.tsx    | Badge + Button 替换                                         |
| src/pages/not-found.tsx    | Button + token 替换                                         |
| src/utils/toast.ts         | 改用 sonner                                                 |
| tsconfig.app.json          | 移除 Lit 装饰器配置                                         |

### 删除文件

| 文件                                    | 原因                    |
| --------------------------------------- | ----------------------- |
| src/components/nav-bar.ts               | Lit -> React            |
| src/components/nav-bar.react.ts         | @lit/react 包装不再需要 |
| src/components/session-sidebar.ts       | 同上                    |
| src/components/session-sidebar.react.ts | 同上                    |
| src/components/message-bubble.ts        | 同上                    |
| src/components/message-bubble.react.ts  | 同上                    |
| src/components/contact-sidebar.ts       | 同上                    |
| src/components/contact-sidebar.react.ts | 同上                    |
| src/components/video-call.ts            | 同上                    |
| src/components/video-call.react.ts      | 同上                    |
| src/utils/icon.ts                       | iconToSvg 仅 Lit 使用   |

---

## 四、浅粉色设计要点

色彩方向: 以 oklch 色相 350 (玫瑰粉) 为主轴, 打造柔和、温暖、有层次感的浅粉界面。

- 背景层: 极浅粉白 (oklch 0.97, chroma 0.008), 不是纯白, 带一丝粉调
- 卡片层: 更接近纯白但仍有粉调 (oklch 0.99, chroma 0.004)
- 主色: 柔玫瑰粉 (oklch 0.65, chroma 0.15), 用于按钮、链接、聚焦环
- 次要色: 淡粉灰 (oklch 0.93), 用于 hover 状态和次要按钮
- 边框: 粉色边框 (oklch 0.90), 比灰色边框更柔和
- 文字: 深粉棕 (oklch 0.25, hue 350), 不是纯黑, 与粉调和谐
- 成功/警告/错误: 保持功能性色彩但降低饱和度, 与粉调协调

排版: 保留现有的 Iosevka / Maple Mono 等宽字体配置。

圆角: --radius: 0.625rem, 柔和圆润。

---

## 五、风险与注意事项

1. Lit -> React 转换中, contact-sidebar 最复杂 (3 个弹窗 + 折叠面板 + 搜索), 需要仔细处理状态管理
2. video-call 组件使用 useImperativeHandle 暴露 show/handleSignal 方法, 需确保 ref 转发正确
3. message-bubble 改用 MessageScroller 后, chat.tsx 中的手动 scrollToBottom 逻辑需同步移除
4. daisyUI 的 collapse 组件自带展开/收起动画, 迁移到 Collapsible 时需确认动画效果
5. 所有 document.getElementById(...).showModal() 调用需替换为 React state 驱动的 Dialog
6. manager.tsx 的 table 是原生 HTML table + daisyUI 类, 需完整替换为 shadcn Table 组合
7. 移除 experimentalDecorators 前确认没有其他文件依赖装饰器语法

---

## 六、执行顺序建议

推荐按以下顺序执行, 每步完成后可独立验证:

1. 阶段 1 (shadcn init) + 阶段 2 (主题) -> 验证: 构建通过, 主题变量生效
2. 阶段 5 (toast) -> 验证: toast 弹出正常
3. 阶段 3.1 (nav-bar) -> 验证: 导航栏渲染, tooltip 正常
4. 阶段 3.2 (session-sidebar) -> 验证: 折叠面板展开, 会话列表加载
5. 阶段 3.3 (message-bubble) -> 验证: 聊天气泡显示, 文件附件正常
6. 阶段 3.4 (contact-sidebar) -> 验证: 联系人列表, 3 个弹窗正常
7. 阶段 3.5 (video-call) -> 验证: 弹窗打开/关闭
8. 阶段 4 (所有页面) -> 验证: 逐页视觉走查
9. 阶段 6 (清理) -> 验证: pnpm build 通过, 无 daisyUI 残留
10. 阶段 7 (最终验证)
