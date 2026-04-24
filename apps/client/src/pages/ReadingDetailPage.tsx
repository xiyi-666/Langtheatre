import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, BookOpenText, ChevronLeft, ChevronRight, ClipboardCheck, MenuSquare } from "lucide-react";
import { getApiBaseUrl, readingMaterial, submitReadingAnswers } from "../api";
import { useAppStore } from "../store";
import type { ReadingMaterial } from "../types";

type VocabLearningItem = {
  word: string;
  pos: string;
  meanings: string[];
};

type GrammarInsight = {
  sentence: string;
  difficultyPoints: string[];
  studySuggestions: string[];
};

const READING_LOADING_HINTS = [
  "正在提取文章段落结构...",
  "正在匹配关键词与语义标签...",
  "正在整理可回看阅读导航...",
  "正在准备答题关联上下文...",
  "正在加载音频与分段播放信息..."
];

const READING_LOADING_PROGRESS_CAP = 92;
const READING_LOADING_PROGRESS_TICK_MS = 360;
const READING_LOADING_HINT_TICK_MS = 4400;
const AUDIO_FETCH_TIMEOUT_MS = 20000;

type AudioMergeState = "idle" | "merging" | "ready" | "fallback";

export function ReadingDetailPage() {
  const { id = "", view = "article" } = useParams();
  const navigate = useNavigate();
  const [item, setItem] = useState<ReadingMaterial | null>(null);
  const [answers, setAnswers] = useState<Record<number, number>>({});
  const [submitted, setSubmitted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [referenceOpen, setReferenceOpen] = useState(false);
  const [articleParagraphIndex, setArticleParagraphIndex] = useState(0);
  const [referenceParagraphIndex, setReferenceParagraphIndex] = useState(0);
  const [audioIndex, setAudioIndex] = useState(0);
  const [mergedAudioUrl, setMergedAudioUrl] = useState("");
  const [audioMergeState, setAudioMergeState] = useState<AudioMergeState>("idle");
  const [audioMergeMessage, setAudioMergeMessage] = useState("");
  const [loadingProgress, setLoadingProgress] = useState(12);
  const [loadingHintIndex, setLoadingHintIndex] = useState(0);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const readingContentRef = useRef<HTMLElement | null>(null);
  const touchStartXRef = useRef<number | null>(null);
  const loadingSeqRef = useRef(0);
  const setResult = useAppStore((s) => s.setResult);
  const refreshUserXP = useAppStore((s) => s.refreshUserXP);

  const activeView = view === "quiz" ? "quiz" : "article";

  const passageParagraphs = useMemo(() => {
    if (!item?.passage) return [] as string[];
    return segmentPassage(item.passage);
  }, [item]);

  const audioQueue = useMemo(() => {
    if (!item) return [] as string[];
    if (item.audioUrls?.length) {
      return item.audioUrls.filter((u) => Boolean(u?.trim()));
    }
    if (item.audioUrl?.trim()) {
      return [item.audioUrl];
    }
    return [] as string[];
  }, [item]);

  const scoreInfo = useMemo(() => {
    if (!item?.questions?.length) return { score: 0, total: 0 };
    const total = item.questions.length;
    let correct = 0;
    item.questions.forEach((q, idx) => {
      const selected = answers[idx];
      if (selected == null) return;
      const userAnswer = q.options?.[selected];
      if (userAnswer && q.answerKey && userAnswer === q.answerKey) {
        correct += 1;
      }
    });
    return { score: correct, total };
  }, [answers, item]);

  const activeReferenceParagraph = passageParagraphs[referenceParagraphIndex] ?? "";
  const activeArticleParagraph = passageParagraphs[articleParagraphIndex] ?? "";

  const learningVocabulary = useMemo(() => {
    const aiVocabulary = (item?.vocabularyItems ?? [])
      .map((v) => {
        const word = (v.word ?? "").trim();
        return {
          word,
          pos: (v.pos ?? "").trim() || "n./v.",
          meanings: sanitizeMeaningsForDisplay(word, (v.meanings ?? []).map((m) => (m ?? "").trim()), item?.topic ?? "")
        };
      })
      .filter((v) => v.word && v.meanings.length > 0);
    if (aiVocabulary.length > 0) {
      const merged = [...aiVocabulary];
      const fallback = buildLearningVocabulary(item?.vocabulary ?? [], item?.passage ?? "", item?.topic ?? "")
        .filter((v) => !isGenericVocabularyItem(v));
      const seen = new Set(merged.map((v) => v.word.toLowerCase()));
      for (const extra of fallback) {
        const key = extra.word.toLowerCase();
        if (seen.has(key)) continue;
        seen.add(key);
        merged.push(extra);
        if (merged.length >= 15) break;
      }
      if (merged.length < 15) {
        for (const extra of buildCuratedTopUpVocabulary(item?.topic ?? "", seen)) {
          merged.push(extra);
          if (merged.length >= 15) break;
        }
      }
      return merged.slice(0, 15);
    }
    return buildLearningVocabulary(item?.vocabulary ?? [], item?.passage ?? "", item?.topic ?? "");
  }, [item?.vocabularyItems, item?.vocabulary, item?.passage, item?.topic]);

  const associationSentence = useMemo(() => {
    const uniqueAi = Array.from(new Set(
      (item?.associationSentences ?? [])
        .map((s) => (s ?? "").trim())
        .filter(Boolean)
        .filter((s) => !containsLowQualityAssociationTemplate(s))
    ));
    if (uniqueAi.length >= 3) {
      return uniqueAi.slice(0, 3);
    }
    const merged = [...uniqueAi];
    const seen = new Set(merged.map((s) => s.toLowerCase()));
    for (const s of buildAssociationSentences(learningVocabulary, item?.topic ?? "")) {
      const key = s.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      merged.push(s);
      if (merged.length >= 3) break;
    }
    return merged.slice(0, 3);
  }, [item?.associationSentences, learningVocabulary, item?.topic]);

  const grammarInsights = useMemo(() => {
    const aiGrammar = (item?.grammarInsights ?? [])
      .map((gi) => ({
        sentence: (gi.sentence ?? "").trim(),
        difficultyPoints: (gi.difficultyPoints ?? []).map((d) => (d ?? "").trim()).filter(Boolean),
        studySuggestions: (gi.studySuggestions ?? []).map((s) => (s ?? "").trim()).filter(Boolean)
      }))
      .filter((gi) => gi.sentence && gi.difficultyPoints.length > 0 && gi.studySuggestions.length > 0);
    if (aiGrammar.length > 0) {
      return aiGrammar;
    }
    return buildGrammarInsights(item?.passage ?? "");
  }, [item?.grammarInsights, item?.passage]);

  const semanticSourceLabel = (() => {
    const aiVocabCount = item?.vocabularyItems?.length ?? 0;
    const aiGrammarCount = item?.grammarInsights?.length ?? 0;
    if (aiVocabCount >= 15 && aiGrammarCount > 0) return "AI 语义解析";
    if (aiVocabCount > 0 || aiGrammarCount > 0) return "AI 语义解析 + 补全";
    return "规则兜底";
  })();

  useEffect(() => {
    loadingSeqRef.current += 1;
    const seq = loadingSeqRef.current;

    void (async () => {
      try {
        const data = await readingMaterial(id);
        if (loadingSeqRef.current !== seq) return;
        setItem(data);
        setLoadingProgress(100);
        setAudioIndex(0);
        setMergedAudioUrl("");
        setAudioMergeState("idle");
        setAudioMergeMessage("");
      } catch (e) {
        if (loadingSeqRef.current !== seq) return;
        console.error("reading detail load failed", e);
        navigate("/reading");
      }
    })();
  }, [id, navigate]);

  useEffect(() => {
    if (item) return;
    const progressTimer = window.setInterval(() => {
      setLoadingProgress((prev) => {
        if (prev >= READING_LOADING_PROGRESS_CAP) return prev;
        const step = prev < 50 ? 3 : prev < 78 ? 2 : 1;
        return Math.min(READING_LOADING_PROGRESS_CAP, prev + step);
      });
    }, READING_LOADING_PROGRESS_TICK_MS);

    let hintTimer: number | null = null;
    const scheduleHint = () => {
      hintTimer = window.setTimeout(() => {
        setLoadingHintIndex((prev) => (prev + 1) % READING_LOADING_HINTS.length);
        scheduleHint();
      }, READING_LOADING_HINT_TICK_MS);
    };
    scheduleHint();

    return () => {
      window.clearInterval(progressTimer);
      if (hintTimer) {
        window.clearTimeout(hintTimer);
      }
    };
  }, [item]);

  useEffect(() => {
    if (!readingContentRef.current) return;
    readingContentRef.current.scrollTo({ top: 0, behavior: "smooth" });
  }, [activeView]);

  async function handleSubmitReadingAnswers() {
    if (!item || submitting) return;
    setSubmitError("");
    setSubmitting(true);
    try {
      const payloadAnswers = (item.questions ?? []).map((q, idx) => {
        const selectedIndex = answers[idx];
        if (selectedIndex == null || selectedIndex < 0) return "";
        return q.options?.[selectedIndex] ?? "";
      });
      const result = await submitReadingAnswers(item.id, payloadAnswers);
      setResult(result);
      await refreshUserXP();
      navigate("/result");
    } catch (error) {
      console.error("submit reading answers failed", error);
      setSubmitError("提交失败，请稍后重试。");
      setSubmitted(true);
    } finally {
      setSubmitting(false);
    }
  }

  useEffect(() => {
    setReferenceParagraphIndex(0);
    setArticleParagraphIndex(0);
  }, [id, referenceOpen]);

  useEffect(() => {
    if (passageParagraphs.length === 0) return;
    setReferenceParagraphIndex((prev) => Math.min(prev, passageParagraphs.length - 1));
    setArticleParagraphIndex((prev) => Math.min(prev, passageParagraphs.length - 1));
  }, [passageParagraphs]);

  useEffect(() => {
    let revoked = "";
    let cancelled = false;

    async function mergeChunks() {
      if (audioQueue.length <= 1) {
        setAudioMergeState("idle");
        setAudioMergeMessage("");
        return;
      }
      if (!canAttemptAudioMerge(audioQueue)) {
        setMergedAudioUrl("");
        setAudioMergeState("fallback");
        setAudioMergeMessage("音频链接无效，已回退连续播放");
        return;
      }
      setAudioMergeState("merging");
      setAudioMergeMessage("正在合并分段音频...");
      try {
        const url = await mergeAudioChunksToWav(audioQueue);
        if (cancelled) {
          URL.revokeObjectURL(url);
          return;
        }
        revoked = url;
        setMergedAudioUrl(url);
        setAudioMergeState("ready");
        setAudioMergeMessage("已合并为单条音频播放");
      } catch (error) {
        if (!cancelled) {
          setMergedAudioUrl("");
          setAudioMergeState("fallback");
          setAudioMergeMessage(formatAudioMergeError(error));
        }
      } finally {
        if (!cancelled) {
        }
      }
    }

    void mergeChunks();

    return () => {
      cancelled = true;
      if (revoked) {
        URL.revokeObjectURL(revoked);
      }
    };
  }, [audioQueue]);

  if (!item) {
    const currentHint = READING_LOADING_HINTS[loadingHintIndex % READING_LOADING_HINTS.length];
    return (
      <main className="page">
        <section className="card reading-loading-shell">
          <h3>阅读文章加载中</h3>
          <p>请稍等，正在为你准备更易读的段落与学习提示。</p>
          <div className="reading-loading-progress" aria-label="阅读加载进度">
            <div className="reading-loading-progress-value" style={{ width: `${loadingProgress}%` }} />
          </div>
          <small>{loadingProgress}%</small>
          <div className="reading-loading-cinematic" aria-live="polite">
            <span key={`hint-${loadingHintIndex}`} className="reading-loading-line">{currentHint}</span>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="page">
      <section className="card reading-shell">
        <header className="reading-header">
          <div>
            <h2>{item.title}</h2>
            <p>{item.topic}</p>
          </div>
          <div className="row">
            <button className="btn-ghost" onClick={() => navigate("/reading")}>
              <ArrowLeft size={14} /> 返回阅读中心
            </button>
          </div>
        </header>

        <nav className="reading-tabs" aria-label="阅读详情子页面切换">
          <button
            type="button"
            className={activeView === "article" ? "route-tab active" : "route-tab"}
            onClick={() => navigate(`/reading/${id}/article`)}
          >
            <BookOpenText size={14} /> 阅读文章
          </button>
          <button
            type="button"
            className={activeView === "quiz" ? "route-tab active" : "route-tab"}
            onClick={() => navigate(`/reading/${id}/quiz`)}
          >
            <ClipboardCheck size={14} /> 阅读答题
          </button>
        </nav>

        <section className="reading-content" ref={readingContentRef}>
          {activeView === "article" ? (
            <>
              <article className="stage-banner">
                <strong>全文音频</strong>
                {item.audioStatus === "READY" && audioQueue.length > 0 ? (
                  <div className="audio-inline">
                    {audioQueue.length > 1 ? (
                      <small>
                        {(audioMergeState === "ready"
                          ? "已合并为单条音频播放"
                          : audioMergeState === "merging"
                            ? "正在合并分段音频..."
                            : audioMergeMessage || "已回退连续播放") + `（${audioIndex + 1}/${audioQueue.length}）`}
                      </small>
                    ) : null}
                    <audio
                      ref={audioRef}
                      controls
                      preload="none"
                      src={mergedAudioUrl || audioQueue[audioIndex]}
                      onEnded={() => {
                        if (mergedAudioUrl) return;
                        if (audioIndex >= audioQueue.length - 1) return;
                        const next = audioIndex + 1;
                        setAudioIndex(next);
                        // Let state update first, then continue autoplay for seamless segmented playback.
                        setTimeout(() => {
                          audioRef.current?.play().catch(() => undefined);
                        }, 0);
                      }}
                    >
                      <track kind="captions" />
                    </audio>
                  </div>
                ) : (
                  <p>{item.audioStatus === "FAILED" ? "音频生成失败，请重新生成材料。" : "音频后台生成中，完成后可播放。"}</p>
                )}
              </article>

              <article className={item.language === "ENGLISH" ? "reading-article en" : "reading-article"}>
                <div className="reading-reference-header">
                  <strong>正文阅读（可左右滑动）</strong>
                  <small>{passageParagraphs.length > 0 ? `第 ${articleParagraphIndex + 1} / ${passageParagraphs.length} 段` : "无段落"}</small>
                </div>

                <div
                  className="reading-swipe-viewer"
                  onTouchStart={(event) => {
                    touchStartXRef.current = event.touches[0]?.clientX ?? null;
                  }}
                  onTouchEnd={(event) => {
                    const startX = touchStartXRef.current;
                    const endX = event.changedTouches[0]?.clientX ?? null;
                    touchStartXRef.current = null;
                    if (startX == null || endX == null) return;
                    const delta = endX - startX;
                    if (Math.abs(delta) < 44) return;
                    if (delta < 0) {
                      setArticleParagraphIndex((prev) => Math.min(prev + 1, passageParagraphs.length - 1));
                    } else {
                      setArticleParagraphIndex((prev) => Math.max(prev - 1, 0));
                    }
                  }}
                >
                  <button
                    type="button"
                    className="swipe-arrow"
                    onClick={() => setArticleParagraphIndex((prev) => Math.max(prev - 1, 0))}
                    disabled={articleParagraphIndex <= 0}
                    aria-label="上一段"
                  >
                    <ChevronLeft size={16} />
                  </button>
                  <article className="reading-reference-item" key={`article-${articleParagraphIndex}`}>
                    <small>第 {articleParagraphIndex + 1} 段</small>
                    <p>{activeArticleParagraph || item.passage}</p>
                  </article>
                  <button
                    type="button"
                    className="swipe-arrow"
                    onClick={() => setArticleParagraphIndex((prev) => Math.min(prev + 1, passageParagraphs.length - 1))}
                    disabled={articleParagraphIndex >= passageParagraphs.length - 1}
                    aria-label="下一段"
                  >
                    <ChevronRight size={16} />
                  </button>
                </div>

                <div className="reading-index-dots" aria-label="正文段落快速跳转">
                  {passageParagraphs.map((_, idx) => (
                    <button
                      key={`article-dot-${idx}`}
                      type="button"
                      className={idx === articleParagraphIndex ? "reading-index-dot active" : "reading-index-dot"}
                      onClick={() => setArticleParagraphIndex(idx)}
                      aria-label={`跳转到第 ${idx + 1} 段`}
                    >
                      <span>◉</span>
                      {idx + 1}
                    </button>
                  ))}
                </div>
              </article>

              <section className="learning-panels">
                <details className="learning-details" open>
                  <summary>重点词汇</summary>
                  <small className="learning-meta">来源：{semanticSourceLabel}</small>
                  <ul className="learning-list">
                    {learningVocabulary.map((v) => (
                      <li key={v.word} className="learning-item">
                        <div className="learning-head">
                          <strong>{v.word}</strong>
                          <span>{v.pos}</span>
                        </div>
                        <ul className="learning-meaning-list">
                          {v.meanings.map((meaning) => (
                            <li key={`${v.word}-${meaning}`}>{meaning}</li>
                          ))}
                        </ul>
                      </li>
                    ))}
                  </ul>
                </details>

                <details className="learning-details" open>
                  <summary>词语联想记忆</summary>
                  <article className="learning-memory">
                    <ol>
                      {associationSentence.map((sentence, idx) => (
                        <li key={`memory-${idx}`}>{highlightVocabulary(sentence, learningVocabulary.map((v) => v.word))}</li>
                      ))}
                    </ol>
                    <small>建议朗读 2-3 次</small>
                  </article>
                </details>

                <details className="learning-details" open>
                  <summary>长难句语法解析</summary>
                  <ul className="learning-list">
                    {grammarInsights.map((insight, idx) => (
                      <li key={`grammar-${idx}`} className="learning-item">
                        <div className="learning-head">
                          <strong>句子 {idx + 1}</strong>
                          <span>难度解析</span>
                        </div>
                        <p>{insight.sentence}</p>
                        <strong>难点</strong>
                        <ul className="learning-sub-list">
                          {insight.difficultyPoints.map((point) => (
                            <li key={`${idx}-${point}`}>{point}</li>
                          ))}
                        </ul>
                        <strong>学习建议</strong>
                        <ul className="learning-sub-list">
                          {insight.studySuggestions.map((tip) => (
                            <li key={`${idx}-${tip}`}>{tip}</li>
                          ))}
                        </ul>
                      </li>
                    ))}
                  </ul>
                </details>
              </section>
            </>
          ) : (
            <>
              <article className="stage-banner reading-quiz-topbar">
                <strong>阅读题（{item.questions?.length ?? 0}题）</strong>
                <button type="button" className="btn-ghost" onClick={() => setReferenceOpen((v) => !v)}>
                  <MenuSquare size={14} /> {referenceOpen ? "关闭参考文章" : "打开参考文章"}
                </button>
              </article>

              <article className="stage-banner">
                <ol>
                  {(item.questions ?? []).map((q, idx) => (
                    <li key={`${q.question}-${idx}`}>
                      <p>{q.question}</p>
                      {q.options?.length ? (
                        <div className="dialogue-list">
                          {q.options.map((opt, i) => {
                            const selected = answers[idx] === i;
                            const isCorrect = submitted && q.answerKey === opt;
                            const isWrongSelected = submitted && selected && q.answerKey !== opt;
                            const classNames = [
                              "option-item",
                              selected ? "selected" : "",
                              isCorrect ? "correct" : "",
                              isWrongSelected ? "wrong" : ""
                            ]
                              .filter(Boolean)
                              .join(" ");
                            return (
                              <button
                                key={`${opt}-${i}`}
                                type="button"
                                className={classNames}
                                onClick={() => {
                                  if (submitted) return;
                                  setAnswers((prev) => ({ ...prev, [idx]: i }));
                                }}
                              >
                                {String.fromCharCode(65 + i)}. {opt}
                              </button>
                            );
                          })}
                        </div>
                      ) : null}
                    </li>
                  ))}
                </ol>
                <div className="row">
                  <button type="button" onClick={handleSubmitReadingAnswers} disabled={submitting}>
                    {submitting ? "提交中..." : "提交答案"}
                  </button>
                  <button
                    type="button"
                    className="btn-ghost"
                    onClick={() => {
                      setAnswers({});
                      setSubmitted(false);
                    }}
                  >
                    重做
                  </button>
                </div>
                {submitted ? <p>得分：{scoreInfo.score} / {scoreInfo.total}</p> : null}
                {submitError ? <p className="error">{submitError}</p> : null}
              </article>

              {referenceOpen ? (
                <aside className="reading-reference-panel">
                  <div className="reading-reference-header">
                    <strong>查阅文章段落</strong>
                    <small>{passageParagraphs.length > 0 ? `第 ${referenceParagraphIndex + 1} / ${passageParagraphs.length} 段` : "无段落"}</small>
                  </div>

                  <div
                    className="reading-swipe-viewer"
                    onTouchStart={(event) => {
                      touchStartXRef.current = event.touches[0]?.clientX ?? null;
                    }}
                    onTouchEnd={(event) => {
                      const startX = touchStartXRef.current;
                      const endX = event.changedTouches[0]?.clientX ?? null;
                      touchStartXRef.current = null;
                      if (startX == null || endX == null) return;
                      const delta = endX - startX;
                      if (Math.abs(delta) < 44) return;
                      if (delta < 0) {
                        setReferenceParagraphIndex((prev) => Math.min(prev + 1, passageParagraphs.length - 1));
                      } else {
                        setReferenceParagraphIndex((prev) => Math.max(prev - 1, 0));
                      }
                    }}
                  >
                    <button
                      type="button"
                      className="swipe-arrow"
                      onClick={() => setReferenceParagraphIndex((prev) => Math.max(prev - 1, 0))}
                      disabled={referenceParagraphIndex <= 0}
                      aria-label="上一段"
                    >
                      <ChevronLeft size={16} />
                    </button>
                    <article className="reading-reference-item" key={`ref-${referenceParagraphIndex}`}>
                      <small>第 {referenceParagraphIndex + 1} 段</small>
                      <p>{activeReferenceParagraph}</p>
                    </article>
                    <button
                      type="button"
                      className="swipe-arrow"
                      onClick={() => setReferenceParagraphIndex((prev) => Math.min(prev + 1, passageParagraphs.length - 1))}
                      disabled={referenceParagraphIndex >= passageParagraphs.length - 1}
                      aria-label="下一段"
                    >
                      <ChevronRight size={16} />
                    </button>
                  </div>

                  <div className="reading-index-dots" aria-label="段落快速跳转">
                    {passageParagraphs.map((_, idx) => (
                      <button
                        key={`dot-${idx}`}
                        type="button"
                        className={idx === referenceParagraphIndex ? "reading-index-dot active" : "reading-index-dot"}
                        onClick={() => setReferenceParagraphIndex(idx)}
                        aria-label={`跳转到第 ${idx + 1} 段`}
                      >
                        <span>◉</span>
                        {idx + 1}
                      </button>
                    ))}
                  </div>

                  <button type="button" className="btn-ghost" onClick={() => navigate(`/reading/${id}/article`)}>
                    去完整阅读页
                  </button>
                </aside>
              ) : null}
            </>
          )}
        </section>
      </section>
    </main>
  );
}

async function mergeAudioChunksToWav(urls: string[]): Promise<string> {
  const buffers = await Promise.all(urls.map((url) => fetchAudioBuffer(url, AUDIO_FETCH_TIMEOUT_MS)));
  const audioContext = new AudioContext();
  try {
    const decoded = await Promise.all(buffers.map((buf) => audioContext.decodeAudioData(buf.slice(0))));
    if (decoded.length === 0) {
      throw new Error("merge-empty");
    }
    const sampleRate = Math.max(...decoded.map((buffer) => buffer.sampleRate));
    const channelCount = Math.max(...decoded.map((buffer) => buffer.numberOfChannels));
    const normalized = await Promise.all(
      decoded.map((buffer) => normalizeAudioBuffer(buffer, sampleRate, channelCount))
    );
    const totalLength = normalized.reduce((sum, b) => sum + b.length, 0);
    const merged = audioContext.createBuffer(channelCount, totalLength, sampleRate);

    let offset = 0;
    for (const buffer of normalized) {
      for (let channel = 0; channel < channelCount; channel++) {
        merged.getChannelData(channel).set(buffer.getChannelData(channel), offset);
      }
      offset += buffer.length;
    }
    const wavBlob = encodeWav(merged);
    return URL.createObjectURL(wavBlob);
  } finally {
    await audioContext.close();
  }
}

function canAttemptAudioMerge(urls: string[]): boolean {
  return urls.every((raw) => {
    const url = (raw ?? "").trim();
    if (!url) return false;
    if (url.startsWith("blob:") || url.startsWith("data:")) return true;
    try {
      new URL(url, window.location.origin);
      return true;
    } catch {
      return false;
    }
  });
}

async function fetchAudioBuffer(url: string, timeoutMs: number): Promise<ArrayBuffer> {
  const requestURL = buildAudioFetchURL(url);
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(requestURL, {
      mode: "cors",
      signal: controller.signal
    });
    if (!response.ok) {
      throw new Error(`audio-fetch-status:${response.status}`);
    }
    return await response.arrayBuffer();
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new Error("audio-fetch-timeout");
    }
    if (error instanceof TypeError) {
      throw new Error("audio-fetch-cors");
    }
    throw error;
  } finally {
    window.clearTimeout(timeout);
  }
}

