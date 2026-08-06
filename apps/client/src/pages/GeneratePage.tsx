import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { motion } from "framer-motion";
import { Clapperboard, Compass, Languages, Sparkles } from "lucide-react";
import { generateTheater, getTheater, getVoiceProfiles } from "../api";
import { AICreditCostNotice } from "../components/AICreditCostNotice";
import { useAppStore } from "../store";
import type { VoiceProfile } from "../types";

const GENERATION_STATUS_STEPS = [
  {
    label: "构思角色设定",
    hint: "角色关系正在润色，剧情张力正在上升。",
    Icon: Sparkles
  },
  {
    label: "生成对话内容",
    hint: "台词语气与场景细节正在对齐。",
    Icon: Languages
  },
  {
    label: "合成语音",
    hint: "声音节奏和停顿点正在优化。",
    Icon: Clapperboard
  },
  {
    label: "准备学习路径",
    hint: "学习建议与复习顺序即将就绪。",
    Icon: Compass
  }
] as const;

const GENERATION_PROGRESS_TICK_MS = 500;
const GENERATION_PROGRESS_CAP = 94;
const GENERATION_AVERAGE_DURATION_MS = 70000;
const GENERATION_STATUS_MILESTONES = [0, 0.18, 0.48, 0.82] as const;
const THEATER_STATUS_POLL_MS = 1500;

export function parseDifficultyText(value: string): { value?: number; error?: string } {
  const normalized = value.trim();
  if (!normalized) return { error: "请输入难度。" };
  if (!/^\d+(?:\.\d+)?$/.test(normalized)) return { error: "难度必须是数字。" };
  const difficulty = Number(normalized);
  if (difficulty < 4 || difficulty > 8) return { error: "难度需在 4.0–8.0 之间。" };
  if (!Number.isInteger((difficulty - 4) * 2)) return { error: "难度请按 0.5 为间隔填写。" };
  return { value: difficulty };
}

function resolveGenerationStatusIndex(progressRatio: number): number {
  for (let index = GENERATION_STATUS_MILESTONES.length - 1; index >= 0; index -= 1) {
    if (progressRatio >= GENERATION_STATUS_MILESTONES[index]) {
      return index;
    }
  }
  return 0;
}

const routeMap = {
  CANTONESE: {
    title: "粤语学习路线",
    subtitle: "从茶餐厅对话到口语场景，逐级提升听力与表达",
    points: [
      { title: "阶段 01", detail: "日常交流：茶餐厅叫餐 / 地铁问路" },
      { title: "阶段 02", detail: "职场语境：见工面试 / 同事倾 project" },
      { title: "阶段 03", detail: "时事专题：人物描述 / 城市文化讨论" },
      { title: "阶段 04", detail: "高阶表达：时事观点 + 辩论组织" }
    ],
    topicSeeds: ["讨论香港茶餐厅文化", "搭地铁问路", "街市买菜讲价", "描述一个你尊敬的人"]
  },
  ENGLISH: {
    title: "English Learning Route",
    subtitle: "Build fluency with daily interactions, workplace talk, and IELTS tasks",
    points: [
      { title: "Stage 01", detail: "Daily language: coffee shop ordering / city directions" },
      { title: "Stage 02", detail: "Workplace talk: interview / team meeting" },
      { title: "Stage 03", detail: "IELTS topics: admire a person / memorable journey" },
      { title: "Stage 04", detail: "Debate drills: AI in education / climate discussion" }
    ],
    topicSeeds: [
      "Ordering at a coffee shop",
      "Asking for directions",
      "Describe a memorable journey",
      "Discuss the impact of AI"
    ]
  }
} as const;

