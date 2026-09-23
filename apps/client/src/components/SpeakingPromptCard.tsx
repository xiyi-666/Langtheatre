import type { SpeakingPrompt } from "../types";
import { resolveAudioUrl } from "../audio";

export function SpeakingPromptCard({ prompt }: { prompt?: SpeakingPrompt }) {
  if (!prompt) return <p className="muted-note">本轮题目已完成，可以提交分析。</p>;
  const sourceLabel = prompt.source?.includes("AI examiner")
    ? "AI 考官追问"
    : prompt.questionId
      ? "基础题库"
      : "内置练习";

  return (
    <article className="speaking-question-card" aria-labelledby="speaking-question-title">
      <div className="speaking-question-meta">
        <span className="speaking-part-badge">Part {prompt.part}</span>
        <span className="muted-note">{sourceLabel}</span>
      </div>
      <h2 id="speaking-question-title">当前题目</h2>
      <p className="speaking-question-copy" lang="en">{prompt.cueCard || prompt.question}</p>
      {prompt.source ? <p className="muted-note speaking-question-source">来源：{prompt.source.includes("AI examiner") ? "AI 考官基于本次上下文生成" : `${prompt.source}（历史练习资料，非当季预测）`}</p> : null}
      <div className="speaking-question-facts" aria-label="答题限制">
        <span>准备 {prompt.preparationSec} 秒</span>
        <span>建议回答 {prompt.answerSec} 秒</span>
      </div>
      <div className="speaking-prompt-audio">
        <p className="speaking-audio-label">考官题目朗读</p>
        {prompt.audioUrl ? (
          <audio key={prompt.audioUrl} aria-label="播放考官题目" controls preload="none" src={resolveAudioUrl(prompt.audioUrl)} />
        ) : (
          <p className="muted-note">本题暂无朗读音频，不影响查看题目与录音作答。</p>
        )}
      </div>
    </article>
  );
}
