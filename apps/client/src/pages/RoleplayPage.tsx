import { FormEvent, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { motion } from "framer-motion";
import { MessageSquare, Mic2, PlayCircle, SquareCheckBig } from "lucide-react";
import { endRoleplay, startRoleplay, submitRoleplayReply } from "../api";
import { useAppStore } from "../store";

export function RoleplayPage() {
  const { theaterId = "" } = useParams();
  const roleplay = useAppStore((s) => s.roleplay);
  const user = useAppStore((s) => s.user);
  const setRoleplay = useAppStore((s) => s.setRoleplay);
  const [text, setText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [showZhSubtitle, setShowZhSubtitle] = useState(true);
  const navigate = useNavigate();
  const latestEvaluationText = useMemo(() => {
    if (!roleplay?.transcript?.length) return "";

    const stripSuggestedAnswer = (text: string) =>
      text
        .replace(/[\s\S]*?(本轮评分|Turn score)/, "$1")
        .replace(/\n?\s*(建议回答|参考回答|Suggested reply|Model answer)[:：][\s\S]*/i, "")
        .trim();

    for (let i = roleplay.transcript.length - 1; i >= 0; i -= 1) {
      const line = roleplay.transcript[i];
      if (!line?.text) continue;
      if (line.text.includes("本轮评分") || line.text.includes("Turn score")) {
        return stripSuggestedAnswer(line.text);
      }
    }
    return "";
  }, [roleplay]);

  const parsedEvaluation = useMemo(() => {
    if (!latestEvaluationText) {
      return { score: "", strengths: [] as string[], improvements: [] as string[], summary: "" };
    }

    const lines = latestEvaluationText
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean);

    const scoreLine =
      lines.find((line) => /^(本轮评分|Turn score)/i.test(line)) ??
      lines.find((line) => /(评分|score)/i.test(line)) ??
      "";

    const strengths = lines.filter((line) => /(优点|亮点|做得好|strength|good point)/i.test(line));
    const improvements = lines.filter((line) => /(改进|建议|提升|improve|suggestion|tip)/i.test(line));

    const summary = lines
      .filter((line) => line !== scoreLine && !strengths.includes(line) && !improvements.includes(line))
      .join("\n");

    return { score: scoreLine, strengths, improvements, summary };
  }, [latestEvaluationText]);

  const visibleTranscript = useMemo(() => {
    if (!roleplay?.transcript?.length) return [];
    return roleplay.transcript.filter((item) => {
      const text = item?.text ?? "";
      return !(text.includes("本轮评分") || text.includes("Turn score"));
    });
  }, [roleplay]);

  async function handleStart() {
    try {
      const userRole = (user?.nickname || user?.email?.split("@")[0] || "Learner").trim();
      const session = await startRoleplay(theaterId, userRole);
      setRoleplay(session);
    } catch (e) {
      console.error("start roleplay failed", e);
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!roleplay || !text.trim()) return;
    setSubmitting(true);
    try {
      const updated = await submitRoleplayReply(roleplay.id, text);
      setRoleplay(updated);
      setText("");
    } catch (e) {
      console.error("submit roleplay reply failed", e);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleEnd() {
    if (!roleplay) return;
    try {
      const completed = await endRoleplay(roleplay.id);
      setRoleplay(completed);
    } catch (e) {
      console.error("end roleplay failed", e);
    }
  }

  return (
    <main className="page">
      <section className="card">
        <h2>角色扮演模式</h2>
        <p>按回合推进对话，系统会持续评估你的上下文匹配与表达质量。</p>
        <div className="row">
          <button onClick={handleStart}><PlayCircle size={16} /> 开始会话</button>
          <button onClick={handleEnd} disabled={!roleplay}><SquareCheckBig size={16} /> 结束会话</button>
          <button className="btn-ghost" onClick={() => setShowZhSubtitle((value) => !value)}>
            {showZhSubtitle ? "隐藏简体中文字幕" : "显示简体中文字幕"}
          </button>
          <button onClick={() => navigate("/library")}>返回剧场库</button>
        </div>
        {roleplay ? (
          <div className="roleplay-grid">
            <aside className="floating-panel">
              <h3>会话状态</h3>
              <p>当前评分：<strong className="score-pulse">{roleplay.currentScore}</strong></p>
              <p>状态：{roleplay.status}</p>
              <p>回合：{roleplay.turnIndex + 1}</p>
              <p><Mic2 size={14} /> 建议每轮控制在 1-2 句，保持场景连贯。</p>
            </aside>

            <section>
              <ul className="dialogue-list transcript-panel">
              {visibleTranscript.map((item, idx) => (
                <motion.li
                  key={`${idx}-${item.speaker}`}
                  className={idx % 2 === 0 ? "speaker-left" : "speaker-right"}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                >
                  <strong>{item.speaker}</strong> {item.text}
                  {showZhSubtitle && item.speaker === "AI-Role" && item.zhSubtitle ? (
                    <p style={{ margin: "4px 0 0", fontSize: 13, opacity: 0.8 }}>{item.zhSubtitle}</p>
                  ) : null}
                </motion.li>
              ))}
              </ul>
              <form onSubmit={handleSubmit} className="row" style={{ marginTop: 10 }}>
                <input value={text} onChange={(e) => setText(e.target.value)} placeholder="输入你的回复" />
                <button type="submit" disabled={submitting}>{submitting ? "提交中..." : "提交回复"}</button>
              </form>
              <p><MessageSquare size={14} /> 回答后系统会生成下一句并更新评分。</p>
              {latestEvaluationText ? (
                <article className="stage-banner" style={{ marginTop: 8 }}>
                  <strong>即时评估</strong>
                  {parsedEvaluation.score ? <p style={{ margin: "6px 0 0" }}><strong>{parsedEvaluation.score}</strong></p> : null}
                  {parsedEvaluation.summary ? <p style={{ whiteSpace: "pre-wrap" }}>{parsedEvaluation.summary}</p> : null}
                  {parsedEvaluation.strengths.length ? (
                    <div style={{ marginTop: 6 }}>
                      <strong>亮点</strong>
                      <ul style={{ margin: "4px 0 0 18px" }}>
                        {parsedEvaluation.strengths.map((item, idx) => (
                          <li key={`strength-${idx}`}>{item}</li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                  {parsedEvaluation.improvements.length ? (
                    <div style={{ marginTop: 6 }}>
                      <strong>可改进</strong>
                      <ul style={{ margin: "4px 0 0 18px" }}>
                        {parsedEvaluation.improvements.map((item, idx) => (
                          <li key={`improve-${idx}`}>{item}</li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                </article>
              ) : null}
              {roleplay.finalFeedback ? <article className="stage-banner"><strong>总结反馈</strong><p>{roleplay.finalFeedback}</p></article> : null}
            </section>
          </div>
        ) : (
          <p>点击“开始会话”进入角色扮演。</p>
        )}
      </section>
    </main>
  );
}
