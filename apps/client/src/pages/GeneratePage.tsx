import { FormEvent, useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { motion } from "framer-motion";
import { Clapperboard, Compass, Languages, Sparkles } from "lucide-react";
import { generateTheater } from "../api";
import { useAppStore } from "../store";

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

const GENERATION_PROGRESS_TICK_MS = 320;
const GENERATION_PROGRESS_STEP = 2;
const GENERATION_STATUS_TICK_MS = 2200;
const GENERATION_PROGRESS_CAP = 92;

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
  const [difficulty, setDifficulty] = useState(5.5);
  const [mode, setMode] = useState<"LISTENING" | "ROLEPLAY" | "APPRECIATION">("LISTENING");
  const [progress, setProgress] = useState(0);
  const [statusIndex, setStatusIndex] = useState(0);
  const loading = useAppStore((s) => s.loading);
  const setLoading = useAppStore((s) => s.setLoading);
  const setTheater = useAppStore((s) => s.setTheater);
  const navigate = useNavigate();

  const routeInfo = useMemo(() => routeMap[language], [language]);
  const stageSeeds = useMemo(() => {
    const langSeeds = stageTopicSeeds[language];
    const index = Math.min(activeStage, langSeeds.length - 1);
    return langSeeds[index] ?? routeMap[language].topicSeeds;
  }, [activeStage, language]);

  useEffect(() => {
    if (!loading) {
      setProgress(0);
      setStatusIndex(0);
      return;
    }
    const timer = window.setInterval(() => {
      setProgress((value) => (value >= GENERATION_PROGRESS_CAP ? value : value + GENERATION_PROGRESS_STEP));
    }, GENERATION_PROGRESS_TICK_MS);

    const statusTimer = window.setInterval(() => {
      setStatusIndex((prev) => Math.min(prev + 1, GENERATION_STATUS_STEPS.length - 1));
    }, GENERATION_STATUS_TICK_MS);

    return () => {
      window.clearInterval(timer);
      window.clearInterval(statusTimer);
    };
  }, [loading]);

  async function handleGenerate(event: FormEvent) {
    event.preventDefault();
    setStatusIndex(0);
    setLoading(true);
    try {
      const theater = await generateTheater({ language, topic, difficulty, mode });
      setProgress(100);
      setTheater(theater);
      navigate(`/theater/${theater.id}`);
    } catch (e) {
      console.error("generate theater failed", e);
    } finally {
      setLoading(false);
    }
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
                type="number"
                step="0.5"
                min={4}
                max={8}
                value={difficulty}
                onChange={(e) => setDifficulty(Number(e.target.value))}
              />
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

          <div className="row" style={{ marginTop: 14 }}>
            <button type="submit" disabled={loading}>
              {loading ? "剧场生成中..." : "开始生成剧场"}
            </button>
            <button type="button" className="btn-ghost" onClick={() => navigate("/library")}>进入剧场库</button>
            <button type="button" className="btn-ghost" onClick={() => navigate("/courses")}>课程中心</button>
          </div>
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
            <small className="status-soft-note">生成完成后可在剧场中心查看</small>
          </div>
        </aside>
      </motion.section>
    </main>
  );
}
