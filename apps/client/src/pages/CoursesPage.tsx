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

const stageThresholds = [0, 120, 280, 480, 720, 980, 1280, 1680];

export function CoursesPage() {
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
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
    const totalXP = user?.totalXP ?? 0;
    const learningIndex = courseXP + practiceXP + accuracyXP;
    const stageXP = totalXP;

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
    const totalProgressPercent = Math.min(100, Math.max(0, Math.round((totalXP / stageThresholds[stageThresholds.length - 1]) * 100)));
    const nextLevelRemaining = Math.max(0, nextTarget - stageXP);
    const courseCompletionPercent = list.length > 0 ? Math.min(100, Math.round((completedCourses / list.length) * 100)) : 0;
    const practiceProgressPercent = Math.min(100, practiceCount * 10);
    const learningIndexPercent = Math.min(100, Math.round((learningIndex / 1200) * 100));

    return {
      completedCourses,
      practiceCount,
      accuracy,
      learningIndex,
      totalXP,
      stageXP,
      stageIndex,
      currentPercent,
      currentStart,
      nextTarget,
      nextLevelRemaining,
      totalProgressPercent,
      courseCompletionPercent,
      practiceProgressPercent,
      learningIndexPercent
    };
  }, [language, list, theaters, roleplay, latestResult, user]);

  useEffect(() => {
    void (async () => {
      try {
        const data = await courses(language);
        setCourses(data);
      } catch (e) {
        console.error("load courses failed", e);
      }
    })();
  }, [language, setCourses]);

  return (
    <main className="page">
      <section className="card">
        <div className="route-header">
          <div className="course-hero">
            <h2>课程中心</h2>
            <div className="course-route-pill">
              <Route size={14} /> {language === "CANTONESE" ? "粤语：生活交流 → 职场表达 → 进阶话题" : "英语：日常场景 → 职场交流 → 雅思口语"}
            </div>

            <div className="course-xp-panel">
              <div className="course-xp-headline">
                <strong>Lv.{learningProgress.stageIndex + 1}</strong>
                <small>总经验 {learningProgress.totalXP}</small>
              </div>
              <div className="course-progress-duo" aria-hidden>
                <div>
                  <span>当前等级进度</span>
                  <div className="mini-progress">
                    <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.currentPercent}%` }} transition={{ duration: 0.7, ease: "easeOut" }} />
                  </div>
                </div>
                <div>
                  <span>总经验里程</span>
                  <div className="mini-progress">
                    <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.totalProgressPercent}%` }} transition={{ duration: 0.9, ease: "easeOut" }} />
                  </div>
                </div>
              </div>
              <div className="course-xp-foot">
                <small>当前阶段：{learningProgress.currentPercent}%</small>
                <small>{learningProgress.stageIndex < stageModules[language].length ? `距下一阶段 ${learningProgress.nextLevelRemaining} XP` : "已达最高阶段"}</small>
              </div>
            </div>
          </div>
          <div className="route-tabs">
            <button className={language === "CANTONESE" ? "route-tab active" : "route-tab"} onClick={() => setLanguage("CANTONESE")}>粤语</button>
            <button className={language === "ENGLISH" ? "route-tab active" : "route-tab"} onClick={() => setLanguage("ENGLISH")}>英语</button>
          </div>
        </div>

        <div className="route-grid" style={{ marginBottom: 12 }}>
          {stageModules[language].map((stage, stageIndex) => {
            const unlocked = stageIndex <= learningProgress.stageIndex;
            const progress = stageIndex < learningProgress.stageIndex ? 100 : stageIndex === learningProgress.stageIndex ? learningProgress.currentPercent : 0;
            return (
              <article key={stage.stage} className="route-point" style={unlocked ? undefined : { opacity: 0.55 }}>
                <div className="row" style={{ justifyContent: "space-between", marginBottom: 4 }}>
                  <strong>{stage.stage}</strong>
                  <small>{unlocked ? `${progress}%` : "未解锁"}</small>
                </div>
                <div className="mini-progress" aria-hidden>
                  <span style={{ width: `${progress}%` }} />
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
                    onClick={() => navigate(`/generate?language=${language}&stage=${stageIndex}&topic=${encodeURIComponent(stage.modules[0])}`)}
                  >
                    进入本阶段学习
                  </button>
                </div>
              </article>
            );
          })}
        </div>

        <article className="stage-banner course-metrics-banner" style={{ marginBottom: 12 }}>
          <strong>学习指标说明</strong>
          <p style={{ margin: "6px 0 0" }}>学习指标用于阶段推荐，不替代总经验；总经验统一以个人中心为准。</p>
          <div className="course-metrics-grid">
            <div className="course-metric-card">
              <small>课程完成</small>
              <strong>{learningProgress.completedCourses}</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.courseCompletionPercent}%` }} transition={{ duration: 0.6, ease: "easeOut" }} />
              </div>
            </div>
            <div className="course-metric-card">
              <small>练习次数</small>
              <strong>{learningProgress.practiceCount}</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.practiceProgressPercent}%` }} transition={{ duration: 0.7, ease: "easeOut" }} />
              </div>
            </div>
            <div className="course-metric-card">
              <small>正确率</small>
              <strong>{(learningProgress.accuracy * 100).toFixed(0)}%</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${Math.round(learningProgress.accuracy * 100)}%` }} transition={{ duration: 0.8, ease: "easeOut" }} />
              </div>
            </div>
            <div className="course-metric-card emphasis">
              <small>学习指标</small>
              <strong>{learningProgress.learningIndex}</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.learningIndexPercent}%` }} transition={{ duration: 0.9, ease: "easeOut" }} />
              </div>
            </div>
          </div>
        </article>

        <div className="row">
          <button onClick={() => navigate("/generate")}>去生成剧场</button>
          <button className="btn-ghost" onClick={() => navigate("/reading")}>阅读训练</button>
          <button className="btn-ghost" onClick={() => navigate("/library")}>我的剧场库</button>
        </div>

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
