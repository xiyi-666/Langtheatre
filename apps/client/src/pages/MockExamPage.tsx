import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, CheckCircle2, CircleAlert, Clock3, Headphones, History, LoaderCircle, PenLine, Play, RefreshCw, RotateCcw, ScrollText, ShieldCheck } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { beginMockExam, deleteMockExam, finishMockExam, getMockExam, getMockExamGenerationCost, listMockExams, retryMockExam, saveMockExamAnswers, startMockExam, submitMockExamSection } from "../api";
import type { MockExam, MockExamQuestion, MockExamSection } from "../types";
import { resolveAudioUrl } from "../audio";
import { canDeleteMockExam, MOCK_EXAM_LABELS, mockExamETA, mockExamLabel, type MockExamType } from "../mockExam";

type DraftPart = { answers: string[]; responses: string[]; revision: number; savedRevision: number };
type Draft = { id: string; paperVersion: string | null | undefined; parts: Record<string, DraftPart> };
type SaveState = "saved" | "pending" | "saving" | "error";

const STATUS_LABELS: Record<MockExam["status"], string> = {
  GENERATING: "正在生成", READY: "待开始", IN_PROGRESS: "作答中", EVALUATING: "正在评估",
  COMPLETED: "已完成", FAILED: "生成失败", EVALUATION_FAILED: "评估失败",
};

function formatTime(seconds: number) {
  const safe = Number.isFinite(seconds) ? Math.max(0, Math.ceil(seconds)) : 0;
  return `${String(Math.floor(safe / 60)).padStart(2, "0")}:${String(safe % 60).padStart(2, "0")}`;
}

function sectionLabel(section: MockExamSection) {
  return { LISTENING: "听力", READING: "阅读", WRITING: "写作", TRANSLATION: "翻译" }[section.skill];
}

function optionValue(question: MockExamQuestion, option: string) {
  return question.type === "true_false_not_given" || question.type === "word_bank" ? option : option.trim().slice(0, 1);
}

function wordCount(text: string) {
  return text.trim() ? text.trim().split(/\s+/).length : 0;
}

function band(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? value.toFixed(1) : "未提供";
}

function errorMessage(error: unknown, label: string) {
  return error instanceof Error && error.message ? `${label}：${error.message}` : label;
}

function hasChanges(draft: Draft | undefined) {
  return !!draft && Object.values(draft.parts).some((part) => part.revision !== part.savedRevision);
}

function isExpired(exam: MockExam | undefined) {
  if (!exam?.startedAt) return false;
  const deadline = Date.parse(exam.startedAt) + exam.totalDurationSeconds * 1000;
  return Number.isFinite(deadline) && Date.now() >= deadline;
}

function hasCachedDraft(id: string) {
  try {
    const cached = JSON.parse(sessionStorage.getItem(cacheKey(id)) ?? "null");
    return !!cached?.parts && Object.keys(cached.parts).length > 0;
  } catch { return false; }
}

function cacheKey(id: string) { return `ielts-mock-draft:${id}`; }

function persistDraft(draft: Draft) {
  try {
    const parts = Object.fromEntries(Object.entries(draft.parts).filter(([, part]) => part.revision !== part.savedRevision));
    if (Object.keys(parts).length) sessionStorage.setItem(cacheKey(draft.id), JSON.stringify({ paperVersion: draft.paperVersion, parts }));
    else sessionStorage.removeItem(cacheKey(draft.id));
    return true;
  } catch { return false; }
}

function hydrateDraft(exam: MockExam): Draft {
  const parts = Object.fromEntries((exam.sections ?? []).map((part) => [part.key, {
    answers: (part.questions ?? []).map((_, index) => part.answers?.[index] ?? ""),
    responses: (part.writingPrompts ?? []).map((_, index) => part.responses?.[index] ?? ""),
    revision: 0, savedRevision: 0,
  }]));
  // 只恢复同一试卷尚未确认保存的输入，不用缓存覆盖整份服务端快照。
  try {
    const cached = JSON.parse(sessionStorage.getItem(cacheKey(exam.id)) ?? "null");
    if (exam.status === "IN_PROGRESS" && cached?.paperVersion === exam.paperVersion && cached?.parts) {
      for (const [key, part] of Object.entries(parts)) {
        const saved = cached.parts[key];
        if (!saved || !Array.isArray(saved.answers) || !Array.isArray(saved.responses)) continue;
        if (!saved.answers.every((value: unknown) => typeof value === "string") || !saved.responses.every((value: unknown) => typeof value === "string")) continue;
        part.answers = part.answers.map((value, index) => saved.answers[index] ?? value);
        part.responses = part.responses.map((value, index) => saved.responses[index] ?? value);
        part.revision = 1;
      }
    }
  } catch { /* 缓存不可用时仍可恢复服务端已保存的答案。 */ }
  return { id: exam.id, paperVersion: exam.paperVersion, parts };
}

