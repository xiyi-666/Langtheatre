import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, CheckCircle2, Circle, Clock3, LogOut, Mic2, RotateCcw, Send, Sparkles } from "lucide-react";
import { abandonSpeakingSession, finishSpeakingSession, getLatestSpeakingSession, getSpeakingSession, startSpeakingSession, submitSpeakingTurn } from "../api";
import type { SpeakingSession } from "../types";
import { recordingToWavDataURL } from "../audioRecorder";
import { resolveAudioUrl } from "../audio";
import { SpeakingPromptCard } from "../components/SpeakingPromptCard";
import { AICreditCostNotice } from "../components/AICreditCostNotice";

const SPEAKING_SESSION_KEY = "ielts-speaking-session-id";
const PROCESSING_STATES = new Set(["PREPARING", "PROCESSING", "EVALUATING"]);
const NON_RESUMABLE_STATES = new Set(["COMPLETED", "ABANDONED", "QUALITY_REVIEW_PENDING"]);
const STATUS_LABELS: Record<string, string> = { PREPARING: "正在准备音频", ACTIVE: "等待回答", PROCESSING: "正在处理回答", TURN_FAILED: "本轮失败，可重试", READY_TO_FINISH: "对话完成，待评估", EVALUATING: "正在评估", EVALUATION_FAILED: "评估失败，可重试", COMPLETED: "已完成", ABANDONED: "已退出", QUALITY_REVIEW_PENDING: "内容暂未开放", PREPARATION_FAILED: "准备失败" };

function scoreLabel(value?: number | null) {
  return typeof value === "number" ? value.toFixed(1) : "无音频证据，不提供";
}

