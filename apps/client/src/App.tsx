import { BookOpenText, Clapperboard, Compass, UserRound } from "lucide-react";
import { Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom";
import { LoginPage } from "./pages/LoginPage";
import { GeneratePage } from "./pages/GeneratePage";
import { TheaterPage } from "./pages/TheaterPage";
import { QuizPage } from "./pages/QuizPage";
import { ResultPage } from "./pages/ResultPage";
import { CoursesPage } from "./pages/CoursesPage";
import { LibraryPage } from "./pages/LibraryPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RoleplayPage } from "./pages/RoleplayPage";

function MobileBottomNav() {
  const location = useLocation();
  if (location.pathname.startsWith("/login")) return null;

  return (
    <nav className="mobile-bottom-nav" aria-label="移动端主导航">
      <NavLink to="/courses" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}>
        <Compass size={16} />
        <span>路线</span>
      </NavLink>
      <NavLink to="/library" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}>
        <BookOpenText size={16} />
        <span>剧场库</span>
      </NavLink>
      <NavLink to="/generate" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}>
        <Clapperboard size={16} />
        <span>生成</span>
      </NavLink>
      <NavLink to="/profile" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}>
        <UserRound size={16} />
        <span>我的</span>
      </NavLink>
    </nav>
  );
}

export function App() {
  return (
    <>
      <Routes>
        <Route path="/" element={<Navigate to="/login" replace />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/generate" element={<GeneratePage />} />
        <Route path="/courses" element={<CoursesPage />} />
        <Route path="/theater/:id" element={<TheaterPage />} />
        <Route path="/quiz/:id" element={<QuizPage />} />
        <Route path="/result" element={<ResultPage />} />
        <Route path="/library" element={<LibraryPage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/roleplay/:theaterId" element={<RoleplayPage />} />
      </Routes>
      <MobileBottomNav />
    </>
  );
}
