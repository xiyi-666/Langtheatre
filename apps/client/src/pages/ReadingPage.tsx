import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookOpenText, ScrollText } from "lucide-react";
import { readingMaterials } from "../api";
import { useAppStore } from "../store";

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
  const user = useAppStore((s) => s.user);
  const [exam, setExam] = useState<ExamType>("IELTS");
  const [materials, setMaterials] = useState<import("../types").ReadingMaterial[]>([]);

  useEffect(() => {
    void (async () => {
      try {
        const data = await readingMaterials(exam);
        setMaterials(data);
      } catch {
        setMaterials([]);
      }
    })();
  }, [exam]);

  const stages = useMemo(() => readingStages[exam], [exam]);

  return (
    <main className="page">
      <section className="card stage-shell">
        <div className="route-header">
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
          <strong>经验口径说明</strong>
          <p>阅读训练提交后会写入统一 XP 体系，个人中心总经验为唯一口径。</p>
          <p>当前总经验：{user?.totalXP ?? 0}</p>
        </article>

        <div className="route-grid" style={{ marginTop: 12 }}>
          {stages.map((stage, index) => (
            <article key={stage.stage} className="route-point">
              <strong>{stage.stage}</strong>
              <small>{exam} 阅读训练</small>
              <p style={{ marginTop: 8 }}>{stage.themes.join(" / ")}</p>
              <button
                type="button"
                className="btn-ghost"
                onClick={() => navigate(`/reading/generate/${exam}/${index}?topic=${encodeURIComponent(stage.themes[0])}`)}
              >
                <BookOpenText size={14} /> 进入本阶段并生成
              </button>
            </article>
          ))}
        </div>

        <article className="stage-banner" style={{ marginTop: 10 }}>
          <strong><ScrollText size={14} /> 历史阅读材料</strong>
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