function buildAudioFetchURL(raw: string): string {
  const url = (raw ?? "").trim();
  if (url.startsWith("blob:") || url.startsWith("data:")) {
    return url;
  }

  const resolved = new URL(url, window.location.origin);
  if (resolved.origin === window.location.origin) {
    return resolved.href;
  }

  const apiBase = getApiBaseUrl();
  return `${apiBase}/media-proxy?url=${encodeURIComponent(resolved.href)}`;
}

async function normalizeAudioBuffer(source: AudioBuffer, targetSampleRate: number, targetChannels: number): Promise<AudioBuffer> {
  if (source.sampleRate === targetSampleRate && source.numberOfChannels === targetChannels) {
    return source;
  }

  const frameCount = Math.max(1, Math.ceil(source.duration * targetSampleRate));
  const offline = new OfflineAudioContext(targetChannels, frameCount, targetSampleRate);
  const input = offline.createBuffer(targetChannels, source.length, source.sampleRate);
  for (let channel = 0; channel < targetChannels; channel++) {
    const sourceChannel = source.getChannelData(Math.min(channel, source.numberOfChannels - 1));
    input.getChannelData(channel).set(sourceChannel);
  }

  const node = offline.createBufferSource();
  node.buffer = input;
  node.connect(offline.destination);
  node.start(0);
  return await offline.startRendering();
}