export function SpeakingPage() {
  const [session, setSession] = useState<SpeakingSession | null>(null);
  const [answer, setAnswer] = useState("");
  const [busy, setBusy] = useState(false);
  const [recording, setRecording] = useState(false);
  const [error, setError] = useState("");
  const [recoveryFailed, setRecoveryFailed] = useState(false);
  const [restoring, setRestoring] = useState(true);
  const [audioDraft, setAudioDraft] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const [prepRemaining, setPrepRemaining] = useState<number>();
  const live = useRef(true);
  const actionRef = useRef(false);
  const currentSession = useRef<SpeakingSession | null>(null);
  const restoreAttempt = useRef(0);
  const recordingStarted = useRef(0);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const prompt = useMemo(() => session?.prompts[session.promptIndex], [session]);
  const processing = Boolean(session && PROCESSING_STATES.has(session.status));
  const canAnswer = !!session && ["ACTIVE", "TURN_FAILED"].includes(session.status);

  const rememberSession = useCallback((next: SpeakingSession) => {
    if (!live.current) return;
    const previous = currentSession.current;
    if (previous && (previous.id !== next.id || previous.promptIndex !== next.promptIndex)) {
      setAnswer(""); setAudioDraft(""); setElapsed(0); setPrepRemaining(undefined);
    }
    currentSession.current = next;
    setSession(next);
    try { localStorage.setItem(SPEAKING_SESSION_KEY, next.id); } catch { /* 本地缓存失败不影响服务端会话。 */ }
  }, []);

  const clearSession = useCallback(() => {
    currentSession.current = null;
    setSession(null);
    setAnswer("");
    setAudioDraft("");
    setElapsed(0);
    setPrepRemaining(undefined);
    setError("");
    setRecoveryFailed(false);
    try { localStorage.removeItem(SPEAKING_SESSION_KEY); } catch { /* 忽略不可用的浏览器存储。 */ }
  }, []);

  const refresh = useCallback(async () => {
    const id = currentSession.current?.id;
    if (!id) return;
    try {
      rememberSession(await getSpeakingSession(id));
      setError("");
      setRecoveryFailed(false);
    } catch (e) {
      setError((e as Error).message);
      setRecoveryFailed(true);
    }
  }, [rememberSession]);

  const restoreSession = useCallback(async () => {
    const attempt = ++restoreAttempt.current;
    setRestoring(true);
    setRecoveryFailed(false);
    try {
      let next: SpeakingSession | null = null;
      let cachedID: string | null = null;
      try { cachedID = localStorage.getItem(SPEAKING_SESSION_KEY); } catch { /* 服务端仍可恢复会话。 */ }
      if (cachedID) {
        try {
          next = await getSpeakingSession(cachedID);
          if (NON_RESUMABLE_STATES.has(next.status)) {
            next = null;
            try { localStorage.removeItem(SPEAKING_SESSION_KEY); } catch { /* 忽略不可用的浏览器存储。 */ }
          }
        } catch {
          try { localStorage.removeItem(SPEAKING_SESSION_KEY); } catch { /* 忽略不可用的浏览器存储。 */ }
        }
      }
      if (!next) next = await getLatestSpeakingSession();
      if (!live.current || attempt !== restoreAttempt.current) return;
      if (next) rememberSession(next);
      setError("");
    } catch (e) {
      if (!live.current || attempt !== restoreAttempt.current) return;
      setError((e as Error).message || "无法恢复上次会话，请稍后重试。");
      setRecoveryFailed(true);
    } finally {
      if (live.current && attempt === restoreAttempt.current) setRestoring(false);
    }
  }, [rememberSession]);

  useEffect(() => {
    live.current = true;
    void restoreSession();
    return () => { live.current = false; restoreAttempt.current += 1; };
  }, [restoreSession]);

  useEffect(() => {
    if (!processing) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => { if (!actionRef.current) await refresh(); if (!cancelled) timer = setTimeout(() => void poll(), 2000); };
    timer = setTimeout(() => void poll(), 1000);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [processing, refresh]);

  useEffect(() => () => {
    if (recorderRef.current) { recorderRef.current.onstop = null; if (recorderRef.current.state === "recording") recorderRef.current.stop(); }
    streamRef.current?.getTracks().forEach((track) => track.stop());
    chunksRef.current = [];
  }, []);

  useEffect(() => {
    if (!recording) return;
    const timer = setInterval(() => {
      const seconds = Math.floor((Date.now() - recordingStarted.current) / 1000); setElapsed(seconds);
      if (seconds >= Math.min(120, prompt?.answerSec || 120) && recorderRef.current?.state === "recording") recorderRef.current.stop();
    }, 250);
    return () => clearInterval(timer);
  }, [recording, prompt?.answerSec]);

  useEffect(() => {
    if (!prepRemaining) return;
    const timer = setTimeout(() => setPrepRemaining((value) => Math.max(0, (value ?? 0) - 1)), 1000);
    return () => clearTimeout(timer);
  }, [prepRemaining]);

  async function start() {
    if (actionRef.current) return;
    actionRef.current = true;
    setBusy(true);
    setError("");
    setRecoveryFailed(false);
    try {
      rememberSession(await startSpeakingSession());
      setAnswer("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      actionRef.current = false;
      setBusy(false);
    }
  }

  async function submitText() {
    if (!session || !answer.trim() || processing || actionRef.current) return;
    actionRef.current = true;
    setBusy(true);
    setError("");
    setRecoveryFailed(false);
    try {
      rememberSession(await submitSpeakingTurn(session.id, session.promptIndex, "", answer, "ENGLISH"));
      // 受理不代表成功，题号真正推进后才清空，便于后台失败后重试。
    } catch (e) {
      setError((e as Error).message);
    } finally {
      actionRef.current = false;
      setBusy(false);
    }
  }

  async function submitAudioBlob(type: string) {
    if (!session) return;
    setBusy(true);
    setError("");
    setRecoveryFailed(false);
    try {
      const audio = await recordingToWavDataURL(new Blob(chunksRef.current, { type }), 16000);
      if (live.current) setAudioDraft(audio);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function toggleRecording() {
    if (!session || busy || processing || actionRef.current || !canAnswer) return;
    if (recording) {
      recorderRef.current?.stop();
      return;
    }
    actionRef.current = true; setBusy(true); setError("");
    setRecoveryFailed(false);
    try {
      if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") throw new Error("当前环境不支持录音，请使用 HTTPS 或 localhost 下的新版浏览器。");
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (!live.current) { stream.getTracks().forEach((track) => track.stop()); return; }
      const type = ["audio/webm;codecs=opus", "audio/mp4", "audio/webm"].find((mime) => MediaRecorder.isTypeSupported(mime));
      streamRef.current = stream;
      const recorder = new MediaRecorder(stream, type ? { mimeType: type } : undefined);
      chunksRef.current = [];
      recorder.ondataavailable = (event) => {
        if (event.data.size) chunksRef.current.push(event.data);
      };
      recorder.onerror = () => { stream.getTracks().forEach((track) => track.stop()); if (live.current) { setRecording(false); setError("录音中断，请检查设备后重新录制。"); } };
      recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        streamRef.current = null;
        setRecording(false);
        if (live.current) void submitAudioBlob(recorder.mimeType);
      };
      recorderRef.current = recorder;
      recordingStarted.current = Date.now(); setElapsed(0); setPrepRemaining(undefined);
      recorder.start(1000);
      setRecording(true);
    } catch (e) {
      streamRef.current?.getTracks().forEach((track) => track.stop());
      setError(e instanceof Error && e.message.includes("当前环境") ? e.message : "无法使用麦克风，请检查设备和浏览器权限。");
    } finally { actionRef.current = false; if (live.current) setBusy(false);
    }
  }

  async function finish() {
    if (!session || processing || actionRef.current) return;
    actionRef.current = true;
    setBusy(true);
    setError("");
    setRecoveryFailed(false);
    try {
      rememberSession(await finishSpeakingSession(session.id));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      actionRef.current = false;
      setBusy(false);
    }
  }

  async function leaveSession() {
    const active = currentSession.current;
    if (!active || actionRef.current || !window.confirm("确认结束并退出本次口语会话吗？已提交的回答会保留，但本次会话不能继续。")) return;
    actionRef.current = true;
    setBusy(true);
    setError("");
    try {
      await abandonSpeakingSession(active.id);
      clearSession();
    } catch (e) {
      setError((e as Error).message || "退出口语会话失败，请稍后重试。");
    } finally {
      actionRef.current = false;
      if (live.current) setBusy(false);
    }
  }

  async function sendAudio() {
    if (!session || !audioDraft || actionRef.current || processing) return;
    actionRef.current = true; setBusy(true); setError(""); setRecoveryFailed(false);
    try { rememberSession(await submitSpeakingTurn(session.id, session.promptIndex, audioDraft, "", "ENGLISH")); }
    catch (e) { setError((e as Error).message || "提交失败，请重试，录音已保留。"); }
    finally { actionRef.current = false; setBusy(false); }
  }
  const canFinish = Boolean(session && session.turns.length >= session.prompts.length && !processing);
  const currentStatus = STATUS_LABELS[session?.status || ""] || "请刷新状态";
  const scoreCards = session?.evaluation ? [
    ["文字连贯性", session.evaluation.textCoherence],
    ["词汇资源", session.evaluation.lexicalResource],
    ["语法准确度", session.evaluation.grammarAccuracy],
  ] as const : [];
  const unassessedScores = session?.evaluation ? [
    ["发音", session.evaluation.pronunciation],
    ["语音流利度", session.evaluation.fluencyCoherence],
    ["整体 IELTS 分", session.evaluation.overallBand],
  ] as const : [];

  return (
    <main className="page speaking-workspace">
      <section className="card speaking-page" aria-labelledby="speaking-title">
        <header className="speaking-header">
          <div>
            <p className="eyebrow">IELTS SPEAKING</p>
            <h1 id="speaking-title">口语模拟面试</h1>
            <p className="speaking-intro">内置题可用于练习；不声称来自真实当季题库。AI 考官会根据你的回答继续出题和追问，帮助你完成一场完整的口语模拟面试。</p>
          </div>
          <Mic2 size={30} aria-hidden="true" />
        </header>

        {(error || session?.lastError) ? (
          <div className="speaking-alert" role="alert">
            <AlertCircle size={18} aria-hidden="true" />
            <div>
              <strong>{error || session?.lastError}</strong>
              {recoveryFailed ? <p>恢复失败，当前页面中的未提交内容仍会保留。可以重试，或开始新一轮。</p> : null}
            </div>
            {recoveryFailed ? <button type="button" className="btn-ghost" onClick={() => void restoreSession()} disabled={busy || recording || restoring}>重试恢复会话</button> : null}
          </div>
        ) : null}

        {!session ? (
          <section className="speaking-empty" aria-labelledby="speaking-empty-title">
            <Sparkles size={28} aria-hidden="true" />
            <h2 id="speaking-empty-title">开始口语模拟</h2>
            <p>AI 考官会为你安排题目并根据回答继续追问。你可以使用录音回答，也可以使用文字回答进行练习。</p>
            {restoring ? <p className="speaking-inline-status" aria-live="polite">正在恢复上次会话…</p> : null}
            <button type="button" aria-label="开始口语模拟" onClick={() => void start()} disabled={busy || restoring}>
              <Sparkles size={16} aria-hidden="true" /> {busy ? "正在准备题目…" : "开始口语模拟"}
            </button>
          </section>
        ) : (
          <div className="speaking-layout">
            <div className="speaking-main">
              {session.status === "COMPLETED" ? (
                <section className="speaking-report" aria-labelledby="speaking-report-title">
                  <div className="speaking-report-heading">
                    <div>
                      <p className="eyebrow">SESSION REPORT</p>
                      <h2 id="speaking-report-title">评估结果</h2>
                    </div>
                    {session.evaluation?.isPartial ? <span className="speaking-status-badge">部分文本评估</span> : <span className="speaking-status-badge is-success">已完成</span>}
                  </div>
                  <div className="speaking-report-summary">
                    <strong>总结</strong>
                    <p>{session.evaluation?.summary || "本次会话已完成，暂时没有可显示的总结。"}</p>
                  </div>
                  {session.evaluation?.isPartial ? <p className="muted-note">本次为部分文本评估：无可核验音频证据时，不返回发音、语音流利或整体 IELTS 分。</p> : null}
                  <section aria-labelledby="evaluated-title">
                    <h3 id="evaluated-title">已评估维度</h3>
                    <dl className="speaking-score-grid">
                      {scoreCards.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{scoreLabel(value)}</dd><small>基于文字回答</small></div>)}
                    </dl>
                  </section>
                  <section className="speaking-unassessed" aria-labelledby="unassessed-title">
                    <h3 id="unassessed-title">未评估维度</h3>
                    <dl className="speaking-score-grid">
                      {unassessedScores.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{scoreLabel(value)}</dd><small>无音频证据，不提供</small></div>)}
                    </dl>
                  </section>
                  <section className="speaking-feedback" aria-labelledby="feedback-title">
                    <h3 id="feedback-title">改进建议</h3>
                    {session.evaluation?.improvements?.length ? <ul>{session.evaluation.improvements.map((item, index) => <li key={index}>{item}</li>)}</ul> : <p className="muted-note">暂无改进建议。</p>}
                  </section>
                  <details className="speaking-evidence">
                    <summary>查看回答原文依据</summary>
                    {session.evaluation?.evidence?.length ? <ul>{session.evaluation.evidence.map((item, index) => <li key={index}>{item}</li>)}</ul> : <p className="muted-note">暂无原文依据。</p>}
                  </details>
                </section>
              ) : (
                <>
                  <SpeakingPromptCard prompt={prompt} />
                  <section className="speaking-answer" aria-labelledby="speaking-answer-title">
                    <div className="speaking-section-heading">
                      <div>
                        <p className="eyebrow">YOUR TURN</p>
                        <h2 id="speaking-answer-title">当前作答</h2>
                      </div>
                      <span className="speaking-status-badge">{currentStatus}</span>
                    </div>
                    <div className="speaking-stage" aria-live="polite">
                      {processing ? (
                        <>
                          <strong>正在处理回答</strong>
                          <p>后台处理中，可以稍后回来。未提交的录音与文字仅保留在当前页面。</p>
                        </>
                      ) : recording ? (
                        <>
                          <strong>正在录音</strong>
                          <p role="timer">已录制 {elapsed} 秒 / {Math.min(120, prompt?.answerSec || 120)} 秒</p>
                        </>
                      ) : (
                        <>
                          <strong>{prompt ? `录音上限 ${Math.min(120, prompt.answerSec || 120)} 秒` : "本轮题目已完成"}</strong>
                          <p>停止后先试听，再确认提交。文字回答仅用于备用练习。</p>
                          {prompt?.preparationSec ? <div className="speaking-prep-row">
                            <button type="button" className="btn-ghost" disabled={busy || recording} onClick={() => setPrepRemaining(prompt.preparationSec)}>开始 {prompt.preparationSec} 秒准备</button>
                            {prepRemaining !== undefined ? <span aria-live="polite">{prepRemaining > 0 ? `准备剩余 ${prepRemaining} 秒` : "准备完成，可以开始作答"}</span> : null}
                          </div> : null}
                        </>
                      )}
                    </div>

                    {audioDraft && !recording ? <div className="speaking-preview">
                      <div className="speaking-preview-heading"><CheckCircle2 size={18} aria-hidden="true" /><strong>录音已准备好</strong></div>
                      <audio controls aria-label="试听我的回答" src={resolveAudioUrl(audioDraft)} />
                      <button type="button" disabled={busy || processing} onClick={() => void sendAudio()}>确认提交录音</button>
                    </div> : null}

                    {!processing && !audioDraft ? <div className="speaking-primary-actions">
                      <button type="button" onClick={() => void toggleRecording()} disabled={busy || processing || !prompt || !canAnswer}>
                        <Mic2 size={16} aria-hidden="true" /> {recording ? "结束录音" : "录音回答"}
                      </button>
                    </div> : null}

                    <details className="speaking-text-fallback" open={Boolean(answer && error)}>
                      <summary>改用文字回答（备用练习方式）</summary>
                      <p className="muted-note">文字输入仅用于练习备用，不会产生发音、语音流利度或完整 IELTS 分。</p>
                      <label htmlFor="speaking-answer">英文回答</label>
                      <textarea id="speaking-answer" aria-label="英文回答" maxLength={12000} value={answer} onChange={(event) => setAnswer(event.target.value)} placeholder="输入你的英文回答，提交后会保留草稿直到本轮成功处理。" rows={7} disabled={busy || processing || recording || !prompt || !canAnswer} />
                      <button type="button" className="btn-warm" aria-label="提交文字" onClick={() => void submitText()} disabled={busy || processing || recording || !answer.trim() || !prompt || !canAnswer}>
                        <Send size={16} aria-hidden="true" /> {busy || processing ? "处理中…" : "提交文字"}
                      </button>
                    </details>
                    {prompt && <AICreditCostNotice action="ROLEPLAY_TURN" />}
                  </section>
                </>
              )}

              {!!session.turns.length && <details className="speaking-transcript">
                <summary>查看本次对话记录</summary>
                {session.turns.map((turn) => <article key={turn.promptIndex}><h3>第 {turn.promptIndex + 1} 题 · Part {turn.part}</h3><p lang="en">考官：{turn.prompt}</p><p lang="en">我的回答：{turn.transcript}</p></article>)}
              </details>}
            </div>

            <aside className="speaking-sidebar" aria-label="口语模拟进度">
              <section className="speaking-progress-card">
                <div className="speaking-sidebar-heading"><div><p className="eyebrow">PROGRESS</p><h2>口语模拟进度</h2></div><Clock3 size={20} aria-hidden="true" /></div>
                <strong className="speaking-progress-count">已完成 {session.turns.length} / {session.prompts.length} 题</strong>
                <ol className="speaking-progress-list">
                  {session.prompts.map((item, index) => {
                    const done = index < session.turns.length;
                    const current = index === session.promptIndex && session.status !== "COMPLETED";
                    return <li key={`${item.part}-${index}`} className={`speaking-progress-item ${done ? "is-done" : ""} ${current ? "is-current" : ""}`} aria-current={current ? "step" : undefined}><span>{done ? <CheckCircle2 size={16} aria-hidden="true" /> : current ? <Circle size={16} aria-hidden="true" /> : <span className="speaking-progress-dot" aria-hidden="true" />}</span><strong>第 {index + 1} 题</strong><small>{done ? "已完成" : current ? "当前" : "待完成"}</small></li>;
                  })}
                </ol>
              </section>
              <section className="speaking-status-card" aria-live="polite">
                <p className="eyebrow">SESSION STATUS</p>
                <strong>{currentStatus}</strong>
                {session.processingMessage ? <p className="muted-note">{session.processingMessage}</p> : null}
                {session.status === "PREPARATION_FAILED" ? <button type="button" disabled={busy} onClick={() => void start()}>重新准备口语对话</button> : null}
              </section>
              {session.status === "COMPLETED" ? <section className="speaking-session-actions">
                <button type="button" onClick={() => void start()} disabled={busy}><Sparkles size={16} aria-hidden="true" /> 开始新一轮对话</button>
              </section> : <section className="speaking-session-actions">
                <h2>会话操作</h2>
                <button type="button" className="btn-ghost" onClick={() => void refresh()} disabled={busy || recording}><RotateCcw size={16} aria-hidden="true" /> 恢复会话</button>
                <button type="button" className={canFinish ? "" : "btn-ghost"} aria-label="提交并评估" onClick={() => void finish()} disabled={busy || !canFinish}>提交并评估</button>
                {!canFinish ? <p className="muted-note">完成全部题目后可提交评估</p> : <AICreditCostNotice action="WRITING_EVALUATION" />}
                <button type="button" className="btn-ghost speaking-exit-button" onClick={() => void leaveSession()} disabled={busy || recording}><LogOut size={16} aria-hidden="true" /> 结束并退出本次会话</button>
              </section>}
            </aside>
          </div>
        )}
      </section>
    </main>
  );
}