function MockListeningAudio({ section }: { section: MockExamSection }) {
  const [clip, setClip] = useState(0);
  const [error, setError] = useState("");
  const audioRef = useRef<HTMLAudioElement>(null);
  const continueRef = useRef(false);
  const clips = section.audioUrls?.length || 1;
  const src = resolveAudioUrl(section.audioUrls?.[clip] ?? section.audioUrl ?? "");

  function chooseClip(index: number) {
    continueRef.current = false;
    audioRef.current?.pause();
    setError("");
    setClip(index);
  }

  function continuePlayback() {
    if (!continueRef.current || !audioRef.current) return;
    continueRef.current = false;
    void audioRef.current.play().catch(() => setError("浏览器暂停了连续播放，请点击播放按钮继续。"));
  }

  return <div className="mock-audio-player">
    <div className="mock-audio-heading"><span><Headphones size={16} /> 听力音频</span><small aria-live="polite">片段 {clip + 1} / {clips}</small></div>
    {src ? <audio ref={audioRef} controls preload="metadata" src={src} aria-label={`${section.title}，片段 ${clip + 1}`} onCanPlay={continuePlayback} onError={() => { continueRef.current = false; setError("当前片段加载失败，请检查网络后重试。"); }} onEnded={() => {
      if (clip + 1 < clips) { continueRef.current = true; setError(""); setClip((index) => index + 1); }
    }} /> : <p className="field-error" role="alert">当前片段音频缺失，请返回列表后重新打开考试。</p>}
    <div className="mock-audio-actions"><button type="button" className="btn-ghost" disabled={clip === 0} onClick={() => chooseClip(clip - 1)}>上一片段</button><small>播放结束后自动续播下一片段</small><button type="button" className="btn-ghost" disabled={clip + 1 >= clips} onClick={() => chooseClip(clip + 1)}>下一片段</button></div>
    {error && <div className="mock-audio-error"><p className="field-error" role="alert">{error}</p><button type="button" className="btn-ghost" onClick={() => { setError(""); audioRef.current?.load(); }}>重新加载音频</button></div>}
  </div>;
}

