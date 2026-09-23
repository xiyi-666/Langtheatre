import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, CheckCircle2, Headphones, LoaderCircle, Play, RefreshCw, Search, Trash2 } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { beginListeningTraining, deleteListeningTraining, finishListeningTraining, getListeningGenerationCost, getListeningTraining, listListeningTrainings, saveListeningAnswers, startListeningTraining } from "../api";
import type { ListeningTraining } from "../types";
import { resolveAudioUrl } from "../audio";
import { mockExamETA } from "../mockExam";
import { useAppStore } from "../store";
import "./ListeningPage.css";

const PARTS = [
  { title: "生活对话", description: "两人交流 · 关注姓名、时间与具体信息", icon: "01" },
  { title: "生活独白", description: "单人介绍 · 跟随说明与信息顺序", icon: "02" },
  { title: "学术讨论", description: "多人讨论 · 区分观点、态度与干扰信息", icon: "03" },
  { title: "学术讲座", description: "单人讲授 · 理解结构、要点与同义替换", icon: "04" },
];
const STATUS: Record<ListeningTraining["status"], string> = { GENERATING: "后台生成中", READY: "可以开始", IN_PROGRESS: "练习中", COMPLETED: "已完成", FAILED: "生成失败" };
const TYPE_LABELS: Record<string, string> = { multiple_choice: "选择题", sentence_completion: "填空题", matching_information: "匹配题" };
const draftKey = (id: string) => `listening-draft-v1:${id}`;

function ErrorDialog({ message, close }: { message: string; close: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => { if (message && !ref.current?.open) ref.current?.showModal(); }, [message]);
  return <dialog ref={ref} className="listening-dialog" aria-labelledby="listening-error-title" onCancel={close} onClose={close}>
    <h2 id="listening-error-title">操作未完成</h2><p>{message}</p>
    <button type="button" onClick={() => { ref.current?.close(); close(); }}>我知道了</button>
  </dialog>;
}

export function ListeningPage() {
  const { id } = useParams();
  return <main className="page listening-workspace"><section className="card listening-page">
    <header className="listening-header"><div><p className="eyebrow">IELTS · LISTENING PRACTICE</p><h1>雅思听力</h1><p>每次专注一个 Part。先听、作答，再用原文复盘。</p></div><span className="listening-emblem"><Headphones size={28} aria-hidden="true" /></span></header>
    {id ? <ListeningDetail key={id} id={id} /> : <ListeningLibrary />}
  </section></main>;
}

