import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { BookOpenText, CircleAlert, Clapperboard, Compass, FilePenLine, ScrollText, UserRound } from "lucide-react";
import { Navigate, NavLink, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
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
import { WritingPage } from "./pages/WritingPage";
import { WritingDetailPage } from "./pages/WritingDetailPage";
import { VoiceDesignPage } from "./pages/VoiceDesignPage";
import { VoiceLibraryPage } from "./pages/VoiceLibraryPage";
import { ReleaseNotesPage } from "./pages/ReleaseNotesPage";
import { isCommercialEdition, isMiniProgramEdition } from "./edition";
import { useAppStore } from "./store";
import { CREDIT_INSUFFICIENT_EVENT, trackClick } from "./api";
import { membershipRoutePaths } from "./membershipRoutes";
import { DemoPage } from "./pages/DemoPage";
import { OnboardingTour } from "./components/OnboardingTour";

const CommercialMembershipPage = __LINGUAQUEST_APP_EDITION__ === "COMMERCIAL"
  ? lazy(() => import("./pages/MembershipPage").then((module) => ({ default: module.MembershipPage })))
  : null;
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

  if (location.pathname.startsWith("/login") || location.pathname.startsWith("/updates")) return null;
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
		data-onboarding="courses-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_COURSES"
		onClick={() => trackClick("NAV_COURSES")}
		onFocus={() => setDesktopVisible(true)}
      >
        <Compass size={16} />
        <span>路线</span>
      </NavLink>
      <NavLink
        to="/reading"
		data-onboarding="reading-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_READING"
		onClick={() => trackClick("NAV_READING")}
        onFocus={() => setDesktopVisible(true)}
      >
        <ScrollText size={16} />
        <span>阅读</span>
      </NavLink>
	  <NavLink to="/writing" data-onboarding="writing-nav" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")} data-analytics-click="NAV_WRITING" onClick={() => trackClick("NAV_WRITING")} onFocus={() => setDesktopVisible(true)}>
        <FilePenLine size={16} /><span>写作</span>
      </NavLink>
      <NavLink
        to="/library"
		data-onboarding="library-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_LIBRARY"
		onClick={() => trackClick("NAV_LIBRARY")}
        onFocus={() => setDesktopVisible(true)}
      >
        <BookOpenText size={16} />
        <span>剧场库</span>
      </NavLink>
      <NavLink
        to="/generate"
		data-onboarding="generate-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_GENERATE"
		onClick={() => trackClick("NAV_GENERATE")}
        onFocus={() => setDesktopVisible(true)}
      >
        <Clapperboard size={16} />
        <span>生成</span>
      </NavLink>
      <NavLink
        to="/profile"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_PROFILE"
		onClick={() => trackClick("NAV_PROFILE")}
        onFocus={() => setDesktopVisible(true)}
      >
        <UserRound size={16} />
        <span>我的</span>
      </NavLink>
    </nav>
  );
}

export function App() {
  const refreshUserXP = useAppStore((s) => s.refreshUserXP);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!localStorage.getItem("accessToken")) return;
    void refreshUserXP();
  }, [refreshUserXP]);

  return (
    <>
      <Routes>
        <Route path="/" element={<Navigate to="/login" replace />} />
		<Route path="/login" element={<LoginPage />} />
		<Route path="/demo" element={<DemoPage />} />
		<Route path="/updates" element={<ReleaseNotesPage />} />
        <Route path="/generate" element={<GeneratePage />} />
        <Route path="/courses" element={<CoursesPage />} />
        <Route path="/theater/:id" element={<TheaterPage />} />
        <Route path="/theater/shared/:shareCode" element={<TheaterPage />} />
        <Route path="/quiz/:id" element={<QuizPage />} />
        <Route path="/result" element={<ResultPage />} />
        <Route path="/library" element={<LibraryPage />} />
        <Route path="/reading" element={<ReadingPage />} />
        <Route path="/reading/library" element={<ReadingPage />} />
        <Route path="/reading/generate/:exam/:stage" element={<ReadingGeneratePage />} />
        <Route path="/reading/:id" element={<ReadingDetailRedirect />} />
        <Route path="/reading/:id/:view" element={<ReadingDetailPage />} />
        <Route path="/writing" element={<WritingPage />} />
        <Route path="/writing/library" element={<WritingPage />} />
        <Route path="/writing/:id" element={<WritingDetailPage />} />
        <Route path="/voices" element={<VoiceLibraryPage />} />
        <Route path="/voices/create" element={<VoiceDesignPage />} />
        {isCommercialEdition && !isMiniProgramEdition && CommercialMembershipPage ? membershipRoutePaths.map((path) => <Route key={path} path={path} element={<Suspense fallback={<main className="page"><p>正在加载会员中心…</p></main>}><CommercialMembershipPage /></Suspense>} />) : null}
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/roleplay/:theaterId" element={<RoleplayPage />} />
      </Routes>
      <MobileBottomNav />
      <OnboardingTour />
      <AICreditInsufficientDialog />
    </>
  );
}

function AICreditInsufficientDialog() {
  const navigate = useNavigate();
  const [message, setMessage] = useState("");
  const dialogRef = useRef<HTMLElement>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);
  const canPurchaseCredits = isCommercialEdition && !isMiniProgramEdition;

  const close = useCallback(() => {
    setMessage("");
    window.setTimeout(() => lastFocusedRef.current?.focus(), 0);
  }, []);

  useEffect(() => {
    const showDialog = (event: Event) => {
      const detail = (event as CustomEvent<{ message?: string }>).detail;
      lastFocusedRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      setMessage(detail?.message?.trim() || "AI 点数不足，请等待每日点数重置后再试。");
    };
    window.addEventListener(CREDIT_INSUFFICIENT_EVENT, showDialog);
    return () => window.removeEventListener(CREDIT_INSUFFICIENT_EVENT, showDialog);
  }, []);

  useEffect(() => {
    if (!message) return;
    dialogRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [close, message]);

  if (!message) return null;

  return (
    <div className="credit-dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) close(); }}>
      <section ref={dialogRef} tabIndex={-1} className="credit-dialog" role="alertdialog" aria-modal="true" aria-labelledby="credit-dialog-title" aria-describedby="credit-dialog-description">
        <span className="credit-dialog-icon"><CircleAlert size={22} /></span>
        <div>
          <p className="credit-dialog-kicker">本次任务未开始</p>
          <h2 id="credit-dialog-title">AI 点数不足</h2>
          <p id="credit-dialog-description">{message}</p>
          <p>请等待每日点数重置后再试；本次请求不会扣除点数。</p>
        </div>
        <div className="credit-dialog-actions">
          {canPurchaseCredits ? <button type="button" onClick={() => { close(); navigate("/membership"); }}>查看点数方案</button> : null}
          <button type="button" className="btn-ghost" onClick={close}>我知道了</button>
        </div>
      </section>
    </div>
  );
}

function ReadingDetailRedirect() {
  const { id = "" } = useParams();
  return <Navigate to={`/reading/${id}/article`} replace />;
}
