import { useEffect, useRef, useState } from "react";
import { BookOpenText, Clapperboard, Compass, ScrollText, UserRound } from "lucide-react";
import { Navigate, NavLink, Route, Routes, useLocation, useParams } from "react-router-dom";
import { LoginPage } from "./pages/LoginPage";
import { GeneratePage } from "./pages/GeneratePage";
import { TheaterPage } from "./pages/TheaterPage";
import { QuizPage } from "./pages/QuizPage";
import { ResultPage } from "./pages/ResultPage";
import { CoursesPage } from "./pages/CoursesPage";
import { LibraryPage } from "./pages/LibraryPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RoleplayPage } from "./pages/RoleplayPage";
import { ReadingPage } from "./pages/ReadingPage";
import { ReadingDetailPage } from "./pages/ReadingDetailPage";
import { ReadingGeneratePage } from "./pages/ReadingGeneratePage";

function MobileBottomNav() {
  const location = useLocation();
  const hideTimerRef = useRef<number | null>(null);
  const [desktopMode, setDesktopMode] = useState(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(min-width: 769px) and (pointer: fine)").matches;
  });
  const [desktopVisible, setDesktopVisible] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const media = window.matchMedia("(min-width: 769px) and (pointer: fine)");
    const onChange = () => {
      const next = media.matches;
      setDesktopMode(next);
      if (!next) {
        setDesktopVisible(false);
      }
    };

    onChange();
    media.addEventListener("change", onChange);
    return () => {
      media.removeEventListener("change", onChange);
    };
  }, []);

  useEffect(() => {
    if (!desktopMode || typeof window === "undefined") {
      if (hideTimerRef.current) {
        window.clearTimeout(hideTimerRef.current);
        hideTimerRef.current = null;
      }
      return;
    }

    const reveal = () => {
      setDesktopVisible(true);
      if (hideTimerRef.current) {
        window.clearTimeout(hideTimerRef.current);
      }
      hideTimerRef.current = window.setTimeout(() => {
        setDesktopVisible(false);
      }, 1300);
    };

    const onMouseMove = (event: MouseEvent) => {
      if (event.clientY >= window.innerHeight - 130) {
        reveal();
      }
    };

    window.addEventListener("mousemove", onMouseMove);
    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      if (hideTimerRef.current) {
        window.clearTimeout(hideTimerRef.current);
        hideTimerRef.current = null;
      }
    };
  }, [desktopMode]);

  if (location.pathname.startsWith("/login")) return null;
  if (location.pathname.startsWith("/theater/shared/")) return null;

  const navClassName = [
    "mobile-bottom-nav",
    desktopMode ? "desktop-auto-nav" : "mobile-fixed-nav",
    desktopMode && desktopVisible ? "visible" : ""
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <nav
      className={navClassName}
      aria-label="主导航"
      onMouseEnter={() => setDesktopVisible(true)}
      onMouseLeave={() => {
        if (!desktopMode || typeof window === "undefined") return;
        if (hideTimerRef.current) {
          window.clearTimeout(hideTimerRef.current);
        }
        hideTimerRef.current = window.setTimeout(() => {
          setDesktopVisible(false);
        }, 600);
      }}
    >
      <NavLink
        to="/courses"
        className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
        onFocus={() => setDesktopVisible(true)}
      >
        <Compass size={16} />
        <span>路线</span>
      </NavLink>
      <NavLink
        to="/reading"
        className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
        onFocus={() => setDesktopVisible(true)}
      >
        <ScrollText size={16} />
        <span>阅读</span>
      </NavLink>
      <NavLink
        to="/library"
        className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
        onFocus={() => setDesktopVisible(true)}
      >
        <BookOpenText size={16} />
        <span>剧场库</span>
      </NavLink>
      <NavLink
        to="/generate"
        className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
        onFocus={() => setDesktopVisible(true)}
      >
        <Clapperboard size={16} />
        <span>生成</span>
      </NavLink>
      <NavLink
        to="/profile"
        className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
        onFocus={() => setDesktopVisible(true)}
      >
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
        <Route path="/theater/shared/:shareCode" element={<TheaterPage />} />
        <Route path="/quiz/:id" element={<QuizPage />} />
        <Route path="/result" element={<ResultPage />} />
        <Route path="/library" element={<LibraryPage />} />
        <Route path="/reading" element={<ReadingPage />} />
        <Route path="/reading/generate/:exam/:stage" element={<ReadingGeneratePage />} />
        <Route path="/reading/:id" element={<ReadingDetailRedirect />} />
        <Route path="/reading/:id/:view" element={<ReadingDetailPage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/roleplay/:theaterId" element={<RoleplayPage />} />
      </Routes>
      <MobileBottomNav />
    </>
  );
}

function ReadingDetailRedirect() {
  const { id = "" } = useParams();
  return <Navigate to={`/reading/${id}/article`} replace />;
}