function formatAudioMergeError(error: unknown): string {
  const message = error instanceof Error ? error.message : "";
  if (message.includes("audio-fetch-timeout")) {
    return "音频拉取超时，已回退连续播放";
  }
  if (message.includes("audio-fetch-cors")) {
    return "音频源不允许浏览器跨域读取，已回退连续播放";
  }
  if (message.includes("audio-fetch-status:")) {
    const status = message.split(":")[1] ?? "";
    return `音频分段下载失败（HTTP ${status}），已回退连续播放`;
  }
  if (message.includes("Unable to decode audio data") || message.includes("EncodingError")) {
    return "音频解码失败，已回退连续播放";
  }
  if (message.includes("merge-empty")) {
    return "未拿到可合并的音频片段，已回退连续播放";
  }
  return "音频合并失败，已回退连续播放";
}

function encodeWav(buffer: AudioBuffer): Blob {
  const channels = buffer.numberOfChannels;
  const sampleRate = buffer.sampleRate;
  const length = buffer.length;
  const bytesPerSample = 2;
  const blockAlign = channels * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = length * blockAlign;
  const wav = new ArrayBuffer(44 + dataSize);
  const view = new DataView(wav);

  writeAscii(view, 0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeAscii(view, 8, "WAVE");
  writeAscii(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, channels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, 16, true);
  writeAscii(view, 36, "data");
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (let i = 0; i < length; i++) {
    for (let channel = 0; channel < channels; channel++) {
      const sample = Math.max(-1, Math.min(1, buffer.getChannelData(channel)[i]));
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
      offset += bytesPerSample;
    }
  }
  return new Blob([wav], { type: "audio/wav" });
}

function writeAscii(view: DataView, offset: number, text: string) {
  for (let i = 0; i < text.length; i++) {
    view.setUint8(offset + i, text.charCodeAt(i));
  }
}

function segmentPassage(passage: string): string[] {
  const normalized = passage.replace(/\r\n/g, "\n").trim();
  if (!normalized) return [];

  // Strategy 1: split by blank lines (author-intended paragraphs).
  const byBlankLine = normalized
    .split(/\n\s*\n+/)
    .map((p) => p.trim())
    .filter(Boolean);
  if (byBlankLine.length > 1) return byBlankLine;

  // Strategy 2: split by single line breaks when source doesn't include blank lines.
  const bySingleLine = normalized
    .split(/\n+/)
    .map((p) => p.trim())
    .filter(Boolean);
  if (bySingleLine.length > 1) return bySingleLine;

  // Strategy 3: fallback to sentence-based chunking for one long paragraph.
  const sentences = normalized
    .split(/(?<=[。！？.!?])\s+/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (sentences.length <= 1) return [normalized];

  const chunks: string[] = [];
  const targetLength = 180;
  let current = "";

  for (const sentence of sentences) {
    const next = current ? `${current} ${sentence}` : sentence;
    if (next.length > targetLength && current) {
      chunks.push(current);
      current = sentence;
      continue;
    }
    current = next;
  }

  if (current) {
    chunks.push(current);
  }

  return chunks.length > 0 ? chunks : [normalized];
}

function buildLearningVocabulary(sourceVocabulary: string[], passage: string, topic: string): VocabLearningItem[] {
  const topicWords = tokenizeWords(topic);
  const passageWords = tokenizeWords(passage);
  const merged = [...sourceVocabulary, ...topicWords, ...passageWords];

  const unique: string[] = [];
  const seen = new Set<string>();
  for (const raw of merged) {
    const cleaned = raw.trim().replace(/^[^A-Za-z]+|[^A-Za-z]+$/g, "");
    if (!cleaned) continue;
    const normalized = cleaned.toLowerCase();
    if (normalized.length < 4) continue;
    if (STOP_WORDS.has(normalized)) continue;
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    unique.push(cleaned);
  }

  for (const fallback of FALLBACK_VOCABULARY) {
    const normalized = fallback.toLowerCase();
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    unique.push(fallback);
    if (unique.length >= 15) break;
  }

  return unique.slice(0, 15).map((word) => ({
    word,
    pos: inferPartOfSpeech(word),
    meanings: explainInChinese(word, topic)
  }));
}

function buildAssociationSentences(vocabulary: VocabLearningItem[], topic: string): string[] {
  if (vocabulary.length === 0) {
    return [
      "Students improve comprehension when they map key words before reading.",
      "Learners can connect evidence and logic to build clearer understanding.",
      "Reviewing one passage with focused vocabulary creates stronger long-term memory."
    ];
  }
  const w = vocabulary.map((v) => v.word);
  const a = w[0] ?? "topic";
  const b = w[1] ?? "context";
  const c = w[2] ?? "pattern";
  const d = w[3] ?? "analysis";
  const e = w[4] ?? "insight";
  const f = w[5] ?? "learning";
  const g = w[6] ?? "strategy";
  const h = w[7] ?? "evidence";
  const i = w[8] ?? "outcome";
  const topicPhrase = topic ? topic.toLowerCase() : "daily study";
  return [
    `In ${topicPhrase}, learners connect ${a} with ${b} so that ${c} becomes easier to understand.`,
    `When students compare ${d}, ${e}, and ${f}, they build a stronger mental map of the passage.`,
    `By linking ${g} to ${h}, readers can predict the ${i} and remember key ideas faster.`
  ];
}

function buildGrammarInsights(passage: string): GrammarInsight[] {
  const rankedSentences = splitSentences(passage)
    .map((sentence) => ({ sentence: sentence.trim(), score: getGrammarComplexityScore(sentence) }))
    .filter((item) => item.sentence.length > 0)
    .sort((a, b) => b.score - a.score)
    .filter((item) => item.score >= 3)
    .slice(0, 4)
    .map((item) => item.sentence);

  if (rankedSentences.length === 0) {
    return [{
      sentence: "This passage does not contain very long sentences, so focus on subject-verb-object patterns and connectors.",
      difficultyPoints: [
        "句子整体较短，结构变化较少。",
        "重点在词块识别与主谓宾定位。"
      ],
      studySuggestions: [
        "先标出主语和谓语，再补充宾语与状语。",
        "使用连接词（because/while/if）练习扩展句。"
      ]
    }];
  }

  return rankedSentences.map((sentence) => {
    const lowered = ` ${sentence.toLowerCase()} `;
    const connectors = GRAMMAR_CONNECTORS.filter((word) => lowered.includes(` ${word} `));
    const commaCount = (sentence.match(/[,，;；]/g) ?? []).length;
    const wordCount = tokenizeWords(sentence).length;
    const difficultyPoints: string[] = [];
    const studySuggestions: string[] = [];

    if (connectors.length > 0) {
      difficultyPoints.push(`出现多个连接词：${connectors.join(" / ")}，从句边界较多。`);
      studySuggestions.push("先圈出连接词，再按连接词切分主句与从句。")
    }

    if (commaCount >= 2) {
      difficultyPoints.push("逗号或分号较多，句内信息块层级复杂。");
      studySuggestions.push("按标点拆成 2-4 个意群，逐块翻译再回并。")
    }

    if (/\b(which|who|that|whose|whom)\b/i.test(sentence)) {
      difficultyPoints.push("含定语从句，先行词与从句关系容易混淆。");
      studySuggestions.push("先找先行词，再判断从句在修饰谁。")
    }

    if (/\b(is|are|was|were|be|been|being)\s+\w+ed\b/i.test(sentence)) {
      difficultyPoints.push("疑似被动语态，动作执行者可能被省略。");
      studySuggestions.push("先改写成主动语态，有助于快速抓住语义主线。")
    }

    if (wordCount >= 24) {
      difficultyPoints.push(`词数约 ${wordCount}，句子跨度长，记忆负担较高。`);
      studySuggestions.push("先提取主干（主语-谓语-核心宾语），再补修饰成分。")
    }

    if (difficultyPoints.length === 0) {
      difficultyPoints.push("句子表面结构不复杂，但隐含逻辑关系需要结合上下文。")
      studySuggestions.push("回到上一个句子找逻辑承接词，再判断当前句作用。")
    }

    return {
      sentence,
      difficultyPoints,
      studySuggestions
    };
  });
}

function getGrammarComplexityScore(sentence: string): number {
  const text = ` ${sentence.toLowerCase()} `;
  const connectorScore = GRAMMAR_CONNECTORS.reduce((score, connector) => {
    return score + (text.includes(` ${connector} `) ? 1 : 0);
  }, 0);
  const punctuationScore = (sentence.match(/[,，;；:：]/g) ?? []).length;
  const lengthScore = tokenizeWords(sentence).length >= 18 ? 2 : 0;
  const relativeScore = /\b(which|who|that|whose|whom)\b/i.test(sentence) ? 2 : 0;
  const passiveScore = /\b(is|are|was|were|be|been|being)\s+\w+ed\b/i.test(sentence) ? 1 : 0;
  return connectorScore + punctuationScore + lengthScore + relativeScore + passiveScore;
}

function splitSentences(text: string): string[] {
  const normalized = text.replace(/\s+/g, " ").trim();
  if (!normalized) return [];
  const matched = normalized.match(/[^.!?。！？]+[.!?。！？]?/g);
  return matched?.map((m) => m.trim()).filter(Boolean) ?? [normalized];
}

function tokenizeWords(text: string): string[] {
  return text.match(/[A-Za-z][A-Za-z'-]*/g) ?? [];
}

function inferPartOfSpeech(word: string): string {
  const w = word.toLowerCase();
  const override = POS_OVERRIDES[w];
  if (override) return override;
  if (/(tion|sion|ment|ness|ity|ship|ance|ence)$/.test(w)) return "n. 名词";
  if (/(ly)$/.test(w)) return "adv. 副词";
  if (/(ous|ful|ive|able|ible|al|ic|less)$/.test(w)) return "adj. 形容词";
  if (/(ing|ed|ize|ise|fy|ate)$/.test(w)) return "v. 动词";
  return "n./v. 常见核心词";
}

function explainInChinese(word: string, topic: string): string[] {
  const key = word.toLowerCase();
  const mapped = WORD_EXPLANATIONS[key];
  if (mapped) return mapped;

  const pos = inferPartOfSpeech(word);
  const topicHint = topic ? `（结合“${topic}”语境）` : "";
  if (pos.startsWith("adj")) {
    return [
      `adj. ${word} 常用于描述性质或状态${topicHint}。`,
      `adj. 用法提示：关注 ${word} 在句中修饰的是对象、过程还是结果。`
    ];
  }
  if (pos.startsWith("adv")) {
    return [
      `adv. ${word} 多用于表示方式、程度或频率${topicHint}。`,
      `adv. 用法提示：观察 ${word} 是修饰动作还是整句逻辑。`
    ];
  }
  if (pos.startsWith("v")) {
    return [
      `v. ${word} 在句中多表示动作或过程${topic ? `（常见于“${topic}”语境）` : ""}。`,
      `v. 用法提示：关注 ${word} 的时态、语态及与宾语的搭配。`
    ];
  }
  return [
    `n. ${word} 常指文本中的关键对象或核心概念${topicHint}。`,
    `n. 用法提示：结合前后句判断 ${word} 在文中更偏“现象/方法/结果”哪一类。`
  ];
}

function sanitizeMeaningsForDisplay(word: string, meanings: string[], topic: string): string[] {
  const cleaned = meanings
    .map((m) => m.trim())
    .filter(Boolean)
    .filter((m) => !containsLowQualityMeaningTemplate(m));
  if (cleaned.length > 0) {
    return Array.from(new Set(cleaned));
  }
  return explainInChinese(word, topic);
}

function isGenericVocabularyItem(item: VocabLearningItem): boolean {
  if (item.meanings.length === 0) return true;
  return item.meanings.every((meaning) => containsLowQualityMeaningTemplate(meaning));
}

function buildCuratedTopUpVocabulary(topic: string, seenWords: Set<string>): VocabLearningItem[] {
  const result: VocabLearningItem[] = [];
  for (const word of FALLBACK_VOCABULARY) {
    const key = word.toLowerCase();
    if (seenWords.has(key)) continue;
    seenWords.add(key);
    result.push({
      word,
      pos: inferPartOfSpeech(word),
      meanings: sanitizeMeaningsForDisplay(word, explainInChinese(word, topic), topic)
    });
  }
  return result;
}

function containsLowQualityMeaningTemplate(text: string): boolean {
  const low = text.toLowerCase();
  const templates = [
    "常见义：该词通常表示",
    "常见义：该词在阅读语境中表示",
    "该词在阅读中通常表示核心概念或关键对象",
    "引申义：可表示相关方法、影响或结果",
    "引申义：可进一步表示相关的方法、影响或结果",
    "需结合上下文判断",
    "需要结合上下文判断"
  ];
  return templates.some((pattern) => low.includes(pattern));
}

function containsLowQualityAssociationTemplate(text: string): boolean {
  const low = text.toLowerCase();
  const templates = [
    "readers can link key words to",
    "and explain one complete idea with evidence from the passage",
    "and retell one complete idea accurately"
  ];
  return templates.some((pattern) => low.includes(pattern));
}

function highlightVocabulary(sentence: string, words: string[]): Array<string | JSX.Element> {
  const wordSet = new Set(words.map((w) => w.toLowerCase()));
  const segments = sentence.split(/(\b[A-Za-z][A-Za-z'-]*\b)/g);
  return segments.map((segment, idx) => {
    const lower = segment.toLowerCase();
    if (wordSet.has(lower)) {
      return <strong key={`hl-${segment}-${idx}`} className="learning-emphasis">{segment}</strong>;
    }
    return segment;
  });
}

const STOP_WORDS = new Set([
  "about", "above", "after", "again", "against", "among", "because", "before", "being", "below", "between",
  "could", "every", "first", "from", "have", "having", "however", "into", "itself", "might", "other",
  "should", "since", "still", "their", "there", "these", "those", "through", "under", "until", "which",
  "while", "within", "would", "your", "yours", "about", "across", "where", "whose", "whom"
]);

const FALLBACK_VOCABULARY = [
  "context", "analysis", "strategy", "evidence", "principle", "approach", "outcome", "impact", "policy", "resource",
  "community", "sustainable", "innovation", "efficiency", "collaboration", "interpretation", "practice", "framework", "pattern", "insight"
];

const POS_OVERRIDES: Record<string, string> = {
  reading: "n. 名词"
};

const WORD_EXPLANATIONS: Record<string, string[]> = {
  context: ["n. 语境；上下文", "n. 背景；来龙去脉"],
  analysis: ["n. 分析；解析", "n. 分解说明；研究结果"],
  strategy: ["n. 策略；行动方案", "n.（长期）布局思路"],
  evidence: ["n. 证据；依据", "n. 迹象；证明材料"],
  principle: ["n. 原则；准则", "n. 原理；基本规律"],
  approach: ["n. 方法；路径", "v. 接近；着手处理"],
  outcome: ["n. 结果；结局", "n. 产出；成效"],
  impact: ["n. 影响；冲击", "v. 对…产生作用"],
  policy: ["n. 政策；方针", "n. 保险单（特定语境）"],
  resource: ["n. 资源；物力财力", "n. 对策；应对手段"],
  community: ["n. 社区；社群", "n. 共同体；群体认同"],
  sustainable: ["adj. 可持续的", "adj. 可长期维持的"],
  innovation: ["n. 创新；革新", "n. 新方法；新制度"],
  efficiency: ["n. 效率；效能", "n. 功效（设备/流程）"],
  collaboration: ["n. 协作；合作", "n. 联合创作；协同"],
  interpretation: ["n. 解释；阐释", "n. 演绎；表演诠释"],
  practice: ["n. 实践；练习", "n. 惯例；做法", "v. 练习；实行"],
  framework: ["n. 框架；结构", "n. 体系；基本思路"],
  pattern: ["n. 模式；规律", "n. 图案；样板"],
  insight: ["n. 洞察；深刻理解", "n. 见解；领悟"],
  issue: ["n. 问题；议题", "n. 发行；发布（报刊等）"],
  factor: ["n. 因素；要素", "n. 因子（数学/科学）"],
  challenge: ["n. 挑战；难题", "v. 质疑；向…挑战"],
  solution: ["n. 解决方案", "n. 溶液（化学）"],
  reflect: ["v. 反映；体现", "v. 反思；认真思考"],
  address: ["v. 处理；应对", "n. 地址", "v. 向…讲话"],
  recent: ["adj. 最近的；新近的", "adj. 近代的；近期发生的"],
  educators: ["n. 教育工作者（复数）", "n. 教育者群体"],
  educator: ["n. 教育工作者", "n. 教育家；教师（语境）"],
  closer: ["adj. 更近的；更紧密的", "adv. 更接近地（比较级）"],
  transportation: ["n. 交通运输", "n. 运输系统；交通方式"],
  climate: ["n. 气候", "n. 氛围；环境趋势（引申）"],
  influences: ["v. 影响（第三人称单数）", "n. 影响力（复数语境）"],
  years: ["n. 年（复数）", "n. 年代；时期（引申）"],
  urban: ["adj. 城市的", "adj. 都市化相关的"],
  classroom: ["n. 教室", "n. 课堂教学场景"],
  learning: ["n. 学习过程；学问", "adj. 学习相关的"],
  reading: ["n. 阅读；阅读能力", "n. 阅读材料；读物（语境）", "n.（考试）阅读题型"],
  technology: ["n. 技术；工艺", "n. 科技手段"],
  attention: ["n. 注意力", "n. 关注；重视"],
  students: ["n. 学生（复数）", "n. 学习者群体"],
  teacher: ["n. 教师", "n. 指导者（引申）"],
  teachers: ["n. 教师（复数）", "n. 教学人员群体"],
  outcomes: ["n. 结果（复数）", "n. 学习产出（教育语境）"],
  paid: ["v. 支付（pay 的过去式/过去分词）", "adj. 有偿的；已付费的"],
  student: ["n. 学生", "n. 学习者；研修者"],
  influence: ["n. 影响；作用", "v. 影响；对…产生作用"],
  compare: ["v. 比较；对照", "v. 比拟（引申）"],
  complexity: ["n. 复杂性", "n. 复杂结构"],
  comprehension: ["n. 理解；领会", "n. 阅读理解能力"],
  indicates: ["v. 表明；显示", "v. 指示；暗示"],
  meaningful: ["adj. 有意义的", "adj. 有价值的；有内涵的"],
  effective: ["adj. 有效的", "adj. 见效的；生效的"]
};

const GRAMMAR_CONNECTORS = [
  "because", "although", "though", "while", "whereas", "which", "that", "when", "if", "unless", "since", "who", "whose", "whom"
];
