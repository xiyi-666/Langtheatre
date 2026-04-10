import { TouchEvent, useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Heart, Share2, Theater, Trash2, TrendingUp } from "lucide-react";
import { deleteTheater, myTheaters, shareTheater, toggleFavorite } from "../api";
import { useAppStore } from "../store";

export function LibraryPage() {
  const [error, setError] = useState("");
  const [languageFilter, setLanguageFilter] = useState<"ALL" | "CANTONESE" | "ENGLISH">("ALL");
  const [statusFilter, setStatusFilter] = useState<"ALL" | "READY" | "GENERATING" | "FAILED">("ALL");
  const [difficultyFilter, setDifficultyFilter] = useState<"ALL" | "4-5.5" | "6-7" | "7.5+">("ALL");
  const theaters = useAppStore((s) => s.theaters);
  const setTheaters = useAppStore((s) => s.setTheaters);
  const [pullDistance, setPullDistance] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [refreshHint, setRefreshHint] = useState("");
  const startYRef = useRef<number | null>(null);
  const navigate = useNavigate();

  const reload = useCallback(async () => {
    setError("");
    try {
      const data = await myTheaters();
      setTheaters(data);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [setTheaters]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const filteredTheaters = theaters.filter((item) => {
    if (languageFilter !== "ALL" && item.language !== languageFilter) return false;
    if (statusFilter !== "ALL" && item.status !== statusFilter) return false;
    if (difficultyFilter === "4-5.5" && (item.difficulty < 4 || item.difficulty > 5.5)) return false;
    if (difficultyFilter === "6-7" && (item.difficulty < 6 || item.difficulty > 7)) return false;
    if (difficultyFilter === "7.5+" && item.difficulty < 7.5) return false;
    return true;
  });

  const routeStats = {
    cant: theaters.filter((item) => item.language === "CANTONESE").length,
    eng: theaters.filter((item) => item.language === "ENGLISH").length,
    ready: theaters.filter((item) => item.status === "READY").length
  };

  function getPracticeLabel(mode: "LISTENING" | "ROLEPLAY" | "APPRECIATION") {
    if (mode === "APPRECIATION") return "进入欣赏";
    if (mode === "ROLEPLAY") return "进入剧场";
    return "继续练习";
  }

  function onTouchStart(event: TouchEvent<HTMLElement>) {
    if (window.scrollY > 0 || isRefreshing) return;
    startYRef.current = event.changedTouches[0]?.clientY ?? null;
  }

  function onTouchMove(event: TouchEvent<HTMLElement>) {
    const start = startYRef.current;
    if (start === null || isRefreshing) return;
    const currentY = event.changedTouches[0]?.clientY ?? start;
    const delta = Math.max(0, Math.min(120, currentY - start));
    setPullDistance(delta);
    setRefreshHint(delta > 70 ? "松开刷新" : "下拉刷新");
  }

  async function onTouchEnd() {
    if (isRefreshing) return;
    if (pullDistance > 70) {
      setIsRefreshing(true);
      setRefreshHint("刷新中...");
      await reload();
      setRefreshHint("刷新完成");
      window.setTimeout(() => setRefreshHint(""), 700);
      setIsRefreshing(false);
    }
    setPullDistance(0);
    startYRef.current = null;
  }

  return (
    <main className="page" onTouchStart={onTouchStart} onTouchMove={onTouchMove} onTouchEnd={onTouchEnd}>
      <section className="card stage-shell">
        <div className="pull-refresh" aria-live="polite" style={{ height: `${pullDistance}px` }}>
          <small>{refreshHint}</small>
        </div>
        <header className="route-header">
          <div>
            <h2>我的剧场库</h2>
            <p>按语种、难度、完成状态快速筛选并继续练习</p>
          </div>
          <button onClick={() => navigate("/generate")}>生成新剧场</button>
        </header>

        <div className="library-filters">
          <label>
            语种
            <select value={languageFilter} onChange={(e) => setLanguageFilter(e.target.value as typeof languageFilter)}>
              <option value="ALL">全部</option>
              <option value="CANTONESE">粤语</option>
              <option value="ENGLISH">英语</option>
            </select>
          </label>
          <label>
            难度
            <select value={difficultyFilter} onChange={(e) => setDifficultyFilter(e.target.value as typeof difficultyFilter)}>
              <option value="ALL">全部</option>
              <option value="4-5.5">4.0-5.5</option>
              <option value="6-7">6.0-7.0</option>
              <option value="7.5+">7.5+</option>
            </select>
          </label>
          <label>
            状态
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)}>
              <option value="ALL">全部</option>
              <option value="READY">已完成</option>
              <option value="GENERATING">生成中</option>
              <option value="FAILED">失败</option>
            </select>
          </label>
        </div>

        <div className="metric-grid" style={{ marginTop: 10 }}>
          <article className="metric-card">
            <strong>粤语</strong>
            <p>{routeStats.cant} 个剧场</p>
          </article>
          <article className="metric-card">
            <strong>英语</strong>
            <p>{routeStats.eng} 个剧场</p>
          </article>
          <article className="metric-card">
            <strong>已完成</strong>
            <p>{routeStats.ready} 条</p>
          </article>
        </div>

        <div className="row">
          <button onClick={() => navigate("/courses")}>课程中心</button>
          <button className="btn-ghost" onClick={() => navigate("/profile")}>个人中心</button>
        </div>
        {error ? <p className="error">{error}</p> : null}
        <ul className="dialogue-list">
          {filteredTheaters.map((item) => (
            <motion.li
              key={item.id}
              className="theater-item"
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
            >
              <div className="row" style={{ justifyContent: "space-between" }}>
                <strong>
                  <Theater size={16} /> {item.topic}
                </strong>
                <span className={item.status === "READY" ? "status-pill done" : "status-pill todo"}>
                  {item.status === "READY" ? "已完成" : "待完成"}
                </span>
              </div>
              <p>
                {item.language === "CANTONESE" ? "粤语" : "英语"} | {item.mode} | 难度 {item.difficulty}
              </p>
              <p><TrendingUp size={14} /> {item.language === "CANTONESE" ? "推荐路径：日常交流 -> 职场 -> 雅思" : "Recommended flow: daily -> workplace -> IELTS"}</p>
              <div className="row">
                <button onClick={() => navigate(`/theater/${item.id}`)}>{getPracticeLabel(item.mode)}</button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    await toggleFavorite(item.id, !item.isFavorite);
                    await reload();
                  }}
                >
                  <Heart size={16} className={item.isFavorite ? "star-active" : ""} /> {item.isFavorite ? "取消收藏" : "收藏"}
                </button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    const code = await shareTheater(item.id);
                    window.alert(`分享码: ${code}`);
                  }}
                >
                  <Share2 size={16} /> 分享
                </button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    const confirmed = window.confirm(`确认删除剧场「${item.topic}」吗？删除后无法恢复。`);
                    if (!confirmed) return;
                    await deleteTheater(item.id);
                    await reload();
                  }}
                >
                  <Trash2 size={16} /> 删除
                </button>
                <button onClick={() => navigate(`/roleplay/${item.id}`)}>角色扮演</button>
              </div>
            </motion.li>
          ))}
        </ul>
      </section>
    </main>
  );
}
