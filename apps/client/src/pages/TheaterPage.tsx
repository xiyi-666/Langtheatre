import { TouchEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { motion } from "framer-motion";
import {
  BookAudio,
  BookOpenText,
  Captions,
  CirclePause,
  CirclePlay,
  Heart,
  Repeat,
  SkipBack,
  SkipForward,
  Star,
  TimerReset
} from "lucide-react";
import { getTheater, toggleFavorite } from "../api";
import { playClip, speakText } from "../audio";
import { useAppStore } from "../store";

export function TheaterPage() {
  const { id = "" } = useParams();
  const [activeIndex, setActiveIndex] = useState(0);
  const [loading, setLoading] = useState(false);
  const [autoPlay, setAutoPlay] = useState(false);
  const [playbackRate, setPlaybackRate] = useState<0.8 | 1 | 1.2>(1);
  const [loopCurrent, setLoopCurrent] = useState(false);
  const [showSubtitle, setShowSubtitle] = useState(true);
  const [showZhSubtitle, setShowZhSubtitle] = useState(false);
  const [favorite, setFavorite] = useState(false);
  const [gestureHint, setGestureHint] = useState("");
  const [vocabDetail, setVocabDetail] = useState("");
  const touchStartXRef = useRef<number | null>(null);
  const touchStartTimeRef = useRef<number | null>(null);
  const lastTapTimeRef = useRef<number>(0);
  const longPressTimerRef = useRef<number | null>(null);
  const hintTimerRef = useRef<number | null>(null);
  const navigate = useNavigate();
  const theater = useAppStore((s) => s.theater);
  const setTheater = useAppStore((s) => s.setTheater);

  useEffect(() => {
    if (!theater || theater.id !== id) {
      void (async () => {
        const data = await getTheater(id);
        setTheater(data);
        setFavorite(Boolean(data.isFavorite));
      })();
    } else {
      setFavorite(Boolean(theater.isFavorite));
    }
  }, [id, setTheater, theater]);

  useEffect(() => {
    return () => {
      if (longPressTimerRef.current) {
        window.clearTimeout(longPressTimerRef.current);
      }
      if (hintTimerRef.current) {
        window.clearTimeout(hintTimerRef.current);
      }
    };
  }, []);

  const dialogueCount = theater?.dialogues.length ?? 0;
  const progress = dialogueCount > 1 ? Math.round((activeIndex / (dialogueCount - 1)) * 100) : 0;
  const speakers = theater?.characters?.length
    ? theater.characters.slice(0, 2).map((c) => c.name)
    : [...new Set((theater?.dialogues ?? []).map((item) => item.speaker))].slice(0, 2);
  const primaryAction =
    theater?.mode === "ROLEPLAY"
      ? { label: "进入角色扮演", onClick: () => navigate(`/roleplay/${theater?.id ?? id}`), aria: "进入角色扮演页面" }
      : theater?.mode === "APPRECIATION"
        ? { label: "完成欣赏", onClick: () => navigate("/library"), aria: "完成欣赏并返回剧场库" }
        : { label: "继续答题", onClick: () => navigate(`/quiz/${theater?.id ?? id}`), aria: "进入答题页面" };

  const learningCards = useMemo(() => {
    if (!theater) return [] as Array<{ term: string; note: string; detail: string }>;

    const stopWords = new Set([
      "the", "a", "an", "is", "are", "to", "of", "and", "in", "for", "on", "with", "that", "this", "it", "you", "we", "i",
      "我", "你", "佢", "是", "的", "了", "在", "有", "和", "就", "都", "而", "及", "與", "呢", "啊", "嗎", "啦", "呀"
    ]);

    const baseText = [theater.topic, theater.sceneDescription ?? "", ...theater.dialogues.map((d) => d.text)].join(" ");
    const candidates = baseText
      .split(/[^\p{L}\p{N}'’-]+/u)
      .map((token) => token.trim())
      .filter((token) => token.length >= (theater.language === "ENGLISH" ? 4 : 2))
      .filter((token) => !stopWords.has(token.toLowerCase()));

    const uniqueTerms = Array.from(new Set(candidates)).slice(0, 6);
    const cards = uniqueTerms.map((term) => {
      if (theater.language === "CANTONESE") {
        return {
          term,
          note: "结合本剧情高频表达提炼",
          detail: `「${term}」来自当前剧情语境。练习建议：先复述原句，再替换同义表达并进行 1 句情景扩展。`
        };
      }
      return {
        term,
        note: "Extracted from current storyline",
        detail: `"${term}" is extracted from this storyline. Practice by paraphrasing the original line, then create one new sentence in the same scene.`
      };
    });

    if (theater.language === "CANTONESE") {
      cards.push(
        {
          term: "语法：时间先后（先…再…）",
          note: "叙事推进常用句式",
          detail: "教学：用“先 + 动作A，再 + 动作B”表达步骤顺序。示例：先落单，再等位。请按剧情自己造 2 句。"
        },
        {
          term: "语法：情态表达（想/要/可以）",
          note: "请求与意图表达",
          detail: "教学：想=愿望，要=需求，可以=许可/能力。示例：我想试下新品；我可以加杯热奶茶吗？"
        }
      );
    } else {
      cards.push(
        {
          term: "Grammar: Sequence (first...then...)",
          note: "Useful for scene narration",
          detail: "Use 'first + action A, then + action B' to describe order. Create 2 sentences based on the current scene."
        },
        {
          term: "Grammar: Modal verbs (can / should / would)",
          note: "Requests and suggestions",
          detail: "Use can for ability/permission, should for advice, would for polite requests. Rewrite one dialogue line with each modal."
        }
      );
    }

    return cards.slice(0, 8);
  }, [theater]);

  function showHint(text: string) {
    setGestureHint(text);
    if (hintTimerRef.current) {
      window.clearTimeout(hintTimerRef.current);
    }
    hintTimerRef.current = window.setTimeout(() => {
      setGestureHint("");
    }, 1000);
  }

  const playDialogue = useCallback(
    async (index: number) => {
      const target = theater?.dialogues[index];
      if (!target) return;
      setLoading(true);
      try {
        if (target.audioUrl) {
          try {
            await playClip(target.audioUrl, playbackRate);
          } catch {
            await speakText(target.text, playbackRate);
          }
        } else {
          await speakText(target.text, playbackRate);
        }
      } catch {
        showHint("音频不可用，请稍后重试");
      } finally {
        setLoading(false);
      }
    },
    [playbackRate, theater]
  );

  async function handlePlayCurrent() {
    await playDialogue(activeIndex);
    if (!loopCurrent) {
      setActiveIndex((value) => Math.min(value + 1, dialogueCount - 1));
    }
  }

  useEffect(() => {
    if (!autoPlay || !theater || dialogueCount === 0) {
      return;
    }
    let disposed = false;
    void (async () => {
      for (let index = 0; index < dialogueCount; index += 1) {
        if (disposed) {
          return;
        }
        setActiveIndex(index);
        await playDialogue(index);
      }
      if (!disposed) {
        setAutoPlay(false);
        showHint("已按顺序播放完成");
      }
    })();
    return () => {
      disposed = true;
    };
  }, [autoPlay, dialogueCount, playDialogue, theater]);

  async function handleToggleFavorite() {
    if (!theater) return;
    const next = !favorite;
    await toggleFavorite(theater.id, next);
    setFavorite(next);
    setTheater({ ...theater, isFavorite: next });
  }

  function handleDialogueTouchStart(event: TouchEvent<HTMLLIElement>) {
    touchStartXRef.current = event.changedTouches[0]?.clientX ?? null;
    touchStartTimeRef.current = Date.now();
  }

  function handleDialogueTouchEnd(event: TouchEvent<HTMLLIElement>, index: number) {
    const startX = touchStartXRef.current;
    const startTime = touchStartTimeRef.current;
    touchStartXRef.current = null;
    touchStartTimeRef.current = null;
    if (startX === null || startTime === null) return;

    const endX = event.changedTouches[0]?.clientX ?? startX;
    const deltaX = endX - startX;
    const deltaTime = Date.now() - startTime;
    const now = Date.now();

    if (Math.abs(deltaX) > 50 && deltaTime < 420) {
      if (deltaX < 0) {
        setActiveIndex((value) => Math.min(value + 1, dialogueCount - 1));
        showHint("已切到下一句");
      } else {
        setActiveIndex((value) => Math.max(value - 1, 0));
        showHint("已切到上一句");
      }
      lastTapTimeRef.current = 0;
      return;
    }

    if (now - lastTapTimeRef.current < 320) {
      setActiveIndex(index);
      void playDialogue(index);
      showHint("双击重播当前句");
      lastTapTimeRef.current = 0;
      return;
    }
    lastTapTimeRef.current = now;
  }

  function startVocabLongPress(detail: string) {
    if (longPressTimerRef.current) {
      window.clearTimeout(longPressTimerRef.current);
    }
    longPressTimerRef.current = window.setTimeout(() => {
      setVocabDetail(detail);
    }, 520);
  }

  function stopVocabLongPress() {
    if (longPressTimerRef.current) {
      window.clearTimeout(longPressTimerRef.current);
      longPressTimerRef.current = null;
    }
  }

  if (!theater) {
    return <main className="page-center">加载剧场中...</main>;
  }

  return (
    <main className="page">
      <section className="card theater-shell stage-shell">
        <div>
          <header className="route-header">
            <div>
              <h2>{theater.topic}</h2>
              <p>难度 {theater.difficulty} | 模式 {theater.mode}</p>
            </div>
            <button className="btn-ghost" onClick={() => navigate("/library")}>返回剧场库</button>
          </header>

          <section className="stage-banner">
            <strong>场景</strong>
            <p>{theater.sceneDescription || (theater.language === "CANTONESE" ? "香港旺角茶餐厅，午餐高峰时段" : "Central London cafe, lunchtime rush")}</p>
            <div className="actors">
              {speakers.map((speaker, index) => (
                <article key={speaker} className="actor-card">
                  <div className="actor-avatar">{speaker.slice(0, 1)}</div>
                  <strong>{speaker}</strong>
                  <p>{index === 0 ? "引导者" : "学习者"}</p>
                </article>
              ))}
            </div>
          </section>

          <ul className="dialogue-list" style={{ marginTop: 12 }}>
            {theater.dialogues.map((dialogue, index) => {
              const positionClass = index % 2 === 0 ? "speaker-left" : "speaker-right";
              const activeClass = index === activeIndex ? " speaker-active" : "";
              return (
                <motion.li
                  key={`${dialogue.speaker}-${index}`}
                  className={`${positionClass}${activeClass}`}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  onClick={() => setActiveIndex(index)}
                  onTouchStart={handleDialogueTouchStart}
                  onTouchEnd={(event) => handleDialogueTouchEnd(event, index)}
                  onDoubleClick={() => {
                    setActiveIndex(index);
                    void playDialogue(index);
                    showHint("双击重播当前句");
                  }}
                >
                  <strong>{dialogue.speaker}</strong>
                  {showSubtitle ? <p style={{ margin: "6px 0" }}>{dialogue.text}</p> : <p style={{ margin: "6px 0" }}>字幕已隐藏</p>}
                  {showSubtitle && showZhSubtitle ? (
                    <p style={{ margin: "2px 0 0", fontSize: 13, opacity: 0.8 }}>
                      {dialogue.zhSubtitle && dialogue.zhSubtitle.trim() !== "" ? dialogue.zhSubtitle : "暂无中文字幕对照"}
                    </p>
                  ) : null}
                  {index === activeIndex ? (
                    <span className="wave-bars" aria-hidden>
                      <span />
                      <span />
                      <span />
                      <span />
                    </span>
                  ) : null}
                </motion.li>
              );
            })}
          </ul>

          <div className="progress-shell" style={{ marginTop: 12 }}>
            <div className="row" style={{ justifyContent: "space-between" }}>
              <small>播放进度</small>
              <small>
                {activeIndex + 1}/{dialogueCount} | {playbackRate}x
              </small>
            </div>
            <div className="progress-bar">
              <div className="progress-value" style={{ width: `${progress}%` }} />
            </div>
          </div>

          <div className="row" style={{ marginTop: 12 }}>
            <button aria-label="播放上一句" onClick={() => setActiveIndex((value) => Math.max(value - 1, 0))}>
              <SkipBack size={16} /> 上一句
            </button>
            <button aria-label="播放当前句" onClick={handlePlayCurrent} disabled={loading}>
              {loading ? <CirclePause size={16} /> : <CirclePlay size={16} />}
              {loading ? "播放中" : "播放当前句"}
            </button>
            <button
              aria-label="自动循环播放"
              onClick={() => {
                if (autoPlay) {
                  setAutoPlay(false);
                  showHint("已关闭自动播放");
                  return;
                }
                setActiveIndex(0);
                setAutoPlay(true);
                showHint("已开启自动播放（从第一句开始）");
              }}
            >
              {autoPlay ? <CirclePause size={16} /> : <CirclePlay size={16} />} {autoPlay ? "关闭自动播放" : "自动播放"}
            </button>
            <button aria-label="播放下一句" onClick={() => setActiveIndex((value) => Math.min(value + 1, dialogueCount - 1))}>
              <SkipForward size={16} /> 下一句
            </button>
            <button aria-label={primaryAction.aria} onClick={primaryAction.onClick}>
              <BookOpenText size={16} /> {primaryAction.label}
            </button>
          </div>

          <div className="row" style={{ marginTop: 10 }}>
            <button
              type="button"
              className={loopCurrent ? "control-chip active" : "control-chip"}
              onClick={() => setLoopCurrent((value) => !value)}
            >
              <Repeat size={14} /> 循环当前句
            </button>
            <button
              type="button"
              className={showSubtitle ? "control-chip active" : "control-chip"}
              onClick={() => setShowSubtitle((value) => !value)}
            >
              <Captions size={14} /> 字幕
            </button>
            <button
              type="button"
              className={showZhSubtitle ? "control-chip active" : "control-chip"}
              onClick={() => setShowZhSubtitle((value) => !value)}
            >
              <Captions size={14} /> 中文字幕对照
            </button>
            <button
              type="button"
              className={playbackRate === 0.8 ? "control-chip active" : "control-chip"}
              onClick={() => setPlaybackRate(0.8)}
            >
              <TimerReset size={14} /> 0.8x
            </button>
            <button
              type="button"
              className={playbackRate === 1 ? "control-chip active" : "control-chip"}
              onClick={() => setPlaybackRate(1)}
            >
              <TimerReset size={14} /> 1.0x
            </button>
            <button
              type="button"
              className={playbackRate === 1.2 ? "control-chip active" : "control-chip"}
              onClick={() => setPlaybackRate(1.2)}
            >
              <TimerReset size={14} /> 1.2x
            </button>
          </div>
          {gestureHint ? <p className="gesture-hint">{gestureHint}</p> : null}
        </div>

        <aside className="floating-panel">
          <h3>重点词汇与语法</h3>
          {learningCards.map((card, index) => (
            <div
              key={`${card.term}-${index}`}
              className="route-point vocab-card"
              style={index > 0 ? { marginTop: 10 } : undefined}
              onPointerDown={() => startVocabLongPress(card.detail)}
              onPointerUp={stopVocabLongPress}
              onPointerLeave={stopVocabLongPress}
            >
              <strong>{card.term}</strong>
              <small>{card.note}</small>
            </div>
          ))}
          {vocabDetail ? <div className="vocab-popover" role="status">{vocabDetail}</div> : null}
          <button type="button" className={favorite ? "control-chip active" : "control-chip"} onClick={handleToggleFavorite}>
            <Heart size={14} /> {favorite ? "已收藏" : "收藏剧场"}
          </button>
          <p style={{ marginTop: 12 }}>
            <Star size={14} /> 点击任一对话气泡即可切换句子并复听。
          </p>
          <p>
            <BookAudio size={14} /> 当前支持字幕切换、循环复听和速率切换。
          </p>
        </aside>
      </section>
    </main>
  );
}
