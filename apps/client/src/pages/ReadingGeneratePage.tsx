import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { BookMarked, Filter, Sparkles } from "lucide-react";
import { contentSources, generateReading, readingMaterials } from "../api";
import { useAppStore } from "../store";
import type { ContentSource } from "../types";

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

  const visibleSources = sources;
  const currentSeeds = useMemo(() => stageSeeds[activeStage], [activeStage, stageSeeds]);

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
    setLoading(true);
    try {
      const generated = await generateReading({
        exam: safeExam,
        topic,
        level: safeExam === "IELTS" ? "upper-intermediate" : "intermediate",
        sourceIds: selectedSourceIds.length > 0 ? selectedSourceIds : visibleSources.slice(0, 5).map((s) => s.id)
      });
      const latest = await readingMaterials(safeExam);
      setMaterials(latest);
      navigate(`/reading/${generated.id}/article`);
    } catch (e) {
      console.error("reading generate failed", e);
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
          <p>{safeExam} · Stage {activeStage + 1}（阅读中心无经验门槛，所有阶段可直接进入）</p>
          <p>总经验以个人中心为准：{user?.totalXP ?? 0}</p>
        </article>

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
          <button onClick={handleGenerateReading} disabled={loading || !topic.trim()}>
            <Sparkles size={14} /> {loading ? "生成中..." : "生成阅读材料"}
          </button>
        </div>

        <div className="tag-row" style={{ marginTop: 10 }}>
          {currentSeeds.map((seed) => (
            <button key={seed} type="button" className="tag-chip" onClick={() => setTopic(seed)}>{seed}</button>
          ))}
        </div>

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
              <li key={item.id} className="dialogue" role="button" onClick={() => navigate(`/reading/${item.id}/article`)}>
                <div className="row" style={{ justifyContent: "space-between" }}>
                  <strong>{item.title}</strong>
                  <small>{item.audioStatus ?? "PENDING"}</small>
                </div>
                <p>{item.topic}</p>
              </li>
            ))}
          </ul>
        </article>
      </section>
    </main>
  );
}
