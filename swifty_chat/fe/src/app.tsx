import { useEffect } from "react";
import {
  createBrowserRouter,
  Navigate,
  redirect,
  RouterProvider,
} from "react-router-dom";
import useAuthStore from "./store/auth";
import useWsStore from "./store/ws";
import Login from "./pages/login";
import Register from "./pages/register";
import SessionList from "./pages/session-list";
import ContactList from "./pages/contact-list";
import Chat from "./pages/chat";
import OwnInfo from "./pages/own-info";
import Manager from "./pages/manager";
import Dashboard from "./pages/dashboard";
import NotFound from "./pages/not-found";

/** Guard protected routes: redirect to /login when not authenticated. */
async function rootLoader() {
  const auth = useAuthStore.getState();
  if (!auth.isLoggedIn) {
    return redirect("/login");
  }
  return null;
}

function RootErrorBoundary() {
  return (
    <div className="bg-base-200 flex min-h-screen items-center justify-center">
      <div className="text-center">
        <h1 className="text-error text-2xl font-semibold">
          Something went wrong
        </h1>
        <p className="text-base-content/60 mt-2">Please refresh the page</p>
      </div>
    </div>
  );
}

const router = createBrowserRouter([
  {
    path: "/",
    loader: rootLoader,
    errorElement: <RootErrorBoundary />,
    children: [
      { index: true, element: <Navigate to="/chat/sessions" replace /> },
      { path: "chat/sessions", element: <SessionList /> },
      { path: "chat/contacts", element: <ContactList /> },
      { path: "chat/profile", element: <OwnInfo /> },
      { path: "chat/:id", element: <Chat /> },
      { path: "manager", element: <Manager /> },
      { path: "dashboard", element: <Dashboard /> },
    ],
  },
  { path: "/login", element: <Login /> },
  { path: "/register", element: <Register /> },
  { path: "*", element: <NotFound /> },
]);

export default function App() {
  // Reconnect websocket on reload when already logged in (mirrors source boot.ts)
  useEffect(() => {
    const auth = useAuthStore.getState();
    if (auth.isLoggedIn) {
      useWsStore.getState().connect(auth.userInfo.uuid);
    }
  }, []);

  return <RouterProvider router={router} />;
}
