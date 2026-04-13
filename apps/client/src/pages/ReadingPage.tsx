import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookMarked, Filter, PlayCircle, Sparkles } from "lucide-react";
import { contentSources, generateReading, readingMaterials } from "../api";
import type { ContentSource } from "../types";

type ExamType = "IELTS" | "CET";
type SourceCategory =
  | "IELTS_OFFICIAL"
  | "IELTS_READING_LISTENING"
  | "IELTS_SPEAKING"
  | "CET_OFFICIAL"
  | "CET_READING_LISTENING"
  | "METHOD_REFERENCE";

const topicSeeds = {
  IELTS: ["Urban transportation and climate impact", "How AI changes classroom learning", "Balancing tourism and local culture"],
  CET: ["Campus recycling initiatives", "Digital habits of college students", "Community volunteer projects"]
};

export function ReadingPage() {
  const navigate = useNavigate();
  const [exam, setExam] = useState<ExamType>("IELTS");
  const [category, setCategory] = useState<SourceCategory | "ALL">("ALL");
  const [topic, setTopic] = useState(topicSeeds.IELTS[0]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [passage, setPassage] = useState("");
  const [questions, setQuestions] = useState<string[]>([]);
  const [sources, setSources] = useState<ContentSource[]>([]);
  const [vocabulary, setVocabulary] = useState<string[]>([]);
  const [materials, setMaterials] = useState<import("../types").ReadingMaterial[]>([]);

  useEffect(() => {
    void (async () => {
      try {
        const [sourceData, readingData] = await Promise.all([
          contentSources({ exam, category: category === "ALL" ? undefined : category }),
          readingMaterials(exam)
        ]);
        setSources(sourceData);
        setMaterials(readingData);
      } catch {
        setSources([]);
        setMaterials([]);
      }
    })();
  }, [exam, category]);

  const visibleSources = sources;

  async function handleGenerateReading() {
    setLoading(true);
    setError("");
    try {
      const generated = await generateReading({
        exam,
        topic,
        level: exam === "IELTS" ? "upper-intermediate" : "intermediate",
        sourceIds: visibleSources.slice(0, 5).map((s) => s.id)
      });

      setPassage(generated.passage);
      setQuestions((generated.questions ?? []).map((q) => q.question));
      setVocabulary(generated.vocabulary ?? []);
      const latest = await readingMaterials(exam);
      setMaterials(latest);
      navigate(`/reading/${generated.id}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="page">
      <section className="card">
        <div className="route-header">
          <div>
            <h2>阅读训练中心</h2>
            <p>按来源白名单生成阅读材料（1C/2B：导航入口 + 课程入口，直连 AI 生成链路）。</p>
          </div>
          <div className="row">
            <button className={exam === "IELTS" ? "route-tab active" : "route-tab"} onClick={() => { setExam("IELTS"); setTopic(topicSeeds.IELTS[0]); }}>
              IELTS
            </button>
            <button className={exam === "CET" ? "route-tab active" : "route-tab"} onClick={() => { setExam("CET"); setTopic(topicSeeds.CET[0]); }}>
              CET
            </button>
          </div>
        </div>

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
          <button onClick={handleGenerateReading} disabled={loading}>
            <Sparkles size={14} /> {loading ? "生成中..." : "生成阅读材料"}
          </button>
        </div>

        <div className="tag-row" style={{ marginTop: 10 }}>
          {topicSeeds[exam].map((seed) => (
            <button key={seed} type="button" className="tag-chip" onClick={() => setTopic(seed)}>{seed}</button>
          ))}
        </div>

        <div className="route-grid" style={{ marginTop: 12 }}>
          {visibleSources.map((item) => (
            <article key={`${item.domain}-${item.name}`} className="route-point">
              <strong>{item.name}</strong>
              <small>{item.domain}</small>
              <p style={{ margin: "6px 0 0" }}>{item.useCases.join(" / ")}</p>
              <small>模式：{item.contentMode}</small>
            </article>
          ))}
        </div>

        {error ? <p className="error">{error}</p> : null}

        {passage ? (
          <article className="stage-banner" style={{ marginTop: 8 }}>
            <strong>生成阅读材料</strong>
            <p style={{ whiteSpace: "pre-wrap" }}>{passage}</p>
            {vocabulary.length ? (
              <>
                <strong>重点词汇</strong>
                <p>{vocabulary.join(" / ")}</p>
              </>
            ) : null}
            {questions.length ? (
              <>
                <strong>理解题</strong>
                <ul>
                  {questions.map((q, i) => <li key={`${q}-${i}`}>{q}</li>)}
                </ul>
              </>
            ) : null}
          </article>
        ) : null}

        <article className="stage-banner" style={{ marginTop: 10 }}>
          <strong>阅读中心历史材料</strong>
          {materials.length === 0 ? <p>暂无历史阅读材料。</p> : null}
          <ul className="dialogue-list">
            {materials.map((item) => (
              <li key={item.id} className="dialogue" role="button" onClick={() => navigate(`/reading/${item.id}`)}>
                <div className="row" style={{ justifyContent: "space-between" }}>
                  <strong>{item.title}</strong>
                  <small>{item.audioStatus ?? "PENDING"}</small>
                </div>
                <p>{item.topic}</p>
                {item.audioStatus === "READY" && item.audioUrl ? (
                  <a className="link-button" href={item.audioUrl} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()}>
                    <PlayCircle size={14} /> 播放全文音频
                  </a>
                ) : (
                  <small>{item.audioStatus === "FAILED" ? "音频生成失败，可稍后重试生成材料。" : "音频后台生成中，完成后可播放。"}</small>
                )}
              </li>
            ))}
          </ul>
        </article>

        <div className="row">
          <button className="btn-ghost" onClick={() => navigate("/courses")}>返回课程中心</button>
          <button className="btn-ghost" onClick={() => navigate("/generate")}>去剧场生成</button>
        </div>
      </section>
    </main>
  );
}
