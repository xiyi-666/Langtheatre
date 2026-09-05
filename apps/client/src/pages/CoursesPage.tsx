import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Compass, Route, Scale, Sparkles } from "lucide-react";
import { courses } from "../api";
import { useAppStore } from "../store";
import { calculateLearningProgress, calculateStageProgress } from "../xp";
import { AdSlot } from "../components/AdSlot";
import { isDemoUser } from "../demoExperience";

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

export function CoursesPage() {
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
  const list = useAppStore((s) => s.courses);
  const theaters = useAppStore((s) => s.theaters);
  const user = useAppStore((s) => s.user);
  const latestResult = useAppStore((s) => s.result);
  const roleplay = useAppStore((s) => s.roleplay);
  const setCourses = useAppStore((s) => s.setCourses);
  const navigate = useNavigate();
  const totalXP = user?.totalXP ?? 0;
  const demoUser = isDemoUser(user);

  const stageProgress = useMemo(
    () => calculateStageProgress(totalXP, stageModules[language].length),
    [language, totalXP]
  );
  const learningProgress = useMemo(
    () => calculateLearningProgress(totalXP),
    [totalXP]
  );

  const learningStats = useMemo(() => {
    const completedCourses = list.filter((item) => item.isActive).length;
    const practiceCount = theaters.filter((item) => item.language === language).length + (roleplay ? roleplay.turnIndex + 1 : 0);
    const accuracy = latestResult && latestResult.totalCount > 0
      ? latestResult.correctCount / latestResult.totalCount
      : Math.min(1, Math.max(0, (roleplay?.currentScore ?? 0) / 100));

    const courseCompletionPercent = list.length > 0 ? Math.min(100, Math.round((completedCourses / list.length) * 100)) : 0;
    const practiceProgressPercent = Math.min(100, practiceCount * 10);

    return {
      completedCourses,
      practiceCount,
      accuracy,
      courseCompletionPercent,
      practiceProgressPercent
    };
  }, [language, list, theaters, roleplay, latestResult]);

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
                <strong>Lv.{learningProgress.level}</strong>
                <small>{learningProgress.rankLabel} · 总经验 {stageProgress.totalXP}</small>
              </div>
              <div className="course-progress-duo" aria-hidden>
                <div>
                  <span>当前等级进度</span>
                  <div className="mini-progress">
                    <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.levelProgress}%` }} transition={{ duration: 0.7, ease: "easeOut" }} />
                  </div>
                </div>
                <div>
                  <span>总经验里程</span>
                  <div className="mini-progress">
                    <motion.span initial={{ width: 0 }} animate={{ width: `${stageProgress.totalProgressPercent}%` }} transition={{ duration: 0.9, ease: "easeOut" }} />
                  </div>
                </div>
              </div>
              <div className="course-xp-foot">
                <small>Lv.{learningProgress.level}：{learningProgress.levelProgress}%</small>
                <small>{learningProgress.level < 999 ? `距下一等级 ${Math.max(0, learningProgress.xpToNextLevel - learningProgress.xpIntoLevel)} XP` : "已达最高等级"}</small>
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
            const unlocked = demoUser || stageIndex <= stageProgress.stageIndex;
            const progress = demoUser ? 100 : stageIndex < stageProgress.stageIndex ? 100 : stageIndex === stageProgress.stageIndex ? stageProgress.currentPercent : 0;
            return (
              <article key={stage.stage} className="route-point" style={unlocked ? undefined : { opacity: 0.55 }}>
                <div className="row" style={{ justifyContent: "space-between", marginBottom: 4 }}>
                  <strong>{stage.stage}</strong>
                  <small>{demoUser ? "演示可体验" : unlocked ? `${progress}%` : "未解锁"}</small>
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
                      onClick={() => navigate(`/generate?language=${language}&stage=${stageIndex}&topic=${encodeURIComponent(module)}`)}
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
          <strong>{demoUser ? "演示模式 · 学习权限已开放" : "统一 XP 说明"}</strong>
          <p style={{ margin: "6px 0 0" }}>课程阶段解锁与个人中心总 XP 共用同一套规则。下方统计仅用于学习反馈，不会额外生成第二套经验。</p>
          {demoUser ? <p>所有课程阶段均可直接体验预置内容，不调用 AI，不消耗点数。</p> : null}
          <div className="course-metrics-grid">
            <div className="course-metric-card">
              <small>课程完成</small>
              <strong>{learningStats.completedCourses}</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${learningStats.courseCompletionPercent}%` }} transition={{ duration: 0.6, ease: "easeOut" }} />
              </div>
            </div>
            <div className="course-metric-card">
              <small>练习次数</small>
              <strong>{learningStats.practiceCount}</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${learningStats.practiceProgressPercent}%` }} transition={{ duration: 0.7, ease: "easeOut" }} />
              </div>
            </div>
            <div className="course-metric-card">
              <small>正确率</small>
              <strong>{(learningStats.accuracy * 100).toFixed(0)}%</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${Math.round(learningStats.accuracy * 100)}%` }} transition={{ duration: 0.8, ease: "easeOut" }} />
              </div>
            </div>
            <div className="course-metric-card emphasis">
              <small>成长等级</small>
              <strong>Lv.{learningProgress.level}</strong>
              <div className="mini-progress" aria-hidden>
                <motion.span initial={{ width: 0 }} animate={{ width: `${learningProgress.levelProgress}%` }} transition={{ duration: 0.9, ease: "easeOut" }} />
              </div>
            </div>
          </div>
        </article>

        <AdSlot placement="COURSES" />

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