export function MockExamPage() {
  const [params, setParams] = useSearchParams();
  const selectedID = params.get("mockExam");
  const [exam, setExam] = useState<MockExam>();
  const [history, setHistory] = useState<MockExam[]>([]);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState("");
  const [loading, setLoading] = useState(false);
  const [opening, setOpening] = useState(!!selectedID);
  const [message, setMessage] = useState("");
  const [pollError, setPollError] = useState("");
  const [storageError, setStorageError] = useState(false);
  const [draft, setDraft] = useState<Draft>();
  const [saveState, setSaveState] = useState<SaveState>("saved");
  const [saveError, setSaveError] = useState("");
  const [sectionIndex, setSectionIndex] = useState(0);
  const [targetBand, setTargetBand] = useState(7);
  const [examType, setExamType] = useState<MockExamType>("IELTS");
  const [historyType, setHistoryType] = useState("ALL");
  const [historyStatus, setHistoryStatus] = useState("ALL");
  const [historyPage, setHistoryPage] = useState(1);
  const [now, setNow] = useState(Date.now());
  const [reload, setReload] = useState(0);
  const [generationCost, setGenerationCost] = useState<number>();
  const [costLoading, setCostLoading] = useState(true);
  const [costError, setCostError] = useState("");
  const [expiredDraftNotice, setExpiredDraftNotice] = useState(false);
  const examRef = useRef<MockExam>();
  const draftRef = useRef<Draft>();
  const mountedRef = useRef(true);
  const busyRef = useRef(false);
  const epochRef = useRef(0);
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>();
  const saveQueueRef = useRef<Promise<void>>(Promise.resolve());
  const timeoutAttemptRef = useRef(new Set<string>());
  const historyRequestRef = useRef(0);

  const acceptExam = useCallback((updated: MockExam) => {
    examRef.current = updated;
    if (mountedRef.current) { setExam(updated); setNow(Date.now()); }
  }, []);

  const loadHistory = useCallback(async (showLoading = true) => {
    const request = ++historyRequestRef.current;
    if (showLoading) setHistoryLoading(true);
    try {
      const items = await listMockExams();
      if (mountedRef.current && request === historyRequestRef.current) { setHistory(items); setHistoryError(""); }
    } catch (error) {
      if (mountedRef.current && request === historyRequestRef.current) setHistoryError(errorMessage(error, "记录加载失败，请重试"));
    } finally { if (mountedRef.current && request === historyRequestRef.current) setHistoryLoading(false); }
  }, []);

  // 列表轮询串行执行；离开列表后由详情轮询接管，不覆盖用户正在输入的答案。
  useEffect(() => {
    if (selectedID) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      if (document.visibilityState !== "hidden" && !busyRef.current) await loadHistory(false);
      if (!stopped) timer = setTimeout(() => { void poll(); }, 5000);
    };
    timer = setTimeout(() => { void poll(); }, 5000);
    return () => { stopped = true; clearTimeout(timer); };
  }, [selectedID, loadHistory]);

  const loadCost = useCallback(async () => {
    setCostLoading(true);
    try {
      const cost = await getMockExamGenerationCost();
      if (mountedRef.current) { setGenerationCost(cost); setCostError(""); }
    } catch (error) {
      if (mountedRef.current) { setGenerationCost(undefined); setCostError(errorMessage(error, "暂时无法确认生成费用")); }
    } finally { if (mountedRef.current) setCostLoading(false); }
  }, []);

  const flushDraft = useCallback((): Promise<void> => {
    clearTimeout(saveTimerRef.current);
    const source = draftRef.current;
    const sourceExam = examRef.current;
    if (!source || sourceExam?.status !== "IN_PROGRESS") return Promise.resolve();
    // 到时后服务端拒绝写入，保留本地草稿并放行交卷/返回，不能等保存成功。
    if (isExpired(sourceExam)) {
      if (hasChanges(source)) {
        persistDraft(source);
        if (mountedRef.current) setExpiredDraftNotice(true);
      }
      return Promise.resolve();
    }
    // 每次排队都读取最新 revision；旧请求的返回不覆盖后续键入内容。
    const task = saveQueueRef.current.catch(() => undefined).then(async () => {
      if (mountedRef.current && draftRef.current === source && hasChanges(source)) setSaveState("saving");
      try {
        while (hasChanges(source)) {
          if (isExpired(sourceExam)) break;
          const entry = Object.entries(source.parts).find(([, part]) => part.revision !== part.savedRevision);
          if (!entry) break;
          const [key, part] = entry;
          const revision = part.revision;
          await saveMockExamAnswers(source.id, key, [...part.answers], [...part.responses]);
          part.savedRevision = revision;
          const persisted = persistDraft(source);
          if (mountedRef.current && draftRef.current === source) setStorageError(!persisted);
        }
        if (mountedRef.current && draftRef.current === source) {
          setSaveState(hasChanges(source) ? "pending" : "saved"); setSaveError("");
          if (hasChanges(source) && isExpired(sourceExam)) setExpiredDraftNotice(true);
        }
      } catch (error) {
        if (isExpired(sourceExam)) {
          persistDraft(source);
          if (mountedRef.current && draftRef.current === source) { setExpiredDraftNotice(hasChanges(source)); setSaveState("pending"); setSaveError(""); }
          return;
        }
        if (mountedRef.current && draftRef.current === source) {
          setSaveState("error");
          setSaveError(errorMessage(error, "答案尚未保存到服务器，请重试"));
        }
        throw error;
      }
    });
    saveQueueRef.current = task;
    void task.catch(() => undefined);
    return task;
  }, []);

  const installExam = useCallback((updated: MockExam) => {
    const restored = hydrateDraft(updated);
    draftRef.current = restored;
    setDraft({ ...restored });
    setSaveState(hasChanges(restored) ? "pending" : "saved");
    setStorageError(false);
    setExpiredDraftNotice(isExpired(updated) && hasCachedDraft(updated.id));
    setSaveError(""); setMessage(""); setPollError("");
    setSectionIndex(Math.max(0, (updated.sections ?? []).findIndex((part) => part.key === updated.currentSection)));
    if (updated.targetBand != null) setTargetBand(updated.targetBand);
    if (updated.exam in MOCK_EXAM_LABELS) setExamType(updated.exam as MockExamType);
    acceptExam(updated);
    if (hasChanges(restored)) saveTimerRef.current = setTimeout(() => { void flushDraft().catch(() => undefined); }, 600);
  }, [acceptExam, flushDraft]);

  useEffect(() => {
    mountedRef.current = true;
    void loadHistory();
    void loadCost();
    return () => {
      mountedRef.current = false;
      clearTimeout(saveTimerRef.current);
      void flushDraft().catch(() => undefined);
    };
  }, [flushDraft, loadHistory, loadCost]);

  useEffect(() => {
    if (selectedID === examRef.current?.id && reload === 0) return;
    let cancelled = false;
    epochRef.current += 1;
    setOpening(!!selectedID);
    setMessage("");
    void (async () => {
      try {
        await flushDraft();
        if (cancelled) return;
        if (!selectedID) { examRef.current = undefined; draftRef.current = undefined; setExam(undefined); setDraft(undefined); return; }
        const loaded = await getMockExam(selectedID);
        if (!cancelled) installExam(loaded);
      } catch (error) {
        if (!cancelled) setMessage(errorMessage(error, "无法打开考试，未保存的输入仍保留，请重试"));
      } finally { if (!cancelled) setOpening(false); }
    })();
    return () => { cancelled = true; };
  }, [selectedID, reload, flushDraft, installExam]);

  const activeID = exam?.id;
  const activeStatus = exam?.status;
  useEffect(() => {
    if (!activeID || !activeStatus || !["GENERATING", "READY", "IN_PROGRESS", "EVALUATING"].includes(activeStatus)) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      const epoch = epochRef.current;
      try {
        if (!busyRef.current) {
          const updated = await getMockExam(activeID);
          if (!cancelled && epoch === epochRef.current && !busyRef.current && examRef.current?.id === activeID) {
            // 轮询只更新服务端状态；输入框始终由本地 draft 驱动。
            acceptExam(updated);
            setPollError("");
          }
        }
      } catch (error) {
        if (!cancelled && epoch === epochRef.current) setPollError(errorMessage(error, "状态同步暂时中断，正在自动重试"));
      } finally { if (!cancelled) timer = setTimeout(() => { void poll(); }, 2000); }
    };
    timer = setTimeout(() => { void poll(); }, 2000);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [activeID, activeStatus, acceptExam]);

  useEffect(() => {
    if (activeStatus !== "IN_PROGRESS") return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [activeID, activeStatus]);

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!hasChanges(draftRef.current)) return;
      void flushDraft().catch(() => undefined);
      event.preventDefault(); event.returnValue = "";
    };
    const onHidden = () => { if (document.visibilityState === "hidden") void flushDraft().catch(() => undefined); };
    window.addEventListener("beforeunload", beforeUnload);
    document.addEventListener("visibilitychange", onHidden);
    return () => { window.removeEventListener("beforeunload", beforeUnload); document.removeEventListener("visibilitychange", onHidden); };
  }, [flushDraft]);

  const runAction = useCallback(async (action: () => Promise<void>) => {
    if (busyRef.current) return;
    busyRef.current = true;
    epochRef.current += 1;
    setLoading(true); setMessage("");
    try { await action(); }
    catch (error) { if (mountedRef.current) setMessage(errorMessage(error, "操作未完成，请重试")); }
    finally { busyRef.current = false; if (mountedRef.current) setLoading(false); }
  }, []);

  const finish = useCallback(async () => {
    const source = examRef.current;
    if (!source || !["IN_PROGRESS", "EVALUATION_FAILED"].includes(source.status)) return;
    await runAction(async () => {
      await flushDraft();
      const updated = await finishMockExam(source.id);
      if (mountedRef.current && examRef.current?.id === source.id) acceptExam(updated);
    });
  }, [acceptExam, flushDraft, runAction]);

  const deadline = exam?.status === "IN_PROGRESS" ? Date.parse(exam.startedAt) + exam.totalDurationSeconds * 1000 : NaN;
  const totalRemaining = Number.isFinite(deadline) ? Math.max(0, Math.ceil((deadline - now) / 1000)) : null;
  useEffect(() => {
    if (totalRemaining !== null && totalRemaining > 0 && totalRemaining <= 2 && hasChanges(draftRef.current) && !busyRef.current) {
      void flushDraft().catch(() => undefined);
    }
  }, [totalRemaining, flushDraft]);
  useEffect(() => {
    if (totalRemaining !== 0 || !activeID || activeStatus !== "IN_PROGRESS" || loading || busyRef.current || timeoutAttemptRef.current.has(activeID)) return;
    timeoutAttemptRef.current.add(activeID);
    void finish();
  }, [activeID, activeStatus, totalRemaining, loading, finish]);

  function selectExam(id?: string) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      if (id) next.set("mockExam", id); else next.delete("mockExam");
      return next;
    }, { replace: true });
  }

  async function removeExam(item: MockExam) {
    if (!canDeleteMockExam(item.status) || !window.confirm(`确定删除这条${mockExamLabel(item.exam)}考试记录吗？试题、作答和成绩会一起删除，无法恢复。`)) return;
    await runAction(async () => {
      if (!await deleteMockExam(item.id)) throw new Error("删除未成功，请刷新后重试");
      try { sessionStorage.removeItem(cacheKey(item.id)); } catch { /* 缓存不影响服务端删除结果。 */ }
      setHistory((previous) => previous.filter((entry) => entry.id !== item.id));
      if (examRef.current?.id === item.id) {
        examRef.current = undefined; draftRef.current = undefined;
        setExam(undefined); setDraft(undefined); selectExam();
      }
      await loadHistory();
    });
  }

  async function backToList() {
    await runAction(async () => {
      await flushDraft();
      examRef.current = undefined; draftRef.current = undefined;
      setExam(undefined); setDraft(undefined); setPollError(""); setExpiredDraftNotice(false); selectExam();
      void loadHistory();
    });
  }

  async function handleStart() {
    if (generationCost === undefined || costLoading) return;
    await runAction(async () => {
      await flushDraft();
      const created = await startMockExam(examType, targetBand);
      if (mountedRef.current) { installExam(created); selectExam(created.id); }
    });
  }

  async function retryCompletedExam() {
    const source = examRef.current;
    if (!source || source.status !== "COMPLETED") return;
    await runAction(async () => {
      const retried = await retryMockExam(source.id);
      if (mountedRef.current) {
        installExam(retried);
        selectExam(retried.id);
        void loadHistory();
      }
    });
  }

  function editPart(key: string, field: "answers" | "responses", index: number, value: string) {
    const source = draftRef.current;
    if (!source || busyRef.current || examRef.current?.status !== "IN_PROGRESS" || isExpired(examRef.current)) return;
    const part = source.parts[key];
    if (!part) return;
    part[field] = part[field].map((previous, position) => position === index ? value : previous);
    part.revision += 1;
    setDraft({ ...source }); setSaveState("pending");
    setStorageError(!persistDraft(source));
    clearTimeout(saveTimerRef.current);
    const remaining = deadline - Date.now();
    saveTimerRef.current = setTimeout(() => { void flushDraft().catch(() => undefined); }, remaining <= 1600 ? 0 : 600);
  }

  async function switchSection(index: number) {
    await runAction(async () => { await flushDraft(); setSectionIndex(index); });
  }

  async function submitSection(event: FormEvent) {
    event.preventDefault();
    const source = examRef.current;
    const part = source?.sections[sectionIndex];
    if (!source || !part || source.status !== "IN_PROGRESS") return;
    if (totalRemaining === 0 || sectionIndex === source.sections.length - 1) { await finish(); return; }
    await runAction(async () => {
      await flushDraft();
      const frontier = source.sections.findIndex((item) => item.key === source.currentSection);
      if (sectionIndex < frontier) { setSectionIndex(sectionIndex + 1); return; }
      const saved = draftRef.current?.parts[part.key];
      if (!saved) throw new Error("答题内容尚未加载，请重新打开考试");
      const updated = await submitMockExamSection(source.id, part.key, saved.answers, saved.responses);
      acceptExam(updated);
      setSectionIndex(Math.max(0, updated.sections.findIndex((item) => item.key === updated.currentSection)));
    });
  }

  const backButton = <button type="button" className="btn-ghost" onClick={() => void backToList()} disabled={loading}><ArrowLeft size={15} /> 返回模拟考试</button>;
  const alerts = <>{message && <p className="field-error" role="alert">{message}</p>}{pollError && <p className="mock-notice" role="status">{pollError}</p>}{expiredDraftNotice && <p className="mock-notice" role="status">考试到时后仅以服务器已保存的答案计分。本地未同步内容未计入本次成绩，已保留在当前浏览器会话中。</p>}</>;
  const costNotice = <div className="mock-cost-notice" role="status">{costLoading ? "正在确认生成费用…" : generationCost !== undefined ? <><strong>{generationCost === 0 ? "本次生成不扣积分" : `本次生成消耗 ${generationCost} 积分`}</strong><span>包含整套试卷、听力音频及本次评估，评估不另收费。</span></> : <><p className="field-error">{costError}</p><button type="button" className="btn-ghost" onClick={() => void loadCost()}>重新查询费用</button></>}</div>;
  const retryCostNotice = <div className="mock-cost-notice" role="status"><strong>重新挑战本套不扣生成积分</strong><span>{costLoading ? "正在确认新试卷生成费用…" : generationCost !== undefined ? generationCost === 0 ? "生成另一套当前不扣积分。" : `生成另一套将消耗 ${generationCost} 积分。` : costError}</span>{generationCost === undefined && !costLoading && <button type="button" className="btn-ghost" onClick={() => void loadCost()}>重新查询费用</button>}</div>;

  if (opening || (!exam && selectedID)) {
    return <main className="page mock-exam-page"><section className="card mock-state-card" aria-busy={opening}>
      {opening ? <LoaderCircle className="mock-spinner" size={32} /> : <CircleAlert size={32} />}
      <h1>{opening ? "正在恢复考试…" : "暂时无法打开考试"}</h1>
      <p>正在读取试卷、已保存答案和考试状态。</p>{alerts}
      <div className="mock-actions">{!opening && <button type="button" onClick={() => setReload((value) => value + 1)}>重新加载</button>}{backButton}</div>
    </section></main>;
  }

  if (!exam) {
    const filteredHistory = history.filter((item) => (historyType === "ALL" || item.exam === historyType) && (historyStatus === "ALL" || item.status === historyStatus));
    const pageCount = Math.max(1, Math.ceil(filteredHistory.length / 8));
    const page = Math.min(historyPage, pageCount);
    const visibleHistory = filteredHistory.slice((page - 1) * 8, page * 8);
    const isIELTS = examType === "IELTS";
    return <main className="page mock-exam-page"><section className="card mock-exam-hero">
      <div className="mock-hero-copy"><span className="eyebrow"><ShieldCheck size={15} /> 模拟考试 · 专项诊断与改进</span>
        <div className="mock-exam-type-tabs" role="group" aria-label="考试类型">{Object.entries(MOCK_EXAM_LABELS).map(([value, label]) => <button type="button" key={value} aria-pressed={examType === value} disabled={loading} className={examType === value ? "active" : ""} onClick={() => setExamType(value as MockExamType)}>{label}</button>)}</div>
        <h1>开始你的测试</h1>
        <p>{isIELTS ? "完成听力、阅读和写作，获得分项 Band 估算与改进建议。口语可单独进入对话训练。" : "完成写作、听力、阅读与翻译，按四六级题型权重计算训练成绩，并获得改进建议。"}</p>
        {isIELTS && <fieldset className="mock-target"><legend>目标 Band</legend><div>{[6, 6.5, 7, 7.5, 8].map((value) => <label key={value}><input type="radio" name="targetBand" value={value} checked={targetBand === value} disabled={loading} onChange={() => setTargetBand(value)} /><span>{value.toFixed(1)}</span></label>)}</div></fieldset>}
        {costNotice}<button type="button" onClick={() => void handleStart()} disabled={loading || costLoading || generationCost === undefined}><Play size={16} /> {loading ? "正在创建考试…" : "生成我的模拟试卷"}</button>
      </div>
      <div className="mock-structure"><div><Headphones size={19} /><strong>听力</strong><span>{isIELTS ? "4 部分 · 40 题 · 建议 30 分钟" : `25 题 · 建议 ${examType === "CET4" ? 25 : 30} 分钟 · 权重 35%`}</span></div><div><ScrollText size={19} /><strong>阅读</strong><span>{isIELTS ? "3 篇文章 · 40 题 · 建议 60 分钟" : "选词填空、匹配、仔细阅读 · 30 题 · 40 分钟 · 权重 35%"}</span></div><div><PenLine size={19} /><strong>{isIELTS ? "写作" : "写作与翻译"}</strong><span>{isIELTS ? "Task 1 + Task 2 · 建议 60 分钟" : "各 30 分钟 · 各占 15%"}</span></div></div>
      <div className="mock-rules"><strong>后台准备，就绪后再开始</strong><span>试卷和听力音频全部就绪后，由你确认开始。全卷限时 {isIELTS ? 150 : examType === "CET4" ? 125 : 130} 分钟，分科时长仅作安排建议。</span><span>生成期间可以离开页面；作答期间自动保存，离开后计时继续。音频可暂停、拖动，用于训练而非正式考场监考。</span><span>{isIELTS ? "训练报告为非官方估算，三技能均分不代表 IELTS 总分。" : "训练成绩采用百分制加权，不等于官方 710 分制常模成绩。"}</span><Link to="/speaking">进入口语对话训练 →</Link></div>{alerts}
    </section><section className="card mock-history"><div className="mock-history-heading"><h2><History size={17} /> 我的模拟考试</h2><button type="button" className="btn-ghost" disabled={historyLoading || loading} onClick={() => void loadHistory()} aria-label="刷新考试记录"><RefreshCw size={15} /></button></div>
      {historyLoading && <p role="status">正在加载记录…</p>}
      {historyError && <p className="field-error" role="alert">{historyError}</p>}
      <div className="mock-history-filters"><label>考试类型<select value={historyType} onChange={(event) => { setHistoryType(event.target.value); setHistoryPage(1); }}><option value="ALL">全部考试</option>{Object.entries(MOCK_EXAM_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label>状态<select value={historyStatus} onChange={(event) => { setHistoryStatus(event.target.value); setHistoryPage(1); }}><option value="ALL">全部状态</option>{Object.entries(STATUS_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><small>共 {filteredHistory.length} 条 · 后台状态自动更新</small></div>
      {!historyLoading && !historyError && !filteredHistory.length && <p className="mock-muted">暂无符合条件的考试。可更换筛选条件或准备一套新试卷。</p>}
      {visibleHistory.map((item) => <div className="mock-history-item" key={item.id}>
        <button type="button" className="mock-history-open" disabled={loading} onClick={() => selectExam(item.id)}><span>{mockExamLabel(item.exam)}{item.targetBand != null && item.exam === "IELTS" ? ` · 目标 ${band(item.targetBand)}` : ""}</span><strong>{STATUS_LABELS[item.status]}{item.status === "COMPLETED" && item.result ? item.exam === "IELTS" ? ` · 三技能均分 ${band(item.result.estimatedBand)}` : ` · 训练得分 ${band(item.result.totalScore)} / 100` : ""}</strong>
        <small>{item.createdAt && Number.isFinite(Date.parse(item.createdAt)) ? new Date(item.createdAt).toLocaleString("zh-CN") : item.startedAt && Number.isFinite(Date.parse(item.startedAt)) ? new Date(item.startedAt).toLocaleString("zh-CN") : "尚未开始计时"}</small>
        {item.status === "GENERATING" && <small>{mockExamETA(item)}</small>}<span className="mock-history-action">{item.status === "READY" ? "进入考试 →" : item.status === "COMPLETED" ? "查看成绩 →" : item.status === "IN_PROGRESS" ? "继续作答 →" : "查看详情 →"}</span>
        </button><button type="button" className="btn-ghost" disabled={loading || !canDeleteMockExam(item.status)} title={canDeleteMockExam(item.status) ? "删除试题、答案和成绩" : "任务或考试进行中，暂不可删除"} onClick={() => void removeExam(item)}>删除</button></div>)}
      {filteredHistory.length > 8 && <nav className="mock-history-pagination" aria-label="考试记录分页"><button type="button" className="btn-ghost" disabled={page <= 1} onClick={() => setHistoryPage(page - 1)}>上一页</button><span>第 {page} / {pageCount} 页</span><button type="button" className="btn-ghost" disabled={page >= pageCount} onClick={() => setHistoryPage(page + 1)}>下一页</button></nav>}
    </section></main>;
  }

  if (exam.status === "COMPLETED" && exam.result) {
    const result = exam.result;
    const isIELTS = exam.exam === "IELTS";
    const evaluations = [...(result.writingEvaluations ?? []), ...(result.translationEvaluation ? [result.translationEvaluation] : [])];
    return <main className="page mock-exam-page"><section className="card mock-result-card">
      <div className="mock-result-heading"><span className="eyebrow"><CheckCircle2 size={15} /> {mockExamLabel(exam.exam)} · 训练完成</span><h1>你的模拟考试报告</h1><p>{isIELTS ? "非官方训练估算。本次未测评口语，不提供 IELTS 总分。" : "按题型权重计算的百分制练习成绩，不是官方 710 分制常模成绩。"}</p></div>
      <div className="mock-band-score"><strong>{band(isIELTS ? result.estimatedBand : result.totalScore)}</strong><span>{isIELTS ? "听力、阅读、写作 · 三技能训练均分" : "加权训练总分 / 100 · 听力35% + 阅读35% + 写作15% + 翻译15%"}</span></div>
      <div className="mock-score-grid"><article><span>听力 {isIELTS ? "Band" : "训练分 / 100"}</span><strong>{band(result.listeningScore)}</strong><small>答对 {result.listeningCorrect} / {result.listeningTotal}</small></article><article><span>阅读 {isIELTS ? "Band" : "训练分 / 100"}</span><strong>{band(result.readingScore)}</strong><small>答对 {result.readingCorrect} / {result.readingTotal}</small></article><article><span>写作 {isIELTS ? "Band" : "训练分 / 100"}</span><strong>{band(isIELTS ? result.writingBand : result.writingScore)}</strong>{isIELTS && <small>Task 1：{band(result.writingTask1Band)}<br />Task 2：{band(result.writingTask2Band)}</small>}</article>{!isIELTS && <article><span>翻译训练分 / 100</span><strong>{band(result.translationScore)}</strong></article>}</div>
      {result.feedback && <p className="mock-feedback">{result.feedback}</p>}
      {evaluations.map((evaluation, index) => <details className="mock-answer-review" key={index}>
        <summary>{isIELTS ? `Task ${index + 1} · 四维评分与原文依据` : `${index === 0 ? "写作" : "翻译"} · 评分与原文依据（百分制）`}</summary>
        <dl className="mock-criterion-scores">{[
          [index === 0 ? "任务完成度" : "任务回应", evaluation.taskResponseScore],
          ["连贯与衔接", evaluation.coherenceScore],
          ["词汇资源", evaluation.vocabularyScore],
          ["语法多样性与准确性", evaluation.grammarScore],
        ].map(([label, score]) => <div key={String(label)}><dt>{label}</dt><dd>{band(typeof score === "number" ? isIELTS ? score * 9 / 100 : score : undefined)}</dd></div>)}</dl>
        <p>{evaluation.summary || evaluation.issues?.join("；")}</p>
        {!!evaluation.evidence?.length && <ul>{evaluation.evidence.map((item, position) => <li key={position}>{item
          .replace("[taskAchievement]", "【任务完成度】").replace("[taskResponse]", "【任务回应】")
          .replace("[coherenceCohesion]", "【连贯与衔接】").replace("[lexicalResource]", "【词汇资源】")
          .replace("[grammaticalRangeAccuracy]", "【语法多样性与准确性】")}</li>)}</ul>}
        {evaluation.revisedExcerpt && <><h3>改写参考</h3><p lang="en">{evaluation.revisedExcerpt}</p></>}
      </details>)}
      <div className="mock-report-columns">{[["表现较好", result.strengths], ["需要改进", result.weaknesses], ["下一步建议", result.recommendations]].map(([title, items]) => <article key={String(title)}><h3>{String(title)}</h3>{Array.isArray(items) && items.length ? <ul>{items.map((item, index) => <li key={index}>{item}</li>)}</ul> : <p className="mock-muted">本次报告未提供此项内容。</p>}</article>)}</div>
      <details className="mock-answer-review"><summary>回看我的作答</summary>{(exam.sections ?? []).map((part) => <article key={part.key}><h3>{part.title}</h3>{(part.questions ?? []).map((question, index) => <div key={index}><p>{question.question}</p><strong>我的答案：{part.answers?.[index] || "未作答"}</strong></div>)}{(part.writingPrompts ?? []).map((prompt, index) => <div key={index}><h4>{prompt.title}</h4><p>{part.responses?.[index] || "未作答"}</p></div>)}</article>)}</details>
      <p><Link to="/speaking">继续口语对话训练 →</Link></p>
      {retryCostNotice}<div className="mock-actions">{backButton}<button type="button" disabled={loading} onClick={() => void retryCompletedExam()}><RotateCcw size={16} /> {loading ? "正在准备重练…" : "重新练习本套题"}</button><button type="button" className="btn-ghost" disabled={loading || costLoading || generationCost === undefined} onClick={() => void handleStart()}>{loading ? "正在处理…" : isIELTS ? `生成另一套 · 目标 ${band(targetBand)}` : `生成另一套${mockExamLabel(exam.exam)}`}</button><button type="button" className="btn-ghost" disabled={loading} onClick={() => void removeExam(exam)}>删除考试</button></div>{alerts}
    </section></main>;
  }

  if (exam.status !== "IN_PROGRESS") {
    const pending = exam.status === "GENERATING" || exam.status === "EVALUATING";
    const failed = exam.status === "FAILED" || exam.status === "EVALUATION_FAILED";
    const ready = exam.status === "READY";
    const title = { GENERATING: "正在准备你的模拟试卷", READY: "试卷已就绪，准备开始", EVALUATING: "正在评估你的作答", FAILED: "试卷生成未完成", EVALUATION_FAILED: "评估暂时未能完成", COMPLETED: "报告暂时不可用", IN_PROGRESS: "" }[exam.status];
    return <main className="page mock-exam-page"><section className={`card mock-state-card ${failed ? "mock-state-error" : ""}`}>
      <span className="eyebrow">{mockExamLabel(exam.exam)}{exam.targetBand != null && exam.exam === "IELTS" ? ` · 目标 Band ${band(exam.targetBand)}` : ""}</span>
      <div className="mock-state-symbol">{pending ? <LoaderCircle className="mock-spinner" size={36} /> : ready ? <CheckCircle2 size={36} /> : <CircleAlert size={36} />}</div>
      <h1>{title}</h1>
      {(pending || failed) && <p className="mock-service-message" role={failed ? "alert" : "status"}>{exam.currentSection || (exam.status === "GENERATING" ? "正在生成题目并准备听力音频，请稍候…" : exam.status === "EVALUATING" ? "正在分析作答并生成训练报告，请稍候…" : "服务暂时不可用，请稍后重试。")}</p>}
      {pending && <div className="mock-wait-track" aria-hidden="true"><span /></div>}
      {exam.status === "GENERATING" && <><p role="status">{mockExamETA(exam)}</p><p>正在后台生成，当前尚未计时。可以关闭本页面或返回列表，稍后选择这套试卷开始考试。</p></>}
      {exam.status === "EVALUATING" && <p>答案已提交，正在等待评估结果。你可以返回列表，稍后查看报告。</p>}
      {exam.status === "EVALUATION_FAILED" && <p>服务端保留了本次作答。可重试评估，无需重新答题。</p>}
      {exam.status === "FAILED" && <p>本次考试尚未开始。返回列表后可重新选择目标并生成试卷。</p>}
      {ready && <><p>点击下方按钮后开始全卷计时。全卷限时 {Math.ceil(exam.totalDurationSeconds / 60)} 分钟，分科时长仅作建议。</p><div className="mock-ready-facts"><span><Headphones size={16} /> 听力音频已完整就绪</span><span><ScrollText size={16} /> 共 {exam.sections.length} 个部分 · {exam.exam === "IELTS" ? "听力、阅读、写作" : "写作、听力、阅读、翻译"}</span><span><Clock3 size={16} /> 开始后离开页面不暂停计时</span></div></>}
      {exam.status === "COMPLETED" && <p>服务端尚未返回报告内容，请重新加载查看。</p>}
      {canDeleteMockExam(exam.status) && <button type="button" className="btn-ghost" disabled={loading} onClick={() => void removeExam(exam)}>删除考试</button>}
      <div className="mock-actions">{ready && <button type="button" disabled={loading} onClick={() => void runAction(async () => { installExam(await beginMockExam(exam.id)); })}><Play size={16} /> {loading ? "正在开始…" : "开始考试 · 启动计时"}</button>}{exam.status === "EVALUATION_FAILED" && <button type="button" disabled={loading} onClick={() => void finish()}><RefreshCw size={16} /> {loading ? "正在提交…" : "重试评估"}</button>}{exam.status === "COMPLETED" && <button type="button" disabled={loading} onClick={() => setReload((value) => value + 1)}>重新加载报告</button>}{backButton}</div>{alerts}
    </section></main>;
  }

  const section = exam.sections?.[sectionIndex];
  if (!section) return <main className="page mock-exam-page"><section className="card mock-state-card"><CircleAlert size={32} /><h1>暂时无法读取试题</h1><p>试卷内容不完整，请重新加载。</p><div className="mock-actions"><button type="button" disabled={loading} onClick={() => setReload((value) => value + 1)}>重新加载</button>{backButton}</div>{alerts}</section></main>;
  const current = draft?.parts[section.key];
  const questions = section.questions ?? [];
  const prompts = section.writingPrompts ?? [];
  const frontier = Math.max(0, exam.sections.findIndex((part) => part.key === exam.currentSection));
  const expired = totalRemaining === 0;
  const locked = loading || expired || !current;
  const completedCount = (current?.answers ?? []).filter((answer) => answer.trim()).length;

  return <main className="page mock-exam-page">
    <div className="mock-session-nav">{backButton}<span className="mock-muted">返回后计时继续</span></div>
    <header className="mock-exam-topbar"><div><span className="eyebrow">{mockExamLabel(exam.exam)} · {sectionLabel(section)}</span><h1>{section.title}</h1></div><div className={`mock-clock ${totalRemaining !== null && totalRemaining < 300 ? "urgent" : ""}`}><Clock3 size={17} /><strong>{totalRemaining === null ? "同步中" : formatTime(totalRemaining)}</strong><small>全卷剩余时间</small></div></header>
    {alerts}{expired && <p className="mock-notice" role="status">全卷时间已结束，作答已锁定。{loading ? "正在保存并交卷…" : "若提交未完成，请点击下方按钮重试。"}</p>}
    {totalRemaining === null && <p className="field-error" role="alert">考试开始时间暂不可用，请刷新状态确认计时。</p>}
    <nav className="mock-exam-progress" aria-label="考试部分"><span>第 {sectionIndex + 1} / {exam.sections.length} 部分</span><div>{exam.sections.map((part, index) => <button key={part.key} type="button" className={index === sectionIndex ? "active" : index < frontier ? "done" : ""} disabled={loading || index > frontier} aria-label={`${part.title}${index > frontier ? "，尚未解锁" : ""}`} aria-current={index === sectionIndex ? "step" : undefined} onClick={() => void switchSection(index)}>{index + 1}</button>)}</div></nav>
    <form className="mock-exam-workspace" onSubmit={(event) => { void submitSection(event); }}><section className="card mock-exam-content">
      <div className="mock-instructions"><strong>{section.instructions}</strong>{section.skill === "LISTENING" && <MockListeningAudio key={`${exam.id}:${section.key}`} section={section} />}</div>
      {section.passage && <article className="mock-passage"><h2>阅读材料</h2><p>{section.passage}</p></article>}
      {!!questions.length && <div className="mock-questions">{questions.map((question, index) => <fieldset key={`${section.key}-${index}`} disabled={locked}><legend>{question.question}</legend>{question.options?.length ? question.options.map((option) => {
        const value = optionValue(question, option);
        return <label key={option}><input type="radio" name={`${section.key}-${index}`} value={value} checked={(current?.answers[index] ?? "") === value} onChange={() => editPart(section.key, "answers", index, value)} /><span>{option}</span></label>;
      }) : <label className="mock-completion-answer"><span>填写答案{question.type === "sentence_completion" ? "（不超过 3 个词，以题干要求为准）" : "（按题干要求填写）"}</span><input type="text" lang="en" autoComplete="off" spellCheck={false} value={current?.answers[index] ?? ""} onChange={(event) => editPart(section.key, "answers", index, event.target.value)} aria-invalid={question.type === "sentence_completion" && wordCount(current?.answers[index] ?? "") > 3} />{question.type === "sentence_completion" && wordCount(current?.answers[index] ?? "") > 3 && <small className="field-error">当前超过 3 个词，请检查答案。</small>}</label>}</fieldset>)}</div>}
      {!!prompts.length && <div className="mock-writing-tasks">{prompts.map((prompt, index) => <label key={`${section.key}-${index}`}><strong>{prompt.title}{section.skill !== "TRANSLATION" && prompt.suggestedWordCount > 0 ? ` · 建议至少 ${prompt.suggestedWordCount} 词` : " · 将原文完整译成英语"}</strong><span>{prompt.instructions}</span><textarea lang="en" spellCheck disabled={locked} value={current?.responses[index] ?? ""} onChange={(event) => editPart(section.key, "responses", index, event.target.value)} placeholder={section.skill === "TRANSLATION" ? "在此输入你的英文翻译…" : "在此输入你的英文作文…"} /><small className="mock-word-count">{wordCount(current?.responses[index] ?? "")} words</small></label>)}</div>}
    </section><aside className="card mock-exam-sidebar"><div><small>当前科目</small><strong>{sectionLabel(section)}</strong></div><div><small>本部分建议用时</small><strong>{formatTime(section.durationSeconds)}</strong><small>仅供安排，全卷统一计时</small></div><div><small>{questions.length ? "本部分已作答" : "写作任务"}</small><strong>{questions.length ? `${completedCount} / ${questions.length} 题` : `${prompts.length} tasks`}</strong></div>
      <p className={`mock-save-state ${saveState === "error" ? "is-error" : ""}`} role="status">{saveState === "saved" ? <CheckCircle2 size={15} /> : saveState === "saving" ? <LoaderCircle className="mock-spinner" size={15} /> : <Clock3 size={15} />}{({ saved: "答案已保存", pending: "等待保存…", saving: "正在保存…", error: "保存未完成" })[saveState]}</p>
      {saveError && <><p className="field-error" role="alert">{saveError}</p><button type="button" className="btn-ghost" disabled={loading} onClick={() => { void flushDraft().catch(() => undefined); }}>重试保存</button></>}
      {storageError && <p className="field-error" role="alert">浏览器暂存不可用，请等待服务器保存成功后再刷新或离开。</p>}
      {expired ? <button type="button" disabled={loading} onClick={() => void finish()}>{loading ? "正在提交…" : "重试交卷"}</button> : <button type="submit" disabled={locked}>{loading ? "正在保存…" : sectionIndex === exam.sections.length - 1 ? "保存并交卷" : "保存并进入下一部分"}</button>}
      <small>交卷后答案不可修改，报告生成后会自动显示。</small>
    </aside></form>
  </main>;
}
