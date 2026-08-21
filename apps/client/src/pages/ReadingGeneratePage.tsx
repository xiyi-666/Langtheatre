import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { BookMarked, Filter, Sparkles } from "lucide-react";
import { contentSources, generateReading, readingMaterials } from "../api";
import { AICreditCostNotice } from "../components/AICreditCostNotice";
import { useAppStore } from "../store";
import { isMiniProgramEdition } from "../edition";
import type { ContentSource } from "../types";
import { calculateStageProgress, getStageRequirement } from "../xp";

type ExamType = "IELTS" | "CET";
type SourceCategory =
  | "IELTS_OFFICIAL"
  | "IELTS_READING_LISTENING"
  | "IELTS_SPEAKING"
  | "CET_OFFICIAL"
  | "CET_READING_LISTENING"
  | "METHOD_REFERENCE";

const readingStageTopics = {
  IELTS: [
    ["Urban transportation and climate impact", "How AI changes classroom learning", "Balancing tourism and local culture"],
    ["Work-life balance in modern cities", "Public health communication", "Online learning effectiveness"],
    ["Renewable energy adoption", "Cultural heritage protection", "Global migration trends"],
    ["Scientific literacy in media", "Data privacy and ethics", "Automation and labor market"]
  ],
  CET: [
    ["Campus recycling initiatives", "Digital habits of college students", "Community volunteer projects"],
    ["Time management for undergraduates", "Dormitory life and teamwork", "Sports and mental wellness"],
    ["Innovation contests on campus", "Internship preparation strategies", "Library usage behavior"],
    ["Entrepreneurship among students", "Career planning under uncertainty", "Cross-cultural communication"]
  ]
} as const;

