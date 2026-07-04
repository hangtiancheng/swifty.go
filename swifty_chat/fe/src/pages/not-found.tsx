import { useNavigate } from "react-router-dom";

export default function NotFound() {
  const navigate = useNavigate();
  return (
    <div className="bg-base-200 flex min-h-screen items-center justify-center">
      <div className="text-center">
        <h1 className="text-primary/20 text-7xl font-bold">404</h1>
        <p className="text-base-content/60 mt-4">Page not found</p>
        <button
          className="btn btn-accent mt-8 font-normal"
          onClick={() => navigate("/login")}
        >
          Back to Home
        </button>
      </div>
    </div>
  );
}