const stageTopicSeeds = {
  CANTONESE: [
    ["茶餐厅点餐", "地铁问路", "街市购物", "日常寒暄"],
    ["预约与改期", "物业/客服沟通", "就医挂号", "电话说明问题"],
    ["社交邀约", "活动复盘", "表达喜好", "温和拒绝"],
    ["自我介绍", "会议发言", "进度同步", "确认分工"],
    ["汇报进展", "跨团队协调", "提出建议", "反馈修订"],
    ["结构化陈述", "比较与取舍", "回答追问", "观点扩展"],
    ["社会议题", "价值判断", "立场辩证", "总结升华"]
  ],
  ENGLISH: [
    ["Ordering food", "Asking directions", "Shopping requests", "Daily greetings"],
    ["Booking & rescheduling", "Service support", "Clinic communication", "Phone clarification"],
    ["Social invitations", "Event recap", "Preference expression", "Polite refusal"],
    ["Self-introduction", "Meeting updates", "Task alignment", "Ownership check"],
    ["Project reporting", "Cross-team sync", "Proposal framing", "Feedback handling"],
    ["Fluency drills", "Coherence linking", "Lexical range", "Pronunciation control"],
    ["Abstract topic stance", "Evidence support", "Counter-argument", "Conclusion impact"]
  ]
} as const;