export function ReadingGeneratePage() {
  const navigate = useNavigate();
  const user = useAppStore((s) => s.user);
  const { exam = "IELTS", stage = "0" } = useParams();
  const [searchParams] = useSearchParams();

  const safeExam: ExamType = exam === "CET" ? "CET" : "IELTS";
  const stageSeeds = readingStageTopics[safeExam];
  const parsedStage = Number.parseInt(stage, 10);
  const activeStage = Number.isFinite(parsedStage) && parsedStage >= 0
    ? Math.min(parsedStage, stageSeeds.length - 1)
    : 0;

  const [category, setCategory] = useState<SourceCategory | "ALL">("ALL");
  const [topic, setTopic] = useState(() => {
    const queryTopic = searchParams.get("topic")?.trim();
    return queryTopic || stageSeeds[activeStage][0];
  });
  const [loading, setLoading] = useState(false);
  const [generateError, setGenerateError] = useState("");
  const [sources, setSources] = useState<ContentSource[]>([]);
  const [selectedSourceIds, setSelectedSourceIds] = useState<string[]>([]);
  const [materials, setMaterials] = useState<import("../types").ReadingMaterial[]>([]);

  useEffect(() => {
    setTopic(searchParams.get("topic")?.trim() || stageSeeds[activeStage][0]);
  }, [activeStage, searchParams, stageSeeds]);

  useEffect(() => {
    void (async () => {
      try {
        const [sourceData, readingData] = await Promise.all([
          contentSources({ exam: safeExam, category: category === "ALL" ? undefined : category }),
          readingMaterials(safeExam)
        ]);
        setSources(sourceData);
        setMaterials(readingData);
      } catch {
        setSources([]);
        setMaterials([]);
      }
    })();
  }, [safeExam, category]);

  useEffect(() => {
    if (!materials.some((item) => item.status === "GENERATING")) return;
    const timer = window.setInterval(() => {
      void readingMaterials(safeExam).then(setMaterials).catch(() => undefined);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [materials, safeExam]);

  const visibleSources = sources;
  const currentSeeds = useMemo(() => stageSeeds[activeStage], [activeStage, stageSeeds]);
  const totalXP = user?.totalXP ?? 0;
  const stageProgress = useMemo(() => calculateStageProgress(totalXP, stageSeeds.length), [stageSeeds.length, totalXP]);
  const stageUnlocked = activeStage <= stageProgress.stageIndex;
  const requiredXP = getStageRequirement(activeStage, stageSeeds.length);

  useEffect(() => {
    if (visibleSources.length === 0) {
      setSelectedSourceIds([]);
      return;
    }
    setSelectedSourceIds((prev) => {
      const visibleSet = new Set(visibleSources.map((s) => s.id));
      const kept = prev.filter((id) => visibleSet.has(id));
      if (kept.length > 0) return kept;
      return visibleSources.slice(0, 5).map((s) => s.id);
    });
  }, [visibleSources]);

  async function handleGenerateReading() {
    if (!stageUnlocked) return;
    setGenerateError("");
    setLoading(true);
    try {
      const requestedBand = Number.parseFloat(searchParams.get("band") ?? "");
      const generated = await generateReading({
        exam: safeExam,
        topic,
        level: safeExam === "IELTS" ? "upper-intermediate" : "intermediate",
        sourceIds: selectedSourceIds.length > 0 ? selectedSourceIds : visibleSources.slice(0, 5).map((s) => s.id),
        band: Number.isFinite(requestedBand) && requestedBand > 0 ? requestedBand : undefined,
        stage: searchParams.get("stageName")?.trim() || `Stage ${activeStage + 1}`,
        section: searchParams.get("section")?.trim() || undefined,
        skillFocus: searchParams.get("skillFocus")?.trim() || undefined,
        questionType: searchParams.get("questionType")?.trim() || undefined,
        scenarioFamily: searchParams.get("scenarioFamily")?.trim() || undefined
      });
		setMaterials((current) => [generated, ...current.filter((item) => item.id !== generated.id)]);
      navigate(`/reading/${generated.id}/article`);
    } catch (e) {
      console.error("reading generate failed", e);
      setGenerateError(isMiniProgramEdition ? "阅读材料生成失败，线上 AI 服务暂时不可用，请稍后重试。" : "真实阅读材料生成失败，请检查模型配置、API Key 或稍后重试。");
    } finally {
      setLoading(false);
    }
  }

  function toggleSource(sourceId: string) {
    setSelectedSourceIds((prev) => {
      if (prev.includes(sourceId)) {
        return prev.filter((id) => id !== sourceId);
      }
      return [...prev, sourceId];
    });
  }

  return (
    <main className="page">
      <section className="card">
        <div className="route-header">
          <div>
            <h2>阅读材料生成台</h2>
          </div>
          <div className="row">
            <button className="btn-ghost" onClick={() => navigate("/reading")}>返回阅读中心</button>
            <button className="btn-ghost" onClick={() => navigate("/courses")}>课程中心</button>
          </div>
        </div>

        <article className="stage-banner" style={{ marginTop: 8 }}>
          <strong>当前考试与阶段</strong>
          <p>{safeExam} · Stage {activeStage + 1} · 当前已解锁至 Stage {stageProgress.stageIndex + 1}</p>
          <p>总经验：{totalXP}</p>
          {!stageUnlocked ? <p className="error">当前阶段尚未解锁，需要 {requiredXP} XP，当前仅有 {totalXP} XP。</p> : null}
        </article>

        <AICreditCostNotice action="READING_GENERATION" />

        <div className="row" style={{ marginTop: 8 }}>
          <label style={{ minWidth: 220 }}>
            <span><Filter size={14} /> 来源分类</span>
            <select value={category} onChange={(e) => setCategory(e.target.value as SourceCategory | "ALL")}>
              <option value="ALL">全部</option>
              <option value="IELTS_OFFICIAL">IELTS 官方</option>
              <option value="IELTS_READING_LISTENING">IELTS 阅读/听力题材</option>
              <option value="IELTS_SPEAKING">IELTS 口语题材</option>
              <option value="CET_OFFICIAL">CET 官方</option>
              <option value="CET_READING_LISTENING">CET 阅读/听力题材</option>
              <option value="METHOD_REFERENCE">方法参考</option>
            </select>
          </label>
          <label style={{ flex: 1, minWidth: 260 }}>
            <span><BookMarked size={14} /> 阅读主题</span>
            <input value={topic} onChange={(e) => setTopic(e.target.value)} />
          </label>
          <button onClick={handleGenerateReading} disabled={loading || !topic.trim() || !stageUnlocked}>
            <Sparkles size={14} /> {stageUnlocked ? (loading ? "生成中..." : "生成阅读材料") : `需 ${requiredXP} XP 解锁`}
          </button>
        </div>

        <div className="tag-row" style={{ marginTop: 10 }}>
          {currentSeeds.map((seed) => (
            <button key={seed} type="button" className="tag-chip" onClick={() => setTopic(seed)}>{seed}</button>
          ))}
        </div>
        {generateError ? <p className="error" style={{ marginTop: 10 }}>{generateError}</p> : null}

        <article className="stage-banner" style={{ marginTop: 10 }}>
          <strong>来源可交互学习</strong>
          <p>点卡片可勾选用于生成。已选 {selectedSourceIds.length} 个来源。</p>
        </article>

        <div className="route-grid" style={{ marginTop: 12 }}>
          {visibleSources.map((item) => (
            <article
              key={`${item.domain}-${item.name}`}
              className="route-point"
              role="button"
              tabIndex={0}
              onClick={() => toggleSource(item.id)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  toggleSource(item.id);
                }
              }}
              style={selectedSourceIds.includes(item.id) ? { borderColor: "#d4aa5b", boxShadow: "0 0 0 2px rgba(212,170,91,0.25)" } : undefined}
            >
              <strong>{item.name}</strong>
              <small>{item.domain}</small>
              <p style={{ margin: "6px 0 0" }}>{item.useCases.join(" / ")}</p>
              <small>模式：{item.contentMode}</small>
              <div className="row" style={{ marginTop: 8 }}>
                <small>{selectedSourceIds.includes(item.id) ? "已选中用于生成" : "点击卡片选中来源"}</small>
              </div>
            </article>
          ))}
        </div>

        <article className="stage-banner" style={{ marginTop: 10 }}>
          <strong>同考试历史材料</strong>
          {materials.length === 0 ? <p>暂无历史阅读材料。</p> : null}
          <ul className="dialogue-list">
            {materials.map((item) => (
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
          </ul>
        </article>
      </section>
    </main>
  );
}