function ListeningLibrary() {
  const navigate = useNavigate();
  const [part, setPart] = useState(1);
  const [band, setBand] = useState(6.5);
  const [items, setItems] = useState<ListeningTraining[]>([]);
  const [cost, setCost] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const action = useRef(false);
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(1);
  const [reload, setReload] = useState(0);
  useEffect(() => {
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    const load = async () => {
      try {
        const [records, amount] = await Promise.all([listListeningTrainings(), getListeningGenerationCost()]);
        if (!live) return;
        setItems(records); setCost(amount);
        if (records.some(record => record.status === "GENERATING")) timer = setTimeout(() => void load(), 3500);
      } catch (e) { if (live) setError((e as Error).message || "听力列表加载失败，请重试"); }
      finally { if (live) setLoading(false); }
    };
    void load();
    return () => { live = false; clearTimeout(timer); };
  }, [reload]);
  async function create() {
    if (action.current || cost === null) return;
    action.current = true; setBusy(true);
    try { const record = await startListeningTraining(part, band); navigate(`/listening/${record.id}`); }
    catch (e) { setError((e as Error).message || "生成请求失败，请重试"); }
    finally { action.current = false; setBusy(false); }
  }
  async function remove(item: ListeningTraining) {
    if (action.current || !window.confirm(`删除这次 Part ${item.part} 听力训练及作答记录？`)) return;
    action.current = true; setBusy(true);
    try {
      await deleteListeningTraining(item.id);
      localStorage.removeItem(draftKey(item.id));
      setItems(current => current.filter(record => record.id !== item.id));
      setReload(value => value + 1);
    } catch (e) { setError((e as Error).message || "删除失败，请重试"); }
    finally { action.current = false; setBusy(false); }
  }
  const filtered = items.filter(item => (!filter || String(item.part) === filter) && `${item.title || ""} Part ${item.part} ${STATUS[item.status]}`.toLowerCase().includes(search.trim().toLowerCase())).sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  const pages = Math.max(1, Math.ceil(filtered.length / 6));
  const currentPage = Math.min(page, pages);
  return <>
    <section className="listening-generator" aria-labelledby="listening-generator-title">
      <div className="listening-section-heading"><h2 id="listening-generator-title">今天练哪一部分？</h2><span>每次 10 题</span></div>
      <div className="listening-parts" role="group" aria-label="选择听力部分">{PARTS.map((item, index) => <button type="button" key={item.icon} className={part === index + 1 ? "listening-part is-selected" : "listening-part"} aria-pressed={part === index + 1} onClick={() => setPart(index + 1)} disabled={busy}><span>PART {item.icon}</span><strong>{item.title}</strong><small>{item.description}</small></button>)}</div>
      <div className="listening-generate-actions"><label>目标难度<select value={band} onChange={e => setBand(Number(e.target.value))} disabled={busy}>{[6, 6.5, 7, 7.5, 8].map(value => <option key={value} value={value}>{value.toFixed(1)}</option>)}</select></label><p>{cost === null ? "正在确认生成费用…" : cost === 0 ? "本次生成不消耗 AI 点数" : `本次生成消耗 ${cost} AI 点数；失败自动退回`}</p><button type="button" onClick={() => void create()} disabled={busy || cost === null}>{busy ? <LoaderCircle size={17} className="listening-spin" /> : <Headphones size={17} />} {busy ? "正在处理…" : "生成听力训练"}</button></div>
      <p className="muted-note">原创 AI 练习，生成后经过题目审核和音频制作；可离开页面稍后回来。目标难度不是官方校准分数，单段训练不换算雅思 Band。</p>
    </section>
    <section aria-labelledby="listening-library-title"><div className="listening-section-heading"><h2 id="listening-library-title">我的听力训练</h2><button type="button" className="btn-ghost" disabled={busy} onClick={() => setReload(value => value + 1)}><RefreshCw size={15} /> 刷新列表</button></div>
      <div className="listening-filters"><label className="listening-search"><Search size={17} aria-hidden="true" /><input aria-label="搜索听力训练" placeholder="搜索标题、Part 或状态" value={search} onChange={e => { setSearch(e.target.value); setPage(1); }} /></label><select aria-label="筛选听力部分" value={filter} onChange={e => { setFilter(e.target.value); setPage(1); }}><option value="">全部 Part</option>{PARTS.map((_, i) => <option key={i} value={i + 1}>Part {i + 1}</option>)}</select></div>
      {loading ? <p role="status">正在加载听力训练…</p> : filtered.length === 0 ? <div className="listening-empty"><Headphones size={32} /><h3>{items.length ? "没有匹配的训练" : "从一段听力开始"}</h3><p>{items.length ? "试试调整搜索词或筛选条件。" : "选择上方 Part 创建训练，生成完成后即可开始。"}</p></div> : <div className="listening-library-grid">{filtered.slice((currentPage - 1) * 6, currentPage * 6).map(item => <article key={item.id} className="listening-record"><div className="listening-section-heading"><span className="listening-tag">Part {item.part}</span><span className={`listening-status is-${item.status.toLowerCase()}`}>{STATUS[item.status]}</span></div><h3>{item.title || PARTS[item.part - 1]?.title || "听力训练"}</h3><p className="muted-note">目标难度 {item.targetBand.toFixed(1)} · {new Date(item.createdAt).toLocaleDateString("zh-CN")}</p>{item.status === "GENERATING" ? <p className="muted-note">{mockExamETA(item)}</p> : null}{item.status === "COMPLETED" ? <p className="listening-record-score">答对 <strong>{item.correct}/{item.total}</strong> · 正确率 {item.accuracy?.toFixed(0)}%</p> : null}<div className="listening-record-actions"><Link className="listening-link" to={`/listening/${item.id}`}>{item.status === "COMPLETED" ? "查看复盘" : item.status === "IN_PROGRESS" ? "继续训练" : item.status === "READY" ? "进入训练" : "查看状态"}</Link><button type="button" className="btn-ghost" aria-label={`删除听力训练 ${item.title || `Part ${item.part}`}`} disabled={busy || item.status === "GENERATING"} onClick={() => void remove(item)}><Trash2 size={16} /></button></div></article>)}</div>}
      {pages > 1 ? <nav className="listening-pagination" aria-label="听力列表分页"><button type="button" className="btn-ghost" disabled={currentPage === 1} onClick={() => setPage(currentPage - 1)}>上一页</button><span>{currentPage} / {pages}</span><button type="button" className="btn-ghost" disabled={currentPage === pages} onClick={() => setPage(currentPage + 1)}>下一页</button></nav> : null}
    </section><ErrorDialog message={error} close={() => setError("")} />
  </>;
}

