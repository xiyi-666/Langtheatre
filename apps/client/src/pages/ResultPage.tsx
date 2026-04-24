import { TouchEvent, useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";
import { Lightbulb, RotateCcw, Share2 } from "lucide-react";
import { Link } from "react-router-dom";
import { useAppStore } from "../store";

export function ResultPage() {
  const result = useAppStore((s) => s.result);
  const user = useAppStore((s) => s.user);
  const score = result?.score ?? 0;
  const xp = result?.xpEarned ?? 0;
  const correct = result?.correctCount ?? 0;
  const total = result?.totalCount ?? 0;
  const [animatedXp, setAnimatedXp] = useState(0);
  const [showMore, setShowMore] = useState(false);
  const swipeStartRef = useRef<number | null>(null);
  const ratio = Math.max(0, Math.min(100, score));
  const r = 74;
  const c = 2 * Math.PI * r;
  const dash = (ratio / 100) * c;

  useEffect(() => {
    if (xp <= 0) {
      setAnimatedXp(0);
      return;
    }
    const step = Math.max(1, Math.ceil(xp / 24));
    const timer = window.setInterval(() => {
      setAnimatedXp((value) => {
        const next = value + step;
        if (next >= xp) {
          window.clearInterval(timer);
          return xp;
        }
        return next;
      });
    }, 35);
    return () => {
      window.clearInterval(timer);
    };
  }, [xp]);

  function onDetailTouchStart(event: TouchEvent<HTMLElement>) {
    swipeStartRef.current = event.changedTouches[0]?.clientY ?? null;
  }

  function onDetailTouchEnd(event: TouchEvent<HTMLElement>) {
    const start = swipeStartRef.current;
    swipeStartRef.current = null;
    if (start === null) return;
    const end = event.changedTouches[0]?.clientY ?? start;
    const delta = end - start;
    if (delta < -50) {
      setShowMore(true);
    }
    if (delta > 50) {
      setShowMore(false);
    }
  }

  return (
    <main className="page-center">
      <section className="card stage-shell">
        <h2>恭喜完成本轮练习</h2>
        <p>学习者：{user?.email ?? "未登录"}</p>
        <p>当前总经验（统一口径）：{user?.totalXP ?? 0}</p>
        <div className="result-shell">
          <motion.div className="floating-panel" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }}>
            <svg viewBox="0 0 180 180" className="score-ring" role="img" aria-label="score ring">
              <circle cx="90" cy="90" r={r} stroke="#f6dcc5" strokeWidth="14" fill="none" />
              <motion.circle
                cx="90"
                cy="90"
                r={r}
                stroke="url(#scoreGradient)"
                strokeWidth="14"
                strokeLinecap="round"
                fill="none"
                transform="rotate(-90 90 90)"
                strokeDasharray={`${dash} ${c - dash}`}
                initial={{ pathLength: 0 }}
                animate={{ pathLength: 1 }}
                transition={{ duration: 1.1, ease: "easeOut" }}
              />
              <defs>
                <linearGradient id="scoreGradient" x1="0" x2="1">
                  <stop offset="0%" stopColor="#ff6f3c" />
                  <stop offset="60%" stopColor="#ffbf55" />
                  <stop offset="100%" stopColor="#3aa9d2" />
                </linearGradient>
              </defs>
              <text x="90" y="96" textAnchor="middle">{score}</text>
            </svg>
            <p style={{ textAlign: "center" }}>得分</p>
          </motion.div>

          <div onTouchStart={onDetailTouchStart} onTouchEnd={onDetailTouchEnd}>
            <button type="button" className="detail-toggle" onClick={() => setShowMore((value) => !value)}>
              {showMore ? "收起详细分析" : "上滑查看详细分析"}
            </button>
            <div className="metric-grid">
              <article className="metric-card">
                <strong>答对题数</strong>
                <p>
                  {correct}/{total}
                </p>
              </article>
              <article className="metric-card">
                <strong>获得 XP</strong>
                <p className="xp-counter">+{animatedXp}</p>
              </article>
              <article className="metric-card">
                <strong>准确率</strong>
                <p>{total > 0 ? Math.round((correct / total) * 100) : 0}%</p>
              </article>
            </div>

            <article className="review-card" style={{ marginTop: 12 }}>
              <strong>错题回顾</strong>
              <p>可返回剧场再次听对话并复盘错误选项，强化关键词理解。</p>
            </article>

            <article className="stage-banner" style={{ marginTop: 12 }}>
              <strong>
                <Lightbulb size={14} /> AI 建议
              </strong>
              <p>{result?.feedback ?? "你对场景上下文理解较稳定，建议继续进行角色扮演模式。"}</p>
              <p>说明：总经验统一以个人中心数据为准，页面学习指标仅用于过程反馈。</p>
            </article>

            {showMore ? (
              <motion.article
                className="extra-analysis"
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
              >
                <h4>进阶分析</h4>
                <p>词汇匹配：建议复听包含场景核心词的两句对话，优先纠正误判问题。</p>
                <p>语境理解：下一轮可切换角色扮演模式，验证主动表达和临场反应。</p>
              </motion.article>
            ) : null}

            <div className="xp-rain" aria-hidden>
              <span />
              <span />
              <span />
            </div>
          </div>
        </div>

        <div className="row" style={{ marginTop: 12 }}>
          <Link to="/generate" className="link-button">
            <RotateCcw size={16} /> 重新练习
          </Link>
          <Link to="/library" className="link-button">
            去剧场库
          </Link>
          <button type="button" className="btn-ghost">
            <Share2 size={16} /> 分享成绩
          </button>
        </div>
      </section>
    </main>
  );
}
