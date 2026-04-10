import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Compass, Route, Scale, Sparkles } from "lucide-react";
import { courses } from "../api";
import { useAppStore } from "../store";

const routeStages = {
  CANTONESE: [
    { title: "阶段 01", detail: "茶餐厅叫餐 / 地铁问路", progress: 36 },
    { title: "阶段 02", detail: "街市讲价 / 日常闲聊", progress: 52 },
    { title: "阶段 03", detail: "面试表达 / 雅思 Part2", progress: 68 },
    { title: "阶段 04", detail: "时事讨论 / 高分表达", progress: 24 }
  ],
  ENGLISH: [
    { title: "Stage 01", detail: "Coffee shop / city direction", progress: 44 },
    { title: "Stage 02", detail: "Shopping talk / social openers", progress: 63 },
    { title: "Stage 03", detail: "Interview / team meeting", progress: 58 },
    { title: "Stage 04", detail: "IELTS speaking / opinion debate", progress: 31 }
  ]
} as const;

export function CoursesPage() {
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
  const [error, setError] = useState("");
  const list = useAppStore((s) => s.courses);
  const setCourses = useAppStore((s) => s.setCourses);
  const navigate = useNavigate();

  useEffect(() => {
    void (async () => {
      setError("");
      try {
        const data = await courses(language);
        setCourses(data);
      } catch (e) {
        setError((e as Error).message);
      }
    })();
  }, [language, setCourses]);

  return (
    <main className="page">
      <section className="card">
        <div className="route-header">
          <div>
            <h2>课程中心</h2>
            <p>
              <Route size={14} /> {language === "CANTONESE" ? "粤语：生活交流到雅思表达" : "英语：日常场景到雅思表达"}
            </p>
          </div>
          <div className="route-tabs">
            <button className={language === "CANTONESE" ? "route-tab active" : "route-tab"} onClick={() => setLanguage("CANTONESE")}>粤语</button>
            <button className={language === "ENGLISH" ? "route-tab active" : "route-tab"} onClick={() => setLanguage("ENGLISH")}>英语</button>
          </div>
        </div>

        <div className="route-grid" style={{ marginBottom: 12 }}>
          {routeStages[language].map((stage) => (
            <article key={stage.title} className="route-point">
              <div className="row" style={{ justifyContent: "space-between" }}>
                <strong>{stage.title}</strong>
                <small>{stage.progress}%</small>
              </div>
              <small>{stage.detail}</small>
              <div className="mini-progress" aria-hidden>
                <span style={{ width: `${stage.progress}%` }} />
              </div>
            </article>
          ))}
        </div>

        <div className="row">
          <button onClick={() => navigate("/generate")}>去生成剧场</button>
          <button className="btn-ghost" onClick={() => navigate("/library")}>我的剧场库</button>
        </div>
        {error ? <p className="error">{error}</p> : null}
        <ul className="dialogue-list">
          {list.map((item) => (
            <motion.li
              key={item.id}
              className="dialogue"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ type: "spring", stiffness: 220, damping: 20 }}
            >
              <div className="row" style={{ justifyContent: "space-between" }}>
                <strong>{item.title}</strong>
                <small><Compass size={12} /> {item.language === "CANTONESE" ? "粤语" : "English"}</small>
              </div>
              <p>{item.description}</p>
              <small>
                <Scale size={12} /> 难度 {item.minLevel}-{item.maxLevel} / {item.category}
              </small>
              <p style={{ margin: "8px 0 0" }}><Sparkles size={12} /> 推荐与对应路线剧场混合练习以提高迁移能力。</p>
            </motion.li>
          ))}
        </ul>
      </section>
    </main>
  );
}