function ListeningAudio({ urls }: { urls: string[] }) {
  const [index, setIndex] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [error, setError] = useState("");
  const audio = useRef<HTMLAudioElement>(null);
  const continuePlaying = useRef(false);
  useEffect(() => {
    const player = audio.current;
    if (!player) return;
    player.playbackRate = speed;
    if (continuePlaying.current) {
      continuePlaying.current = false;
      void player.play().catch(() => setError("连续播放被浏览器暂停，请点击播放继续。"));
    }
  }, [index, speed]);
  function changeClip(next: number) { audio.current?.pause(); continuePlaying.current = false; setError(""); setIndex(next); }
  return <section className="listening-player" aria-label="听力播放器"><div className="listening-section-heading"><strong><Headphones size={17} /> 听力音频</strong><span>语音片段 {index + 1} / {urls.length}</span></div><audio ref={audio} controls preload="none" aria-label="播放听力音频" src={resolveAudioUrl(urls[index] || "")} onError={() => setError("音频暂时无法播放，请检查网络后重试。")} onEnded={() => { if (index + 1 < urls.length) { continuePlaying.current = true; setIndex(index + 1); } }} /><div className="listening-player-actions"><button type="button" className="btn-ghost" disabled={index === 0} onClick={() => changeClip(index - 1)}>上一片段</button><button type="button" className="btn-ghost" disabled={index + 1 >= urls.length} onClick={() => changeClip(index + 1)}>下一片段</button><label>播放速度<select aria-label="播放速度" value={speed} onChange={e => setSpeed(Number(e.target.value))}>{[0.75, 1, 1.25, 1.5, 2].map(value => <option key={value} value={value}>{value}×</option>)}</select></label></div><p className="muted-note">首次打开不自动播放；开始后按片段连续播放，可暂停、拖动或反复收听。</p>{error ? <p role="alert">{error} <button type="button" className="btn-ghost" onClick={() => { audio.current?.load(); setError(""); }}>重新加载音频</button></p> : null}</section>;
}

