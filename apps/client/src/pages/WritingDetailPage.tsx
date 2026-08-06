import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, Clock3, FilePenLine, Sparkles, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { deleteWritingSession, getWritingSession, submitWritingSession } from "../api";
import { AICreditCostNotice } from "../components/AICreditCostNotice";
import type { WritingSession } from "../types";

function formatRemaining(seconds: number) {
  const safe = Math.max(0, seconds);
  return `${String(Math.floor(safe / 60)).padStart(2, "0")}:${String(safe % 60).padStart(2, "0")}`;
}

export function WritingDetailPage() {
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const [session, setSession] = useState<WritingSession>();
  const [essay, setEssay] = useState("");
  const [now, setNow] = useState(Date.now());
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const deleteButtonRef = useRef<HTMLButtonElement | null>(null);

  const loadSession = useCallback(async () => {
    if (!id) {
      setMessage("未找到写作练习。");
      setLoading(false);
      return;
    }
    setLoading(true);
    setMessage("");
    try {
      const loaded = await getWritingSession(id);
      setSession(loaded);
      setEssay(loaded.essay);
      setNow(Date.now());
    } catch (error) {
      setSession(undefined);
      setMessage((error as Error).message || "写作练习加载失败，请稍后重试。");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void loadSession();
  }, [loadSession]);

  useEffect(() => {
    if (session?.status !== "WRITING") return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [session?.id, session?.status]);

  useEffect(() => {
    if (session?.status !== "EVALUATING") return;
    const timer = window.setInterval(() => {
      void getWritingSession(session.id).then(setSession).catch((error) => console.error("refresh writing evaluation failed", error));
    }, 1500);
    return () => window.clearInterval(timer);
  }, [session?.id, session?.status]);

  const remaining = useMemo(() => {
    if (session?.status !== "WRITING") return 0;
    const deadline = new Date(session.startedAt).getTime() + session.timeLimitSeconds * 1000;
    return Math.ceil((deadline - now) / 1000);
  }, [now, session]);
  const wordCount = useMemo(() => essay.trim() ? essay.trim().split(/\s+/).length : 0, [essay]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!session || session.status !== "WRITING" || isDeleting) return;
    setSubmitting(true);
    setMessage("");
    try {
      setSession(await submitWritingSession(session.id, essay));
    } catch (error) {
      setMessage((error as Error).message || "提交失败，请稍后重试。");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDeleteSession() {
    if (!session || isDeleting || session.status === "EVALUATING") return;
    const detail = session.status === "WRITING" ? "当前未提交的写作内容也会丢失。" : "作文内容和 AI 评分将一并删除。";
    const confirmed = window.confirm(`确定删除写作练习“${session.prompt.title}”吗？删除后无法恢复；${detail}`);
    if (!confirmed) return;

    setDeleteError("");
    setIsDeleting(true);
    try {
      await deleteWritingSession(session.id);
      navigate("/writing", { replace: true, state: { message: "写作练习已删除", focus: "writing-history" } });
    } catch (error) {
      setDeleteError((error as Error).message || "删除写作练习失败，请稍后重试。");
      window.requestAnimationFrame(() => deleteButtonRef.current?.focus());
    } finally {
      setIsDeleting(false);
    }
  }

  if (loading) {
    return <main className="page writing-page"><section className="card writing-loading"><p>正在加载写作练习…</p><button type="button" className="btn-ghost" onClick={() => navigate("/writing")}>返回写作库</button></section></main>;
  }

  if (!session) {
    return <main className="page writing-page"><section className="card writing-loading"><h2>无法打开写作练习</h2><p role="alert">{message || "该练习不存在或你没有访问权限。"}</p><div className="row"><button type="button" onClick={() => void loadSession()}>重试加载</button><button type="button" className="btn-ghost" onClick={() => navigate("/writing")}>返回写作库</button></div></section></main>;
  }

  return (
    <main className="page writing-page" aria-busy={isDeleting}>
      <header className="card route-header">
        <div><span className="eyebrow"><FilePenLine size={15} /> Writing practice</span><h2>{session.prompt.title}</h2><p>{session.status === "WRITING" ? "正在写作" : session.status === "EVALUATING" ? "AI 正在评分" : "评分已完成"}</p></div>
        <div className="row">
          <button type="button" className="btn-ghost" disabled={isDeleting} onClick={() => navigate("/writing")}><ArrowLeft size={14} /> 返回写作库</button>
          <button ref={deleteButtonRef} type="button" className="btn-danger" disabled={isDeleting || session.status === "EVALUATING"} onClick={() => void handleDeleteSession()} aria-label={`删除写作练习《${session.prompt.title}》`}><Trash2 size={14} /> {isDeleting ? "删除中…" : "删除练习"}</button>
        </div>
      </header>
      {message ? <p className="field-error" role="alert">{message}</p> : null}
      {isDeleting ? <p role="status" aria-live="polite">正在删除，请稍候…</p> : null}
      {deleteError ? <p className="field-error" role="alert">{deleteError}</p> : null}
      {session.status === "EVALUATING" ? <p className="muted-note">AI 正在评分，评分完成后可删除该练习。</p> : null}
      <section className="writing-workspace">
        <article className="card writing-prompt-card">
          <div className="writing-timer"><Clock3 size={18} /><strong>{session.status === "WRITING" ? formatRemaining(remaining) : session.status === "EVALUATING" ? "评分中" : "已完成"}</strong></div>
          <span>{session.exam}</span><h3>{session.prompt.title}</h3><p>{session.prompt.instructions}</p><small>建议字数：{session.prompt.suggestedWordCount} words · {session.progressMessage}</small>
        </article>
        {session.status === "WRITING" ? <form className="card writing-editor" onSubmit={handleSubmit}>
          <textarea value={essay} disabled={isDeleting} onChange={(event) => setEssay(event.target.value)} placeholder="Write your English essay here…" spellCheck lang="en" />
          <AICreditCostNotice action="WRITING_EVALUATION" />
          <div className="row"><span>{wordCount} words {remaining <= 0 ? "· 已超出计时，请立即提交" : ""}</span><button type="submit" disabled={submitting || isDeleting}>{submitting ? "提交中…" : "提交并开始 AI 评分"}</button></div>
        </form> : null}
        {session.status === "EVALUATING" ? <article className="card writing-loading"><Sparkles size={22} /><h3>AI 正在批改</h3><p>{session.progressMessage}</p></article> : null}
        {session.status === "COMPLETED" && session.evaluation ? <article className="card writing-evaluation">
          <div className="writing-total"><strong>{session.evaluation.overallScore.toFixed(0)}</strong><span>/ 100 总分{session.exam === "IELTS" ? ` · 约 Band ${(session.evaluation.overallScore / 10).toFixed(1)}` : ""}</span></div>
          <div className="writing-score-grid">{[["语法", session.evaluation.grammarScore], ["词汇", session.evaluation.vocabularyScore], ["连贯性", session.evaluation.coherenceScore], ["任务回应", session.evaluation.taskResponseScore]].map(([label, score]) => <div key={String(label)}><span>{label}</span><strong>{Number(score).toFixed(0)}</strong><i><b style={{ width: `${Number(score)}%` }} /></i></div>)}</div>
          <p>{session.evaluation.summary}</p><h4>亮点</h4><ul>{session.evaluation.strengths.map((item) => <li key={item}>{item}</li>)}</ul><h4>优先改进</h4><ul>{session.evaluation.issues.map((item) => <li key={item}>{item}</li>)}</ul><h4>下一步</h4><ul>{session.evaluation.suggestions.map((item) => <li key={item}>{item}</li>)}</ul><blockquote>{session.evaluation.revisedExcerpt}</blockquote>
          <button type="button" className="btn-ghost" onClick={() => navigate("/writing")}>再写一篇</button>
        </article> : null}
      </section>
    </main>
  );
}