export function GeneratePage() {
  const [searchParams] = useSearchParams();
  const presetLanguage = searchParams.get("language") === "ENGLISH" ? "ENGLISH" : "CANTONESE";
  const presetTopic = searchParams.get("topic")?.trim() ?? "";
  const rawStage = Number.parseInt(searchParams.get("stage") ?? "0", 10);
  const maxStageIndex = stageTopicSeeds[presetLanguage].length - 1;
  const presetStage = Number.isFinite(rawStage) && rawStage >= 0 ? Math.min(rawStage, maxStageIndex) : 0;

  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">(presetLanguage);
  const [activeStage, setActiveStage] = useState(presetStage);
  const initialSeed = stageTopicSeeds[presetLanguage][Math.min(presetStage, stageTopicSeeds[presetLanguage].length - 1)]?.[0]
    ?? routeMap[presetLanguage].topicSeeds[0];
  const [topic, setTopic] = useState(presetTopic || initialSeed);
  const [difficultyText, setDifficultyText] = useState("5.5");
  const [difficultyError, setDifficultyError] = useState<string | null>(null);
  const [mode, setMode] = useState<"LISTENING" | "ROLEPLAY" | "APPRECIATION">("LISTENING");
  const [voiceMode, setVoiceMode] = useState<"AUTO" | "LIBRARY">("AUTO");
  const [voiceProfiles, setVoiceProfiles] = useState<VoiceProfile[]>([]);
  const [selectedVoiceProfileIds, setSelectedVoiceProfileIds] = useState<string[]>([]);
  const [progress, setProgress] = useState(0);
  const [statusIndex, setStatusIndex] = useState(0);
  const [pendingTheaterId, setPendingTheaterId] = useState<string | null>(null);
  const [generationError, setGenerationError] = useState("");
  const loading = useAppStore((s) => s.loading);
  const setLoading = useAppStore((s) => s.setLoading);
  const setTheater = useAppStore((s) => s.setTheater);
  const navigate = useNavigate();
  const difficultyInputRef = useRef<HTMLInputElement>(null);

  const routeInfo = useMemo(() => routeMap[language], [language]);
  const stageSeeds = useMemo(() => {
    const langSeeds = stageTopicSeeds[language];
    const index = Math.min(activeStage, langSeeds.length - 1);
    return langSeeds[index] ?? routeMap[language].topicSeeds;
  }, [activeStage, language]);
  const readyVoiceProfiles = useMemo(
    () => voiceProfiles.filter((profile) => profile.status === "READY" && profile.language === language),
    [language, voiceProfiles]
  );

  useEffect(() => {
    let active = true;
    void getVoiceProfiles()
      .then((profiles) => {
        if (active) setVoiceProfiles(profiles);
      })
      .catch((error) => console.error("load voice profiles failed", error));
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!loading) {
      setProgress(0);
      setStatusIndex(0);
      return;
    }
    const startedAt = Date.now();
    const timer = window.setInterval(() => {
      const elapsedMs = Date.now() - startedAt;
      const progressRatio = Math.min(elapsedMs / GENERATION_AVERAGE_DURATION_MS, 1);
      const easedRatio = 1 - Math.pow(1 - progressRatio, 1.15);
      const nextProgress = Math.min(GENERATION_PROGRESS_CAP, Math.round(easedRatio * GENERATION_PROGRESS_CAP));
      setProgress((value) => (nextProgress > value ? nextProgress : value));
      setStatusIndex((value) => Math.max(value, resolveGenerationStatusIndex(progressRatio)));
    }, GENERATION_PROGRESS_TICK_MS);

    return () => {
      window.clearInterval(timer);
    };
  }, [loading]);

  useEffect(() => {
    if (!pendingTheaterId) {
      return;
    }
    const theaterID = pendingTheaterId;
    let cancelled = false;

    async function pollGeneratedTheater() {
      try {
        while (!cancelled) {
          const theater = await getTheater(theaterID);
          if (cancelled) {
            return;
          }
          setTheater(theater);
          if (theater.status === "READY") {
            setStatusIndex(GENERATION_STATUS_STEPS.length - 1);
            setProgress(100);
            setPendingTheaterId(null);
            setLoading(false);
            navigate(`/theater/${theater.id}`);
            return;
          }
          if (theater.status === "FAILED") {
            setPendingTheaterId(null);
            setLoading(false);
            setGenerationError("剧场生成失败，请稍后重试。");
            return;
          }
          await new Promise((resolve) => window.setTimeout(resolve, THEATER_STATUS_POLL_MS));
        }
      } catch (error) {
        if (cancelled) {
          return;
        }
        console.error("poll theater status failed", error);
        setPendingTheaterId(null);
        setLoading(false);
        setGenerationError("剧场状态查询失败，请稍后重试。");
      }
    }

    void pollGeneratedTheater();

    return () => {
      cancelled = true;
      setLoading(false);
    };
  }, [navigate, pendingTheaterId, setLoading, setTheater]);

  async function handleGenerate(event: FormEvent) {
    event.preventDefault();
    setGenerationError("");
    setPendingTheaterId(null);
    setStatusIndex(0);
    const parsedDifficulty = parseDifficultyText(difficultyText);
    if (parsedDifficulty.error || parsedDifficulty.value === undefined) {
      setDifficultyError(parsedDifficulty.error ?? "请输入有效难度。");
      difficultyInputRef.current?.focus();
      return;
    }
    if (voiceMode === "LIBRARY" && selectedVoiceProfileIds.length === 0) {
      setGenerationError("请至少选择一个已完成的角色音色，或改为自动生成音色。");
      return;
    }
    setLoading(true);
    try {
      const theater = await generateTheater({
        language,
        topic,
        difficulty: parsedDifficulty.value,
        mode,
        voiceMode,
        voiceProfileIds: voiceMode === "LIBRARY" ? selectedVoiceProfileIds : []
      });
      setTheater(theater);
      if (theater.status === "READY") {
        setProgress(100);
        setLoading(false);
        navigate(`/theater/${theater.id}`);
        return;
      }
      if (theater.status === "FAILED") {
        setLoading(false);
        setGenerationError("剧场生成失败，请稍后重试。");
        return;
      }
      setPendingTheaterId(theater.id);
    } catch (e) {
      console.error("generate theater failed", e);
      setGenerationError("剧场创建失败，请稍后重试。");
      setLoading(false);
    }
  }

  function toggleVoiceProfile(profileID: string) {
    setSelectedVoiceProfileIds((current) => {
      if (current.includes(profileID)) return current.filter((id) => id !== profileID);
      if (current.length >= 3) {
        setGenerationError("一次最多为前三个角色分配 3 个音色。");
        return current;
      }
      setGenerationError("");
      return [...current, profileID];
    });
  }

  return (
    <main className="page-center">
      <motion.section className="card route-shell stage-shell" initial={{ opacity: 0, y: 18 }} animate={{ opacity: 1, y: 0 }}>
        <form onSubmit={handleGenerate} className="stage-main">
          <div className="route-header">
            <div>
              <h2>AI 小剧场生成台</h2>
              <p>{routeInfo.subtitle}</p>
            </div>
            <div className="route-tabs">
              <button
                type="button"
                className={language === "CANTONESE" ? "route-tab active" : "route-tab"}
                onClick={() => {
                  setLanguage("CANTONESE");
                  setActiveStage(0);
                  setTopic(stageTopicSeeds.CANTONESE[0][0]);
                  setSelectedVoiceProfileIds([]);
                }}
              >
                粤语
              </button>
              <button
                type="button"
                className={language === "ENGLISH" ? "route-tab active" : "route-tab"}
                onClick={() => {
                  setLanguage("ENGLISH");
                  setActiveStage(0);
                  setTopic(stageTopicSeeds.ENGLISH[0][0]);
                  setSelectedVoiceProfileIds([]);
                }}
              >
                英语
              </button>
            </div>
          </div>

          <div className="row" style={{ marginTop: 8 }}>
            <small>当前阶段预设：{language === "CANTONESE" ? `阶段 ${String(activeStage + 1).padStart(2, "0")}` : `Stage ${String(activeStage + 1).padStart(2, "0")}`}</small>
          </div>

          <div className="section-kicker">Stage Composer</div>
          <h3>{routeInfo.title}</h3>
          <div className="route-grid">
            {routeInfo.points.map((point) => (
              <article key={point.title} className="route-point">
                <strong>{point.title}</strong>
                <small>{point.detail}</small>
              </article>
            ))}
          </div>

          <div className="tag-row" style={{ marginTop: 10 }}>
            {stageSeeds.map((seed) => (
              <button key={seed} type="button" className="tag-chip" onClick={() => setTopic(seed)}>
                {seed}
              </button>
            ))}
          </div>

          <div className="row" style={{ marginTop: 12 }}>
            <label style={{ flex: 1, minWidth: 180 }}>
              <span>主题</span>
              <input value={topic} onChange={(e) => setTopic(e.target.value)} />
            </label>
            <label style={{ minWidth: 130 }}>
              <span>难度</span>
              <input
                ref={difficultyInputRef}
                type="number"
                step="0.5"
                min={4}
                max={8}
                inputMode="decimal"
                value={difficultyText}
                aria-invalid={Boolean(difficultyError)}
                aria-describedby="difficulty-hint difficulty-error"
                onChange={(e) => {
                  setDifficultyText(e.target.value);
                  setDifficultyError(null);
                }}
                onBlur={() => {
                  const parsedDifficulty = parseDifficultyText(difficultyText);
                  if (parsedDifficulty.error || parsedDifficulty.value === undefined) {
                    setDifficultyError(parsedDifficulty.error ?? "请输入有效难度。");
                    return;
                  }
                  setDifficultyText(parsedDifficulty.value.toFixed(1));
                  setDifficultyError(null);
                }}
              />
              <small id="difficulty-hint">4.0–8.0，每次 0.5</small>
              {difficultyError ? <span id="difficulty-error" className="field-error" role="alert">{difficultyError}</span> : null}
            </label>
            <label style={{ minWidth: 160 }}>
              <span>模式</span>
              <select value={mode} onChange={(e) => setMode(e.target.value as typeof mode)}>
                <option value="LISTENING">听力理解</option>
                <option value="ROLEPLAY">角色扮演</option>
                <option value="APPRECIATION">欣赏模式</option>
              </select>
            </label>
          </div>

          <section className="voice-selection-panel" aria-label="剧场角色音色">
            <div className="voice-selection-head">
              <div>
                <strong>角色音色</strong>
                <p>可由系统自动匹配，或复用你在个人中心设计并保存的角色音色。</p>
              </div>
              <div className="route-tabs" role="radiogroup" aria-label="音色来源">
                <button type="button" className={voiceMode === "AUTO" ? "route-tab active" : "route-tab"} onClick={() => setVoiceMode("AUTO")}>自动生成</button>
                <button type="button" className={voiceMode === "LIBRARY" ? "route-tab active" : "route-tab"} onClick={() => setVoiceMode("LIBRARY")}>从音色库选择</button>
              </div>
            </div>
            {voiceMode === "LIBRARY" ? (
              readyVoiceProfiles.length > 0 ? (
                <div className="voice-profile-choice-grid">
                  {readyVoiceProfiles.map((profile) => {
                    const selected = selectedVoiceProfileIds.includes(profile.id);
                    return (
                      <button key={profile.id} type="button" className={selected ? "voice-profile-choice active" : "voice-profile-choice"} onClick={() => toggleVoiceProfile(profile.id)} aria-pressed={selected}>
                        <strong>{profile.name}</strong>
                        <small>{profile.prompt}</small>
                        <span>{selected ? `角色 ${selectedVoiceProfileIds.indexOf(profile.id) + 1}` : "点击分配角色"}</span>
                      </button>
                    );
                  })}
                </div>
              ) : (
                <p className="voice-library-empty">当前语言没有可用音色。<button type="button" className="text-button" onClick={() => navigate("/voices/create")}>创建音色</button> 后可返回这里分配角色。</p>
              )
            ) : null}
          </section>

          <AICreditCostNotice action="THEATER_GENERATION" />
          <div className="row" style={{ marginTop: 14 }}>
            <button type="submit" disabled={loading}>
              {loading ? "剧场生成中..." : "开始生成剧场"}
            </button>
            <button type="button" className="btn-ghost" onClick={() => navigate("/library")}>进入剧场库</button>
            <button type="button" className="btn-ghost" onClick={() => navigate("/courses")}>课程中心</button>
          </div>
          {generationError ? <p style={{ marginTop: 12, color: "#a6422b" }}>{generationError}</p> : null}
        </form>

        <aside className="floating-panel">
          <div className="row" style={{ alignItems: "center", justifyContent: "space-between" }}>
            <h3 style={{ margin: 0 }}>生成进度</h3>
            <div className={loading ? "spin-core" : "spin-core paused"} />
          </div>
          <div className="progress-shell">
            <div className="progress-bar">
              <div className="progress-value" style={{ width: `${progress}%` }} />
            </div>
            <p>{progress}%</p>
            {GENERATION_STATUS_STEPS.map((step, idx) => {
              const Icon = step.Icon;
              const stepClass = !loading
                ? "status-step"
                : idx === statusIndex
                  ? "status-step active"
                  : idx < statusIndex
                    ? "status-step done"
                    : "status-step";
              const stepText = !loading
                ? `${step.label}（待生成）`
                : idx === statusIndex
                  ? `正在${step.label}...`
                  : idx < statusIndex
                    ? `${step.label}（已完成）`
                    : `${step.label}（待生成）`;
              return (
                <p key={step.label} className={stepClass}>
                  <Icon size={14} /> {stepText}
                </p>
              );
            })}
            <p key={`hint-${loading ? statusIndex : "idle"}`} className="status-dynamic-hint">
              {loading ? GENERATION_STATUS_STEPS[statusIndex]?.hint : "点击“开始生成剧场”后，将按阶段依次生成并自动推进。"}
            </p>
            <small className="status-soft-note">剧场会先异步生成，待语音完成后自动进入详情页</small>
          </div>
        </aside>
      </motion.section>
    </main>
  );
}
