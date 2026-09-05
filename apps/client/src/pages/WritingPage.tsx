import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FilePenLine, Search, Sparkles } from "lucide-react";
import { listWritingSessions, startWritingSession } from "../api";
import type { WritingSession } from "../types";
import { useLocation, useNavigate } from "react-router-dom";
import { isMiniProgramEdition } from "../edition";
import { getDemoSessionDifficulty, isDemoUser, waitForDemoGeneration } from "../demoExperience";
import { useAppStore } from "../store";
import { DemoGenerationProgress } from "../components/DemoGenerationProgress";

function parseWritingMinutes(value: string): { value?: number; error?: string } {
  const normalized = value.trim();
  if (!normalized) return { error: "请输入限时时长。" };
  if (!/^\d+$/.test(normalized)) return { error: "限时必须是整数分钟。" };
  const minutes = Number(normalized);
  if (minutes < 5 || minutes > 120) return { error: "限时需在 5–120 分钟之间。" };
  return { value: minutes };
}

function getWritingStatus(session: WritingSession): { label: string; action: string; className: string } {
  if (session.status === "COMPLETED") return { label: "已评分", action: "查看评分", className: "done" };
  if (session.status === "EVALUATING") return { label: "评分中", action: "查看进度", className: "todo" };
  return { label: "待写作", action: "继续写作", className: "todo" };
}

function formatStartedAt(startedAt: string): string {
  const timestamp = new Date(startedAt);
  return Number.isNaN(timestamp.getTime()) ? "刚刚创建" : timestamp.toLocaleString("zh-CN", { dateStyle: "medium", timeStyle: "short" });
}

