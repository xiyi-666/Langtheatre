import { TouchEvent, useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Check, Copy, Heart, Share2, Theater, Trash2, TrendingUp } from "lucide-react";
import { deleteTheater, myTheaters, shareTheater, toggleFavorite } from "../api";
import { useAppStore } from "../store";

export function LibraryPage() {
  const [languageFilter, setLanguageFilter] = useState<"ALL" | "CANTONESE" | "ENGLISH">("ALL");
  const [statusFilter, setStatusFilter] = useState<"ALL" | "READY" | "GENERATING" | "FAILED">("ALL");
  const [difficultyFilter, setDifficultyFilter] = useState<"ALL" | "4-5.5" | "6-7" | "7.5+">("ALL");
  const [favoriteFilter, setFavoriteFilter] = useState<"ALL" | "ONLY">("ALL");
  const theaters = useAppStore((s) => s.theaters);
  const setTheaters = useAppStore((s) => s.setTheaters);
  const [sharingID, setSharingID] = useState("");
  const [shareHint, setShareHint] = useState("");
  const [pullDistance, setPullDistance] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [refreshHint, setRefreshHint] = useState("");
  const startYRef = useRef<number | null>(null);
  const navigate = useNavigate();

  const reload = useCallback(async () => {
    try {
      const data = await myTheaters();
      setTheaters(data);
    } catch (e) {
      console.error("load theaters failed", e);
    }
  }, [setTheaters]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const filteredTheaters = theaters.filter((item) => {
    if (languageFilter !== "ALL" && item.language !== languageFilter) return false;
    if (statusFilter !== "ALL" && item.status !== statusFilter) return false;
    if (favoriteFilter === "ONLY" && !item.isFavorite) return false;
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

  async function ensureShareLink(item: (typeof theaters)[number]): Promise<{ code: string; url: string }> {
    const code = item.shareCode && item.shareCode.trim() !== "" ? item.shareCode : await shareTheater(item.id);
    const url = `${window.location.origin}/theater/shared/${encodeURIComponent(code)}`;
    return { code, url };
  }

  function flashShareHint(text: string) {
    setShareHint(text);
    window.setTimeout(() => setShareHint(""), 1600);
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
          <label>
            收藏
            <select value={favoriteFilter} onChange={(e) => setFavoriteFilter(e.target.value as typeof favoriteFilter)}>
              <option value="ALL">全部</option>
              <option value="ONLY">仅收藏</option>
            </select>
          </label>
        </div>
        {shareHint ? <p className="share-hint">{shareHint}</p> : null}

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
        <ul className="dialogue-list">
          {filteredTheaters.length === 0 ? (
            <li className="theater-item">
              <p style={{ margin: 0 }}>当前筛选条件下暂无剧场，可切换筛选或先生成新剧场。</p>
            </li>
          ) : null}
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
              {item.shareCode ? <p className="share-code-line">分享码：{item.shareCode}</p> : null}
              <p><TrendingUp size={14} /> {item.language === "CANTONESE" ? "推荐路径：日常交流 -> 职场 -> 专业" : "Recommended flow: daily -> workplace -> IELTS"}</p>
              <div className="row">
                <button onClick={() => navigate(`/theater/${item.id}`)}>{getPracticeLabel(item.mode)}</button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    try {
                      await toggleFavorite(item.id, !item.isFavorite);
                      await reload();
                    } catch (e) {
                      console.error("toggle favorite failed", e);
                    }
                  }}
                >
                  <Heart size={16} className={item.isFavorite ? "star-active" : ""} /> {item.isFavorite ? "取消收藏" : "收藏"}
                </button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    try {
                      setSharingID(item.id);
                      const payload = await ensureShareLink(item);
                      await reload();
                      if (navigator.share) {
                        await navigator.share({
                          title: `LinguaQuest 剧场：${item.topic}`,
                          text: `分享剧场「${item.topic}」，可通过分享码 ${payload.code} 打开。`,
                          url: payload.url
                        });
                        flashShareHint("已调用系统分享面板");
                      } else if (navigator.clipboard?.writeText) {
                        await navigator.clipboard.writeText(payload.url);
                        flashShareHint(`分享链接已复制（${payload.code}）`);
                      } else {
                        window.prompt("请复制分享链接", payload.url);
                        flashShareHint(`已生成分享链接（${payload.code}）`);
                      }
                    } catch (e) {
                      console.error("share theater failed", e);
                      flashShareHint("分享失败，请稍后重试");
                    } finally {
                      setSharingID("");
                    }
                  }}
                >
                  <Share2 size={16} /> {sharingID === item.id ? "处理中..." : "分享"}
                </button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    try {
                      const payload = await ensureShareLink(item);
                      await reload();
                      if (navigator.clipboard?.writeText) {
                        await navigator.clipboard.writeText(payload.url);
                        flashShareHint(`链接已复制（${payload.code}）`);
                      } else {
                        window.prompt("请复制分享链接", payload.url);
                        flashShareHint(`已生成可复制链接（${payload.code}）`);
                      }
                    } catch (e) {
                      console.error("copy share link failed", e);
                      flashShareHint("复制失败，请稍后重试");
                    }
                  }}
                >
                  {item.shareCode ? <Check size={16} /> : <Copy size={16} />} 复制链接
                </button>
                <button
                  className="btn-ghost"
                  onClick={async () => {
                    const confirmed = window.confirm(`确认删除剧场「${item.topic}」吗？删除后无法恢复。`);
                    if (!confirmed) return;
                    try {
                      await deleteTheater(item.id);
                      await reload();
                    } catch (e) {
                      console.error("delete theater failed", e);
                    }
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
