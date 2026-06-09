import { defineConfig } from "vite";
import { resolve } from "path";
import { larkMvcPlugin } from "@lark.js/mvc/vite";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [larkMvcPlugin(), tailwindcss()],
  resolve: { alias: { "@": resolve(__dirname, "./src") } },
});
