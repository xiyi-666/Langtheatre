import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { BookOpenText, ScrollText, Search } from "lucide-react";
import { readingMaterials } from "../api";
import { useAppStore } from "../store";
import { calculateStageProgress, getStageRequirement } from "../xp";

type ExamType = "IELTS" | "CET";

const readingStages = {
  IELTS: [
    { stage: "Stage 01", themes: ["城市与环境", "教育与学习", "旅游与文化"] },
    { stage: "Stage 02", themes: ["职场与生活平衡", "公共服务", "在线学习"] },
    { stage: "Stage 03", themes: ["科技与能源", "文化遗产", "社会趋势"] },
    { stage: "Stage 04", themes: ["媒体与科学", "数据与伦理", "自动化与就业"] }
  ],
  CET: [
    { stage: "Stage 01", themes: ["校园生活", "数字习惯", "志愿服务"] },
    { stage: "Stage 02", themes: ["时间管理", "团队协作", "身心健康"] },
    { stage: "Stage 03", themes: ["竞赛与创新", "实习准备", "学习效率"] },
    { stage: "Stage 04", themes: ["职业规划", "沟通表达", "综合写读"] }
  ]
} as const;

export function ReadingPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const isLibraryPage = location.pathname === "/reading/library";
  const user = useAppStore((s) => s.user);
  const [exam, setExam] = useState<ExamType>("IELTS");
  const [materials, setMaterials] = useState<import("../types").ReadingMaterial[]>([]);
  const [loadingMaterials, setLoadingMaterials] = useState(true);
  const [materialsError, setMaterialsError] = useState("");
  const [historyMessage, setHistoryMessage] = useState("");
  const [historyQuery, setHistoryQuery] = useState("");
  const [historyStatus, setHistoryStatus] = useState("ALL");
  const [historyPage, setHistoryPage] = useState(1);
  const historyTitleRef = useRef<HTMLHeadingElement | null>(null);

  const loadMaterials = useCallback(async (showLoading = true) => {
    if (showLoading) {
      setLoadingMaterials(true);
      setMaterialsError("");
    }
    try {
      setMaterials(await readingMaterials(exam));
    } catch (error) {
      if (showLoading) {
        setMaterials([]);
        setMaterialsError((error as Error).message || "阅读材料加载失败，请稍后重试。");
      }
    } finally {
      if (showLoading) setLoadingMaterials(false);
    }
  }, [exam]);

  useEffect(() => {
    if (isLibraryPage) void loadMaterials();
  }, [isLibraryPage, loadMaterials]);

  useEffect(() => {
    const state = location.state as { message?: string } | null;
    if (!isLibraryPage || !state?.message) return;
    setHistoryMessage(state.message);
    void loadMaterials();
    navigate(location.pathname, { replace: true, state: null });
  }, [isLibraryPage, loadMaterials, location.pathname, location.state, navigate]);

  useEffect(() => {
    if (historyMessage && !loadingMaterials) historyTitleRef.current?.focus();
  }, [historyMessage, loadingMaterials]);

  useEffect(() => {
    if (!materials.some((item) => item.status === "GENERATING")) return;
    const timer = window.setInterval(() => {
      void loadMaterials(false);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [loadMaterials, materials]);

  const stages = useMemo(() => readingStages[exam], [exam]);
  const totalXP = user?.totalXP ?? 0;
  const stageProgress = useMemo(() => calculateStageProgress(totalXP, stages.length), [stages.length, totalXP]);
  const filteredMaterials = useMemo(() => {
    const query = historyQuery.trim().toLowerCase();
    return materials.filter((item) => {
      const status = item.status ?? "READY";
      const matchesStatus = historyStatus === "ALL" || status === historyStatus;
      const matchesQuery = !query || [item.title, item.topic, item.level, item.language].some((value) => value.toLowerCase().includes(query));
      return matchesStatus && matchesQuery;
    });
  }, [historyQuery, historyStatus, materials]);
  const historyPageCount = Math.max(1, Math.ceil(filteredMaterials.length / 8));
  const visibleMaterials = filteredMaterials.slice((historyPage - 1) * 8, historyPage * 8);

  useEffect(() => {
    setHistoryPage(1);
  }, [exam, historyQuery, historyStatus]);

  return (
    <main className="page">
      <section className="card stage-shell">
        {!isLibraryPage ? <><div className="route-header">
          <div>
            <h2>阅读训练中心</h2>
            <p>先选择阶段，再进入子页生成阅读材料</p>
          </div>
          <div className="row">
            <button className={exam === "IELTS" ? "route-tab active" : "route-tab"} onClick={() => setExam("IELTS")}>
              IELTS
            </button>
            <button className={exam === "CET" ? "route-tab active" : "route-tab"} onClick={() => setExam("CET")}>
              CET
            </button>
          </div>
        </div>

        <article className="stage-banner" style={{ marginTop: 10 }}>
          <strong>统一 XP 说明</strong>
          <p>阅读训练获得的 XP 会直接累加到个人中心总 XP，阅读阶段也按同一套 XP 规则解锁。</p>
          <p>当前总经验：{totalXP} · 当前解锁至 Stage {stageProgress.stageIndex + 1}</p>
        </article>

        <div className="route-grid" style={{ marginTop: 12 }}>
          {stages.map((stage, index) => (
            <article key={stage.stage} className="route-point" style={index <= stageProgress.stageIndex ? undefined : { opacity: 0.55 }}>
              <strong>{stage.stage}</strong>
              <small>{exam} 阅读训练 · {index <= stageProgress.stageIndex ? "已解锁" : `需 ${getStageRequirement(index, stages.length)} XP`}</small>
              <p style={{ marginTop: 8 }}>{stage.themes.join(" / ")}</p>
              <button
                type="button"
                className="btn-ghost"
                disabled={index > stageProgress.stageIndex}
                onClick={() => navigate(`/reading/generate/${exam}/${index}?topic=${encodeURIComponent(stage.themes[0])}`)}
              >
                <BookOpenText size={14} /> 进入本阶段并生成
              </button>
            </article>
          ))}
        </div>
        <article className="stage-banner" style={{ marginTop: 12 }}><h3><ScrollText size={14} /> 阅读材料库</h3><p>在独立子页中搜索、筛选和分页查看你生成的阅读材料。</p><button type="button" className="btn-ghost" onClick={() => navigate("/reading/library")}>查看我的阅读材料</button></article>
        </> : null}

        {isLibraryPage ? <article className="stage-banner" style={{ marginTop: 10 }} aria-labelledby="reading-history-title">
          <div className="route-header"><div><h2 ref={historyTitleRef} id="reading-history-title" tabIndex={-1}><ScrollText size={16} /> 阅读材料库</h2><p>搜索、筛选并按页查看已生成的材料。</p></div><button type="button" className="btn-ghost" onClick={() => navigate("/reading")}>返回阅读训练</button></div>
           <div className="library-toolbar" aria-label="筛选阅读材料">
             <label className="library-search"><Search size={16} /><span className="sr-only">搜索阅读材料</span><input value={historyQuery} onChange={(event) => setHistoryQuery(event.target.value)} placeholder="搜索标题、主题或难度" /></label>
             <label>状态<select value={historyStatus} onChange={(event) => setHistoryStatus(event.target.value)}><option value="ALL">全部状态</option><option value="READY">已完成</option><option value="GENERATING">生成中</option><option value="FAILED">生成失败</option></select></label>
           </div>
           {historyMessage ? <p role="status" aria-live="polite">{historyMessage}</p> : null}
          {loadingMaterials ? <p className="muted-note">正在加载阅读材料…</p> : null}
          {materialsError ? <p className="field-error" role="alert">{materialsError}</p> : null}
          {materialsError ? <button type="button" className="btn-ghost" onClick={() => void loadMaterials()}>重试加载</button> : null}
           {!loadingMaterials && !materialsError && materials.length === 0 ? <p>暂无历史阅读材料。</p> : null}
           {!loadingMaterials && !materialsError && materials.length > 0 ? <p className="library-result-count">找到 {filteredMaterials.length} 份材料，第 {historyPage} / {historyPageCount} 页。</p> : null}
           {!loadingMaterials && !materialsError && materials.length > 0 && filteredMaterials.length === 0 ? <p className="muted-note">没有符合当前搜索或筛选条件的阅读材料。</p> : null}
           {!loadingMaterials && !materialsError ? <ul key={`${historyPage}-${historyQuery}-${historyStatus}`} className="dialogue-list library-page-list">
             {visibleMaterials.map((item) => (
              <li key={item.id} className="dialogue">
                <div className="row" style={{ justifyContent: "space-between" }}>
                  <strong>{item.title}</strong>
                  <small>{item.status === "GENERATING" ? "生成中" : item.status === "FAILED" ? "生成失败" : item.audioStatus ?? "PENDING"}</small>
                </div>
                <p>{item.topic}</p>
                {item.status === "GENERATING" ? <div className="task-progress" aria-live="polite"><div className="task-progress-head"><span>{item.generationMessage || "正在生成阅读材料"}</span><strong>{item.generationProgress ?? 0}%</strong></div><div className="progress-bar"><div className="progress-value" style={{ width: `${Math.max(4, item.generationProgress ?? 0)}%` }} /></div></div> : null}
                <div className="dialogue-actions">
                  <button type="button" className="btn-ghost" onClick={() => navigate(`/reading/${item.id}/article`)}>
                    {item.status === "GENERATING" ? "查看生成进度" : item.status === "FAILED" ? "查看失败原因" : "查看材料"}
                  </button>
                </div>
              </li>
             ))}
           </ul> : null}
           {!loadingMaterials && !materialsError && filteredMaterials.length > 8 ? <div className="library-pagination"><button type="button" className="btn-ghost" disabled={historyPage <= 1} onClick={() => setHistoryPage((page) => page - 1)}>上一页</button><span>第 {historyPage} / {historyPageCount} 页</span><button type="button" className="btn-ghost" disabled={historyPage >= historyPageCount} onClick={() => setHistoryPage((page) => page + 1)}>下一页</button></div> : null}
         </article> : null}
      </section>
    </main>
  );
}