function ListeningDetail({ id }: { id: string }) {
  const [record, setRecord] = useState<ListeningTraining | null>(null);
  const [answers, setAnswers] = useState<string[]>([]);
  const answersRef = useRef<string[]>([]);
  const persisted = useRef("");
  const pendingSave = useRef<Promise<void> | null>(null);
  const live = useRef(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const action = useRef(false);
  const [reload, setReload] = useState(0);
  const [saveMessage, setSaveMessage] = useState("");
  const [wrongOnly, setWrongOnly] = useState(false);
  useEffect(() => { live.current = true; return () => { live.current = false; }; }, []);
  const hydrate = useCallback((next: ListeningTraining) => {
    setRecord(next);
    if (next.status !== "IN_PROGRESS" && next.status !== "COMPLETED") return;
    const saved = next.questions.map((_, i) => next.answers?.[i] || "");
    persisted.current = JSON.stringify(saved);
    let restored = saved;
    try {
      if (next.status === "COMPLETED") localStorage.removeItem(draftKey(id));
      else {
        const raw = localStorage.getItem(draftKey(id));
        const cached: unknown = raw ? JSON.parse(raw) : null;
        if (Array.isArray(cached) && cached.length === saved.length && cached.every(value => typeof value === "string" && value.length <= 200)) restored = cached;
      }
    } catch { setSaveMessage("无法读取本地草稿，已恢复服务端答案。"); }
    answersRef.current = restored; setAnswers(restored);
  }, [id]);
  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const load = async () => {
      try {
        const next = await getListeningTraining(id);
        if (!active) return;
        hydrate(next);
        if (next.status === "GENERATING") timer = setTimeout(() => void load(), 2500);
      } catch (e) { if (active) setError((e as Error).message || "暂时无法打开听力训练"); }
    };
    void load();
    return () => { active = false; clearTimeout(timer); };
  }, [id, reload, hydrate]);
  const save = useCallback(async () => {
    while (pendingSave.current) await pendingSave.current;
    if (!live.current || action.current) return;
    const snapshot = [...answersRef.current];
    const signature = JSON.stringify(snapshot);
    if (signature === persisted.current) return;
    const work = (async () => {
      setSaveMessage("正在保存答案…");
      try {
        await saveListeningAnswers(id, snapshot);
        persisted.current = signature;
        if (live.current) setSaveMessage(JSON.stringify(answersRef.current) === signature ? "答案已保存" : "最新修改等待保存");
      } catch { if (live.current) setSaveMessage("保存失败，当前答案仍保留。请重试保存；提交时会再次发送全部答案。"); }
    })();
    pendingSave.current = work;
    await work;
    if (pendingSave.current === work) pendingSave.current = null;
  }, [id]);
  useEffect(() => {
    if (record?.status !== "IN_PROGRESS" || busy) return;
    const timer = setTimeout(() => void save(), 800);
    return () => clearTimeout(timer);
  }, [answers, busy, record?.status, save]);
  function edit(index: number, value: string) {
    const next = [...answersRef.current]; next[index] = value;
    answersRef.current = next; setAnswers(next);
    try { localStorage.setItem(draftKey(id), JSON.stringify(next)); setSaveMessage("本地草稿已保留，等待同步"); }
    catch { setSaveMessage("本地空间不足，请保持页面打开并保存答案。"); }
  }
  async function begin() {
    if (action.current) return; action.current = true; setBusy(true);
    try { hydrate(await beginListeningTraining(id)); }
    catch (e) { setError((e as Error).message || "暂时无法开始听力训练"); }
    finally { action.current = false; setBusy(false); }
  }
  async function finish() {
    if (action.current || !record) return;
    const blank = answersRef.current.filter(answer => !answer.trim()).length;
    if (!window.confirm(blank ? `还有 ${blank} 题未作答，将按错误计入本次结果。确认提交？` : "提交后显示答案与听力原文，本次作答将不能修改。确认提交？")) return;
    action.current = true; setBusy(true);
    try {
      await pendingSave.current;
      const next = await finishListeningTraining(id, [...answersRef.current]);
      if (!live.current) return;
      hydrate(next); setSaveMessage("");
      void useAppStore.getState().refreshUserXP();
    } catch (e) { if (live.current) setError((e as Error).message || "提交失败，答案已保留，请重试"); }
    finally { action.current = false; if (live.current) setBusy(false); }
  }
  const completed = record?.status === "COMPLETED";
  const attempted = answers.filter(answer => answer.trim()).length;
  return <>
    <div className="listening-detail-nav"><Link to="/listening"><ArrowLeft size={16} /> 听力训练列表</Link>{record ? <span>Part {record.part} · {STATUS[record.status]}</span> : null}</div>
    {record?.status === "GENERATING" ? <button type="button" className="btn-ghost" onClick={() => setReload(value => value + 1)}><RefreshCw size={15} /> 刷新生成状态</button> : null}
    {!record ? <div className="listening-empty"><p>{error ? "加载未完成，可返回听力训练列表或重试。" : "正在打开听力训练…"}</p><button type="button" className="btn-ghost" onClick={() => setReload(value => value + 1)}>重新加载</button><Link className="listening-link" to="/listening">返回听力训练列表</Link></div> : null}
    {record && ["GENERATING", "FAILED", "READY"].includes(record.status) ? <section className="listening-state" role="status">{record.status === "GENERATING" ? <LoaderCircle className="listening-spin" size={32} /> : <Headphones size={32} />}<h2>{record.status === "READY" ? "听力已准备好" : record.status === "FAILED" ? "本次生成未完成" : "正在准备你的听力训练"}</h2><p>{record.status === "READY" ? `${PARTS[record.part - 1].title} · 10题，可暂停、重听，不强制限时。` : record.message}</p>{record.status === "GENERATING" ? <><div className="listening-wait"><span /></div><p>{mockExamETA(record)}</p><p className="muted-note">任务在后台继续，回到列表不会中断生成。</p></> : null}{record.status === "READY" ? <button type="button" disabled={busy} onClick={() => void begin()}><Play size={16} /> {busy ? "正在打开…" : "开始听力训练"}</button> : <Link className="listening-link" to="/listening">返回听力训练列表</Link>}{record.status === "FAILED" ? <p className="muted-note">未提供替代题目或评分。可以返回列表重新创建。</p> : null}</section> : null}
    {record && (record.status === "IN_PROGRESS" || completed) ? <>
      <div className="listening-section-heading"><div><span className="listening-tag">PART {record.part}</span><h2 lang="en">{record.title}</h2></div><span>{completed ? "训练复盘" : `${attempted} / ${record.questions.length} 已作答`}</span></div>
      {completed ? <section className="listening-result" aria-label="听力训练结果"><div><span>答对题数</span><strong>{record.correct} <small>/ {record.total}</small></strong></div><div><span>正确率</span><strong>{record.accuracy?.toFixed(0)}<small>%</small></strong></div><p>{record.feedback}</p>{record.recommendations?.length ? <ul>{record.recommendations.map((text, index) => <li key={index}>{text}</li>)}</ul> : null}</section> : null}
      <div className="listening-training-layout"><div className="listening-audio-column"><ListeningAudio urls={record.audioUrls || []} /><div className="listening-answer-nav" aria-label="题号导航">{record.questions.map((q, index) => <a key={index} href={`#listening-question-${index}`} className={completed ? q.correct ? "is-correct" : "is-wrong" : answers[index]?.trim() ? "is-answered" : ""} onClick={() => setWrongOnly(false)} aria-label={`第 ${index + 1} 题`}>{index + 1}</a>)}</div><p className="muted-note">建议先完整听一遍再作答；填空须遵守题目字数限制，选择与匹配题每题选一项。</p>{!completed ? <><p className="listening-save-state" role="status">{saveMessage || "答案会自动保存"}</p><button type="button" className="btn-ghost" disabled={busy} onClick={() => void save()}>保存答案</button><button type="button" disabled={busy} onClick={() => void finish()}><CheckCircle2 size={17} /> {busy ? "正在提交…" : "提交听力答案"}</button></> : null}</div>
      <section className="listening-questions" aria-label="听力题目"><p className="listening-instructions" lang="en">{record.instructions}</p>{completed ? <label className="listening-wrong-filter"><input type="checkbox" checked={wrongOnly} onChange={e => setWrongOnly(e.target.checked)} /> 只看错题与未答题</label> : null}{completed && wrongOnly && record.questions.every(q => q.correct) ? <p>本次全部答对，没有错题。</p> : null}{record.questions.map((q, index) => completed && wrongOnly && q.correct ? null : <article className="listening-question" key={index} id={`listening-question-${index}`}><div className="listening-section-heading"><strong>第 {index + 1} 题 · {TYPE_LABELS[q.type] || "听力题"}</strong>{completed ? <span className={q.correct ? "listening-correct" : "listening-wrong"}>{q.correct ? "回答正确" : answers[index]?.trim() ? "需要复盘" : "未作答"}</span> : null}</div><p id={`listening-prompt-${index}`} lang="en">{q.question}</p><fieldset disabled={busy || completed} aria-labelledby={`listening-prompt-${index}`}>{q.type === "sentence_completion" ? <input type="text" aria-label={`第 ${index + 1} 题答案`} maxLength={200} autoComplete="off" spellCheck={false} value={answers[index] || ""} onChange={e => edit(index, e.target.value)} placeholder="填写英文答案" /> : q.options.map(option => { const value = option.trim().slice(0, 1); return <label className={answers[index] === value ? "listening-option is-selected" : "listening-option"} key={option}><input type="radio" name={`listening-${index}`} value={value} checked={answers[index] === value} onChange={() => edit(index, value)} /><span lang="en">{option}</span></label>; })}</fieldset>{completed ? <div className="listening-question-review"><p>你的答案：<strong>{answers[index]?.trim() || "未作答"}</strong> · 参考答案：<strong lang="en">{q.answerKey}</strong></p><p>原文依据：</p><blockquote lang="en">{q.evidence}</blockquote></div> : null}</article>)}</section></div>
      {completed ? <details className="listening-transcript"><summary>查看完整听力原文</summary><p lang="en">{record.transcript}</p></details> : null}
    </> : null}<ErrorDialog message={error} close={() => setError("")} />
  </>;
}
