import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Compass, Route, Scale, Sparkles } from "lucide-react";
import { courses } from "../api";
import { useAppStore } from "../store";

const stageModules = {
  CANTONESE: [
    {
      stage: "阶段 01",
      modules: ["茶餐厅点餐", "地铁问路", "街市购物", "日常寒暄"]
    },
    {
      stage: "阶段 02",
      modules: ["预约与改期", "物业/客服沟通", "就医挂号", "电话说明问题"]
    },
    {
      stage: "阶段 03",
      modules: ["社交邀约", "活动复盘", "表达喜好", "温和拒绝"]
    },
    {
      stage: "阶段 04",
      modules: ["自我介绍", "会议发言", "进度同步", "确认分工"]
    },
    {
      stage: "阶段 05",
      modules: ["汇报进展", "跨团队协调", "提出建议", "反馈修订"]
    },
    {
      stage: "阶段 06",
      modules: ["结构化陈述", "比较与取舍", "回答追问", "观点扩展"]
    },
    {
      stage: "阶段 07",
      modules: ["社会议题", "价值判断", "立场辩证", "总结升华"]
    }
  ],
  ENGLISH: [
    {
      stage: "Stage 01",
      modules: ["Ordering food", "Asking directions", "Shopping requests", "Daily greetings"]
    },
    {
      stage: "Stage 02",
      modules: ["Booking & rescheduling", "Service support", "Clinic communication", "Phone clarification"]
    },
    {
      stage: "Stage 03",
      modules: ["Social invitations", "Event recap", "Preference expression", "Polite refusal"]
    },
    {
      stage: "Stage 04",
      modules: ["Self-introduction", "Meeting updates", "Task alignment", "Ownership check"]
    },
    {
      stage: "Stage 05",
      modules: ["Project reporting", "Cross-team sync", "Proposal framing", "Feedback handling"]
    },
    {
      stage: "Stage 06",
      modules: ["Fluency drills", "Coherence linking", "Lexical range", "Pronunciation control"]
    },
    {
      stage: "Stage 07",
      modules: ["Abstract topic stance", "Evidence support", "Counter-argument", "Conclusion impact"]
    }
  ]
} as const;

const routeStages = {
  CANTONESE: [
    { title: "阶段 01", detail: "生存沟通：问路 / 点餐 / 购物" },
    { title: "阶段 02", detail: "生活协作：预约 / 求助 / 事务办理" },
    { title: "阶段 03", detail: "社交表达：寒暄 / 邀约 / 观点交换" },
    { title: "阶段 04", detail: "职场基础：会议跟进 / 邮件口语化" },
    { title: "阶段 05", detail: "职场进阶：汇报 / 协调 / 反馈" },
    { title: "阶段 06", detail: "公开表达：陈述 / 论证 / 回应追问" },
    { title: "阶段 07", detail: "高阶话题：社会议题 / 立场辩证" }
  ],
  ENGLISH: [
    { title: "Stage 01", detail: "Daily survival: directions / food / shopping" },
    { title: "Stage 02", detail: "Daily tasks: booking / requests / service calls" },
    { title: "Stage 03", detail: "Social flow: small talk / invitations / opinions" },
    { title: "Stage 04", detail: "Workplace basics: meetings / status updates" },
    { title: "Stage 05", detail: "Workplace growth: negotiation / feedback" },
    { title: "Stage 06", detail: "Exam-oriented speaking: fluency & coherence" },
    { title: "Stage 07", detail: "IELTS topics: abstract discussion & argument" }
  ]
} as const;

const stageThresholds = [0, 120, 280, 480, 720, 980, 1280, 1680];

