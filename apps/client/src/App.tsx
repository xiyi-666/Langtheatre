import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { BookOpenText, CircleAlert, Clapperboard, Compass, FilePenLine, ScrollText, UserRound, ClipboardCheck, Mic2, Headphones } from "lucide-react";
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
import { MockExamPage } from "./pages/MockExamPage";
import { SpeakingPage } from "./pages/SpeakingPage";

const ListeningPage = lazy(() => import("./pages/ListeningPage").then(module => ({ default: module.ListeningPage })));

const CommercialMembershipPage = __LINGUAQUEST_APP_EDITION__ === "COMMERCIAL"
  ? lazy(() => import("./pages/MembershipPage").then((module) => ({ default: module.MembershipPage })))
  : null;
function MobileBottomNav() {
  const location = useLocation();

  if (location.pathname.startsWith("/login") || location.pathname.startsWith("/updates")) return null;
  if (location.pathname.startsWith("/theater/shared/")) return null;

  return (
    <nav className="mobile-bottom-nav" aria-label="主导航">
      <NavLink
        to="/courses"
		data-onboarding="courses-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_COURSES"
		onClick={() => trackClick("NAV_COURSES")}
      >
        <Compass size={16} />
        <span>路线</span>
      </NavLink>
      <NavLink to="/listening" data-onboarding="listening-nav" className={({ isActive }) => isActive ? "mobile-nav-link active" : "mobile-nav-link"} onClick={() => trackClick("NAV_LISTENING")}>
        <Headphones size={16} /><span>听力</span>
      </NavLink>
      <NavLink
        to="/reading"
		data-onboarding="reading-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_READING"
		onClick={() => trackClick("NAV_READING")}
      >
        <ScrollText size={16} />
        <span>阅读</span>
      </NavLink>
	  <NavLink to="/mock-exam" data-onboarding="mock-exam-nav" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")} data-analytics-click="NAV_MOCK_EXAM" onClick={() => trackClick("NAV_MOCK_EXAM")}>
        <ClipboardCheck size={16} /><span>模拟考试</span>
      </NavLink>
	  <NavLink to="/speaking" data-onboarding="speaking-nav" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")} data-analytics-click="NAV_SPEAKING" onClick={() => trackClick("NAV_SPEAKING")}>
        <Mic2 size={16} /><span>口语</span>
      </NavLink>
	  <NavLink to="/writing" data-onboarding="writing-nav" className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")} data-analytics-click="NAV_WRITING" onClick={() => trackClick("NAV_WRITING")}>
        <FilePenLine size={16} /><span>写作</span>
      </NavLink>
      <NavLink
        to="/library"
		data-onboarding="library-nav"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_LIBRARY"
		onClick={() => trackClick("NAV_LIBRARY")}
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
      >
        <Clapperboard size={16} />
        <span>生成</span>
      </NavLink>
      <NavLink
        to="/profile"
		className={({ isActive }) => (isActive ? "mobile-nav-link active" : "mobile-nav-link")}
		data-analytics-click="NAV_PROFILE"
		onClick={() => trackClick("NAV_PROFILE")}
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
        <Route path="/mock-exam" element={<MockExamPage />} />
        <Route path="/speaking" element={<SpeakingPage />} />
        <Route path="/listening" element={<Suspense fallback={<main className="page"><p>正在加载听力训练…</p></main>}><ListeningPage /></Suspense>} />
        <Route path="/listening/:id" element={<Suspense fallback={<main className="page"><p>正在加载听力训练…</p></main>}><ListeningPage /></Suspense>} />
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