export function WritingPage() {
	const user = useAppStore((state) => state.user);
  const demoUser = isDemoUser(user);
  const navigate = useNavigate();
  const location = useLocation();
  const isLibraryPage = location.pathname === "/writing/library";
  const [exam, setExam] = useState<"IELTS" | "CET4" | "CET6">("IELTS");
  const [minutesText, setMinutesText] = useState("40");
  const [minutesError, setMinutesError] = useState<string | null>(null);
  const [sessions, setSessions] = useState<WritingSession[]>([]);
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [sessionsError, setSessionsError] = useState("");
  const [message, setMessage] = useState("");
  const [historyMessage, setHistoryMessage] = useState("");
  const [starting, setStarting] = useState(false);
  const [demoProgressComplete, setDemoProgressComplete] = useState(false);
  const [historyQuery, setHistoryQuery] = useState("");
  const [historyExam, setHistoryExam] = useState<"ALL" | "IELTS" | "CET4" | "CET6">("ALL");
  const [historyStatus, setHistoryStatus] = useState<"ALL" | WritingSession["status"]>("ALL");
  const [historyPage, setHistoryPage] = useState(1);
  const minutesInputRef = useRef<HTMLInputElement>(null);
  const historyTitleRef = useRef<HTMLHeadingElement>(null);

  const loadSessions = useCallback(async () => {
    setLoadingSessions(true);
    setSessionsError("");
    setSessions([]);
    try {
      setSessions(await listWritingSessions());
    } catch (error) {
      setSessionsError((error as Error).message || "写作记录加载失败，请稍后重试。");
    } finally {
      setLoadingSessions(false);
    }
  }, []);

  useEffect(() => {
    if (isLibraryPage) void loadSessions();
  }, [isLibraryPage, loadSessions]);

  useEffect(() => {
    const state = location.state as { message?: string } | null;
    if (!isLibraryPage || !state?.message) return;
    setHistoryMessage(state.message);
    void loadSessions();
    navigate(location.pathname, { replace: true, state: null });
  }, [isLibraryPage, loadSessions, location.pathname, location.state, navigate]);

  useEffect(() => {
    if (historyMessage && !loadingSessions) historyTitleRef.current?.focus();
  }, [historyMessage, loadingSessions]);

  const filteredSessions = useMemo(() => {
    const query = historyQuery.trim().toLowerCase();
    return sessions.filter((session) => {
      const matchesExam = historyExam === "ALL" || session.exam === historyExam;
      const matchesStatus = historyStatus === "ALL" || session.status === historyStatus;
      const matchesQuery = !query || [session.prompt.title, session.prompt.instructions, session.exam].some((value) => value.toLowerCase().includes(query));
      return matchesExam && matchesStatus && matchesQuery;
    }).sort((left, right) => Date.parse(right.startedAt) - Date.parse(left.startedAt));
  }, [historyExam, historyQuery, historyStatus, sessions]);
  const historyPageCount = Math.max(1, Math.ceil(filteredSessions.length / 8));
  const visibleSessions = filteredSessions.slice((historyPage - 1) * 8, historyPage * 8);

  useEffect(() => {
    setHistoryPage(1);
  }, [historyExam, historyQuery, historyStatus]);

  async function handleStart(event: FormEvent) {
    event.preventDefault();
    const parsedMinutes = parseWritingMinutes(minutesText);
    if (parsedMinutes.error || parsedMinutes.value === undefined) {
      setMinutesError(parsedMinutes.error ?? "请输入有效限时。");
      minutesInputRef.current?.focus();
      return;
    }
    setStarting(true); setMessage(""); setDemoProgressComplete(false);
    const startedAt = Date.now();
    try {
      const created = await startWritingSession(exam, parsedMinutes.value * 60, getDemoSessionDifficulty(user));
      if (demoUser) {
        await waitForDemoGeneration(startedAt);
        setDemoProgressComplete(true);
        await new Promise((resolve) => window.setTimeout(resolve, 350));
      }
      navigate(`/writing/${created.id}`);
    } catch (error) { setMessage((error as Error).message || (isMiniProgramEdition ? "题目生成失败，线上 AI 服务暂时不可用，请稍后重试。" : "题目生成失败，请检查模型配置。")); }
    finally { setStarting(false); }
  }

  return (
    <main className="page writing-page">
      {!isLibraryPage ? <><section className="card writing-hero">
        <span className="eyebrow"><FilePenLine size={15} /> English writing lab</span>
        <h2>限时英语写作</h2>
        <p>生成主题后在规定时间内写作；历史练习可随时回访评分与建议。</p>
        {demoUser ? <article className="stage-banner"><strong>演示模式 · 学习权限已开放</strong><p>IELTS、四级和六级写作均使用预置题目与评分流程，不调用 AI，不消耗点数。</p></article> : null}
        <form className="writing-start" onSubmit={handleStart}>
          <label>考试类型<select value={exam} onChange={(event) => setExam(event.target.value as typeof exam)}><option value="IELTS">IELTS</option><option value="CET4">大学英语四级</option><option value="CET6">大学英语六级</option></select></label>
          <label>
            限时（分钟）
            <input
              ref={minutesInputRef}
              type="number"
              min="5"
              max="120"
              step="1"
              inputMode="numeric"
              value={minutesText}
              aria-invalid={Boolean(minutesError)}
              aria-describedby={minutesError ? "writing-minutes-hint writing-minutes-error" : "writing-minutes-hint"}
              onChange={(event) => { setMinutesText(event.target.value); setMinutesError(null); }}
              onBlur={() => {
                const parsedMinutes = parseWritingMinutes(minutesText);
                if (parsedMinutes.error || parsedMinutes.value === undefined) {
                  setMinutesError(parsedMinutes.error ?? "请输入有效限时。");
                  return;
                }
                setMinutesText(String(parsedMinutes.value));
                setMinutesError(null);
              }}
            />
            <small id="writing-minutes-hint">5–120 分钟，整数</small>
            {minutesError ? <span id="writing-minutes-error" className="field-error" role="alert">{minutesError}</span> : null}
          </label>
          <button type="submit" disabled={starting}><Sparkles size={16} /> {starting ? "生成中…" : "生成写作主题"}</button>
        </form>
        <DemoGenerationProgress
          active={demoUser && starting}
          complete={demoProgressComplete}
          title="正在准备你的英语写作任务"
          note="正在载入预置题目、限时配置和评分维度，不调用 AI，不消耗点数。"
          steps={["确认考试类型", "准备写作主题", "配置限时时间", "即将开始写作"]}
        />
        {message ? <p className="muted-note">{message}</p> : null}
      </section>
      <section className="card stage-banner" style={{ marginTop: 16 }}><h3>我的写作库</h3><p>在独立子页中搜索、筛选并按页查看全部写作练习。</p><button type="button" className="btn-ghost" onClick={() => navigate("/writing/library")}>查看写作材料</button></section></> : null}

      {isLibraryPage ? <section className="card writing-library" aria-labelledby="writing-history-title">
        <div className="route-header">
          <div><h2 ref={historyTitleRef} id="writing-history-title" tabIndex={-1}>我的写作库</h2><p>搜索、筛选并按页查看写作练习。</p></div>
          <div className="row"><button type="button" className="btn-ghost" onClick={() => void loadSessions()} disabled={loadingSessions}>重新加载</button><button type="button" className="btn-ghost" onClick={() => navigate("/writing")}>返回写作训练</button></div>
         </div>
         <div className="library-toolbar" aria-label="筛选写作练习">
           <label className="library-search"><Search size={16} /><span className="sr-only">搜索写作练习</span><input value={historyQuery} onChange={(event) => setHistoryQuery(event.target.value)} placeholder="搜索题目或要求" /></label>
           <label>考试<select value={historyExam} onChange={(event) => setHistoryExam(event.target.value as typeof historyExam)}><option value="ALL">全部考试</option><option value="IELTS">IELTS</option><option value="CET4">大学英语四级</option><option value="CET6">大学英语六级</option></select></label>
           <label>状态<select value={historyStatus} onChange={(event) => setHistoryStatus(event.target.value as typeof historyStatus)}><option value="ALL">全部状态</option><option value="WRITING">待写作</option><option value="EVALUATING">评分中</option><option value="COMPLETED">已评分</option></select></label>
         </div>
         {loadingSessions ? <p className="muted-note">正在加载写作记录…</p> : null}
        {historyMessage ? <p role="status" aria-live="polite">{historyMessage}</p> : null}
        {sessionsError ? <p className="field-error" role="alert">{sessionsError}</p> : null}
         {!loadingSessions && !sessionsError && sessions.length === 0 ? <p>还没有写作练习，先生成一份主题开始吧。</p> : null}
         {!loadingSessions && !sessionsError && sessions.length > 0 ? <p className="library-result-count">找到 {filteredSessions.length} 份练习，第 {historyPage} / {historyPageCount} 页。</p> : null}
         {!loadingSessions && !sessionsError && sessions.length > 0 && filteredSessions.length === 0 ? <p className="muted-note">没有符合当前搜索或筛选条件的写作练习。</p> : null}
         {!loadingSessions && sessions.length > 0 ? <ul key={`${historyPage}-${historyQuery}-${historyExam}-${historyStatus}`} className="dialogue-list writing-session-list library-page-list">
           {visibleSessions.map((session) => {
            const status = getWritingStatus(session);
            return <li key={session.id} className="dialogue writing-session-item">
              <div className="row" style={{ justifyContent: "space-between" }}><strong>{session.prompt.title}</strong><span className={`status-pill ${status.className}`}>{status.label}</span></div>
              <p>{session.exam} · {Math.round(session.timeLimitSeconds / 60)} 分钟 · 建议 {session.prompt.suggestedWordCount} words</p>
              <small>创建于 {formatStartedAt(session.startedAt)}</small>
              <div className="dialogue-actions"><button type="button" className="btn-ghost" onClick={() => navigate(`/writing/${session.id}`)}>{status.action}</button></div>
            </li>;
           })}
         </ul> : null}
         {!loadingSessions && !sessionsError && filteredSessions.length > 8 ? <div className="library-pagination"><button type="button" className="btn-ghost" disabled={historyPage <= 1} onClick={() => setHistoryPage((page) => page - 1)}>上一页</button><span>第 {historyPage} / {historyPageCount} 页</span><button type="button" className="btn-ghost" disabled={historyPage >= historyPageCount} onClick={() => setHistoryPage((page) => page + 1)}>下一页</button></div> : null}
       </section> : null}
    </main>
  );
}