export function CoursesPage() {
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
  const [error, setError] = useState("");
  const list = useAppStore((s) => s.courses);
  const theaters = useAppStore((s) => s.theaters);
  const user = useAppStore((s) => s.user);
  const latestResult = useAppStore((s) => s.result);
  const roleplay = useAppStore((s) => s.roleplay);
  const setCourses = useAppStore((s) => s.setCourses);
  const navigate = useNavigate();

  const learningProgress = useMemo(() => {
    const completedCourses = list.filter((item) => item.isActive).length;
    const practiceCount = theaters.filter((item) => item.language === language).length + (roleplay ? roleplay.turnIndex + 1 : 0);
    const accuracy = latestResult && latestResult.totalCount > 0
      ? latestResult.correctCount / latestResult.totalCount
      : Math.min(1, Math.max(0, (roleplay?.currentScore ?? 0) / 100));

    const courseXP = completedCourses * 70;
    const practiceXP = practiceCount * 15;
    const accuracyXP = Math.round(accuracy * 400);
    const baseXP = user?.totalXP ?? 0;
    const stageXP = baseXP + courseXP + practiceXP + accuracyXP;

    let stageIndex = stageThresholds.length - 1;
    for (let i = 0; i < stageThresholds.length - 1; i += 1) {
      if (stageXP >= stageThresholds[i] && stageXP < stageThresholds[i + 1]) {
        stageIndex = i;
        break;
      }
    }

    const currentStart = stageThresholds[stageIndex];
    const nextTarget = stageThresholds[Math.min(stageIndex + 1, stageThresholds.length - 1)];
    const denominator = Math.max(1, nextTarget - currentStart);
    const currentPercent = Math.min(100, Math.max(0, Math.round(((stageXP - currentStart) / denominator) * 100)));

    return {
      completedCourses,
      practiceCount,
      accuracy,
      stageXP,
      stageIndex,
      currentPercent,
      currentStart,
      nextTarget
    };
  }, [language, list, theaters, roleplay, latestResult, user]);

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
              <Route size={14} /> {language === "CANTONESE" ? "粤语：生活交流 → 职场表达 → 进阶话题" : "英语：日常场景 → 职场交流 → 雅思口语"}
            </p>
            <small>
              当前等级：Lv.{learningProgress.stageIndex + 1} · 总经验 {learningProgress.stageXP}
              {learningProgress.stageIndex < routeStages[language].length ? ` · 距离下一阶段 ${Math.max(0, learningProgress.nextTarget - learningProgress.stageXP)} XP` : ""}
            </small>
          </div>
          <div className="route-tabs">
            <button className={language === "CANTONESE" ? "route-tab active" : "route-tab"} onClick={() => setLanguage("CANTONESE")}>粤语</button>
            <button className={language === "ENGLISH" ? "route-tab active" : "route-tab"} onClick={() => setLanguage("ENGLISH")}>英语</button>
          </div>
        </div>

        <div className="route-grid" style={{ marginBottom: 12 }}>
          {routeStages[language].map((stage, index) => {
            const unlocked = index <= learningProgress.stageIndex;
            const progress = index < learningProgress.stageIndex ? 100 : index === learningProgress.stageIndex ? learningProgress.currentPercent : 0;
            return (
              <article key={stage.title} className="route-point" style={unlocked ? undefined : { opacity: 0.55 }}>
                <div className="row" style={{ justifyContent: "space-between" }}>
                  <strong>{stage.title}</strong>
                  <small>{progress}%</small>
                </div>
                <small>{stage.detail}</small>
                <div className="mini-progress" aria-hidden>
                  <span style={{ width: `${progress}%` }} />
                </div>
              </article>
            );
          })}
        </div>

        <article className="stage-banner" style={{ marginBottom: 12 }}>
          <strong>经验计算说明</strong>
          <p style={{ margin: "6px 0 0" }}>经验分 = 课程完成数 × 70 + 练习数 × 15 + 正确率加成（最高 400）+ 用户基础经验</p>
          <p style={{ margin: "6px 0 0" }}>
            当前统计：课程完成 {learningProgress.completedCourses} / 练习次数 {learningProgress.practiceCount} / 正确率 {(learningProgress.accuracy * 100).toFixed(0)}%
          </p>
        </article>

        <div className="row">
          <button onClick={() => navigate("/generate")}>去生成剧场</button>
          <button className="btn-ghost" onClick={() => navigate("/reading")}>阅读训练</button>
          <button className="btn-ghost" onClick={() => navigate("/library")}>我的剧场库</button>
        </div>

        <h3 style={{ margin: "14px 0 8px" }}>{language === "CANTONESE" ? "阶段小节" : "Stage Modules"}</h3>
        <div className="route-grid" style={{ marginBottom: 12 }}>
          {stageModules[language].map((stage, stageIndex) => {
            const unlocked = stageIndex <= learningProgress.stageIndex;
            return (
              <article key={stage.stage} className="route-point" style={unlocked ? undefined : { opacity: 0.55 }}>
                <div className="row" style={{ justifyContent: "space-between", marginBottom: 4 }}>
                  <strong>{stage.stage}</strong>
                  <small>{unlocked ? "已解锁" : "未解锁"}</small>
                </div>
                <div className="tag-row">
                  {stage.modules.map((module) => (
                    <button
                      key={module}
                      type="button"
                      className="tag-chip"
                      disabled={!unlocked}
                      onClick={() => navigate(`/generate?language=${language}&topic=${encodeURIComponent(module)}`)}
                    >
                      {module}
                    </button>
                  ))}
                </div>
                <div className="row" style={{ marginTop: 8 }}>
                  <button
                    type="button"
                    className="btn-ghost"
                    disabled={!unlocked}
                    onClick={() =>
                      navigate(`/generate?language=${language}&topic=${encodeURIComponent(stage.modules[0])}`)
                    }
                  >
                    进入本阶段学习
                  </button>
                </div>
              </article>
            );
          })}
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
