import { FormEvent, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Clapperboard, Compass, Languages, Sparkles } from "lucide-react";
import { generateTheater } from "../api";
import { useAppStore } from "../store";

const routeMap = {
  CANTONESE: {
    title: "粤语学习路线",
    subtitle: "从茶餐厅对话到雅思口语场景，逐级提升听力与表达",
    points: [
      { title: "阶段 01", detail: "日常交流：茶餐厅叫餐 / 地铁问路" },
      { title: "阶段 02", detail: "职场语境：见工面试 / 同事倾 project" },
      { title: "阶段 03", detail: "雅思专题：人物描述 / 城市文化讨论" },
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

export function GeneratePage() {
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
  const [topic, setTopic] = useState("讨论香港茶餐厅文化");
  const [difficulty, setDifficulty] = useState(5.5);
  const [mode, setMode] = useState<"LISTENING" | "ROLEPLAY" | "APPRECIATION">("LISTENING");
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  const loading = useAppStore((s) => s.loading);
  const setLoading = useAppStore((s) => s.setLoading);
  const setTheater = useAppStore((s) => s.setTheater);
  const navigate = useNavigate();

  const routeInfo = useMemo(() => routeMap[language], [language]);

  useEffect(() => {
    if (!loading) {
      setProgress(0);
      return;
    }
    const timer = window.setInterval(() => {
      setProgress((value) => (value >= 92 ? value : value + 4));
    }, 220);
    return () => {
      window.clearInterval(timer);
    };
  }, [loading]);

  async function handleGenerate(event: FormEvent) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const theater = await generateTheater({ language, topic, difficulty, mode });
      setProgress(100);
      setTheater(theater);
      navigate(`/theater/${theater.id}`);
    } catch (e) {
      setError((e as Error).message);
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
                  setTopic(routeMap.CANTONESE.topicSeeds[0]);
                }}
              >
                粤语
              </button>
              <button
                type="button"
                className={language === "ENGLISH" ? "route-tab active" : "route-tab"}
                onClick={() => {
                  setLanguage("ENGLISH");
                  setTopic(routeMap.ENGLISH.topicSeeds[0]);
                }}
              >
                英语
              </button>
            </div>
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
            {routeInfo.topicSeeds.map((seed) => (
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

          {error ? <p className="error">{error}</p> : null}

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
            <div className="spin-core" />
          </div>
          <div className="progress-shell">
            <div className="progress-bar">
              <div className="progress-value" style={{ width: `${progress}%` }} />
            </div>
            <p>{progress}%</p>
            <p className="status-step"><Sparkles size={14} /> 正在构思角色设定...</p>
            <p className="status-step"><Languages size={14} /> 正在生成对话内容...</p>
            <p className="status-step"><Clapperboard size={14} /> 正在合成语音...</p>
            <p className="status-step"><Compass size={14} /> 正在准备学习路径...</p>
          </div>
        </aside>
      </motion.section>
    </main>
  );
}
