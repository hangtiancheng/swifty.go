type ToastType = "info" | "success" | "warning" | "error";

const ALERT_CLASS: Record<ToastType, string> = {
  info: "bg-green-50 text-green-700 border border-green-200",
  success: "bg-green-100 text-green-800 border border-green-300",
  warning: "bg-amber-50 text-amber-700 border border-amber-200",
  error: "bg-red-50 text-red-700 border border-red-200",
};

export function showToast(message: string, type: ToastType = "info", duration = 3000) {
  const container = document.getElementById("toast-container") ?? createContainer();
  const el = document.createElement("div");
  el.className = `rounded-lg px-4 py-3 text-sm shadow-md ${ALERT_CLASS[type]}`;
  el.textContent = message;
  container.appendChild(el);
  setTimeout(() => {
    el.remove();
    if (container.childElementCount === 0) container.remove();
  }, duration);
}

function createContainer(): HTMLElement {
  const c = document.createElement("div");
  c.id = "toast-container";
  c.className = "fixed top-4 right-4 z-50 flex flex-col gap-2";
  document.body.appendChild(c);
  return c;
}
