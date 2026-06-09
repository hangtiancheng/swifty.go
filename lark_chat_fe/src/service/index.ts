import { Service } from "@lark.js/mvc";

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
        payload.set({ error: (err as Error).message });
        callback();
      });
  },
  30,
  5,
);

export default AppService;
