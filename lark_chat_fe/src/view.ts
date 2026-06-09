import { defineView, Router } from "@lark.js/mvc";

export default defineView({
  navigate(path: string, params?: Record<string, unknown>) {
    Router.to(path, params);
  },
});
