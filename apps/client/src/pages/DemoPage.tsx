import { ArrowRight, BookOpenText, Check, CheckCircle2, Clapperboard, Copy, FilePenLine, Headphones, Languages, Pause, Play, RotateCcw, Sparkles, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { playClip, speakText, stopAudioPlayback } from "../audio";
import { demoAccount, demoFixture } from "../demoFixture";

type DemoTab = "theater" | "reading" | "writing";
type TheaterLanguage = "cantonese" | "english";

export function DemoPage() {
  const [copied, setCopied] = useState<"username" | "password" | null>(null);
  const [copyError, setCopyError] = useState(false);
  const [activeTab, setActiveTab] = useState<DemoTab>("theater");
  const [theaterLanguage, setTheaterLanguage] = useState<TheaterLanguage>("cantonese");
  const [playingKey, setPlayingKey] = useState<string | null>(null);
  const [audioError, setAudioError] = useState("");

  useEffect(() => () => stopAudioPlayback(), []);

  const copyValue = async (field: "username" | "password") => {
    try {
      await navigator.clipboard.writeText(demoAccount[field]);
      setCopied(field);
      setCopyError(false);
      window.setTimeout(() => setCopied((current) => current === field ? null : current), 1800);
    } catch {
      setCopied(null);
      setCopyError(true);
    }
  };

  const playText = async (key: string, text: string, lang: string, audio?: string) => {
    if (playingKey === key) {
      stopAudioPlayback();
      setPlayingKey(null);
      return;
    }
    stopAudioPlayback();
    setPlayingKey(key);
    setAudioError("");
    try {
      if (audio) await playClip(audio);
      else await speakText(text, 0.95, lang);
    } catch {
      setAudioError("音频播放失败，请检查浏览器的播放权限后重试。");
    } finally {
      setPlayingKey((current) => current === key ? null : current);
    }
  };

  const theater = demoFixture.theaters[theaterLanguage];

  return (
    <main className="page demo-page">
      <div className="demo-shell">
        <header className="demo-hero">
          <div className="demo-hero-copy"><span className="section-kicker"><Sparkles size={14} /> LinguaQuest · 产品演示</span><h1>先看结果，再开始练习。</h1><p>这里展示的是已经准备好的阅读、写作和双语剧场内容。无需等待，不会调用 AI，不消耗点数，也不会保存任何内容。</p><div className="demo-hero-actions"><Link className="demo-primary-action" to="/login?from=demo">登录演示账号并进入产品 <ArrowRight size={17} /></Link><Link className="btn-ghost demo-back-action" to="/login">返回登录</Link></div></div>
          <aside className="demo-ready-note" aria-label="演示状态"><span className="demo-ready-mark"><Check size={19} /></span><div><strong>示例已准备好</strong><p>预置结果 · 无需生成</p></div></aside>
        </header>

        <section className="demo-account-card" aria-labelledby="demo-account-title"><div className="demo-account-copy"><span className="demo-label">快速进入</span><h2 id="demo-account-title">演示账号</h2><p>登录后可查看完整产品流程，仅用于演示体验。</p></div><div className="demo-account-fields">{(["username", "password"] as const).map((field) => <div className="demo-account-field" key={field}><span>{field === "username" ? "用户名" : "密码"}</span><code>{demoAccount[field]}</code><button type="button" className="demo-copy-button" onClick={() => void copyValue(field)} aria-label={`复制${field === "username" ? "用户名" : "密码"}`}>{copied === field ? <Check size={15} /> : <Copy size={15} />}{copied === field ? "已复制" : "复制"}</button></div>)}<small className={copyError ? "demo-copy-feedback is-error" : "demo-copy-feedback"} role="status">{copyError ? "复制失败，请手动输入账号信息。" : copied ? "账号信息已复制。" : "密码区分大小写。"}</small></div></section>

        <section className="demo-results" aria-labelledby="demo-results-title"><div className="demo-results-heading"><div><span className="demo-label">预置结果</span><h2 id="demo-results-title">打开就能体验的三个练习</h2></div><span className="demo-static-badge">静态演示，不产生 AI 消耗</span></div><nav className="demo-tabs" aria-label="演示内容选择"><button type="button" className={activeTab === "theater" ? "active" : ""} onClick={() => { stopAudioPlayback(); setPlayingKey(null); setActiveTab("theater"); }}><Clapperboard size={17} /> 双语剧场</button><button type="button" className={activeTab === "reading" ? "active" : ""} onClick={() => { stopAudioPlayback(); setPlayingKey(null); setActiveTab("reading"); }}><BookOpenText size={17} /> 英语阅读</button><button type="button" className={activeTab === "writing" ? "active" : ""} onClick={() => { stopAudioPlayback(); setPlayingKey(null); setActiveTab("writing"); }}><FilePenLine size={17} /> 英语写作</button></nav>

          {activeTab === "theater" && <article className="demo-result-panel" aria-labelledby="demo-theater-title"><div className="demo-panel-heading"><div><span className="demo-label">剧场 · {theaterLanguage === "cantonese" ? "粤语" : "English"}</span><h3 id="demo-theater-title">{theater.title}</h3><p className="demo-muted">{theater.subtitle} · {theater.speakers}</p></div><span className="demo-badge">已准备</span></div><div className="demo-language-switch" role="group" aria-label="剧场语言"><button type="button" className={theaterLanguage === "cantonese" ? "active" : ""} onClick={() => { stopAudioPlayback(); setPlayingKey(null); setTheaterLanguage("cantonese"); }}>粤语剧场</button><button type="button" className={theaterLanguage === "english" ? "active" : ""} onClick={() => { stopAudioPlayback(); setPlayingKey(null); setTheaterLanguage("english"); }}>英语剧场</button></div><div className="demo-audio-control"><span><Headphones size={15} /> {theaterLanguage === "cantonese" ? "预置粤语 MP3" : "浏览器英语语音"} · 不调用 TTS</span><span className="demo-audio-hint">点击每句台词右侧的播放按钮试听</span></div>{audioError ? <p className="demo-audio-error" role="alert">{audioError}</p> : null}<div className="demo-dialogue-list">{theater.lines.map((line, index) => { const key = `${theaterLanguage}-line-${index}`; const isPlaying = playingKey === key; return <div className={`demo-dialogue ${index % 2 ? "is-second" : ""}`} key={key}><div className="demo-dialogue-speaker"><strong>{line.speaker}</strong><small>{line.role}</small><button type="button" className="demo-line-play" onClick={() => void playText(key, line.text, theaterLanguage === "cantonese" ? "zh-HK" : (index % 2 ? "en-US" : "en-GB"), "audio" in line ? line.audio : undefined)} aria-label={`${isPlaying ? "停止" : "播放"}${line.speaker}的台词`}>{isPlaying ? <Pause size={15} /> : <Play size={15} />}{isPlaying ? "停止" : "播放"}</button></div><p>{line.text}</p><small>{line.translation}</small></div>; })}</div></article>}

          {activeTab === "reading" && <article className="demo-result-panel" aria-labelledby="demo-reading-title"><div className="demo-panel-heading"><div><span className="demo-label">阅读 · English</span><h3 id="demo-reading-title">{demoFixture.reading.title}</h3><p className="demo-muted">{demoFixture.reading.level}</p></div><span className="demo-badge">已准备</span></div><div className="demo-reading-audio"><button type="button" onClick={() => void playText("reading", demoFixture.reading.paragraphs.join(" "), "en-US")}><Headphones size={16} /> {playingKey === "reading" ? "停止朗读" : "播放全文"}</button><span>本地预置朗读，不调用 AI</span>{playingKey === "reading" ? <Square size={14} /> : null}</div><div className="demo-reading-body">{demoFixture.reading.paragraphs.map((paragraph, index) => <div className="demo-reading-paragraph" key={paragraph}><p>{paragraph}</p><button type="button" className="demo-inline-play" onClick={() => void playText(`reading-${index}`, paragraph, "en-US")} aria-label={`播放第 ${index + 1} 段`}>{playingKey === `reading-${index}` ? <Pause size={14} /> : <Play size={14} />} {playingKey === `reading-${index}` ? "停止" : "播放本段"}</button></div>)}</div><div className="demo-question"><strong>示例题目</strong><p>{demoFixture.reading.question}</p><ol>{demoFixture.reading.options.map((option, index) => <li className={option === demoFixture.reading.answer ? "is-answer" : ""} key={option}>{String.fromCharCode(65 + index)}. {option}</li>)}</ol><p className="demo-answer"><CheckCircle2 size={16} /> 正确答案：{demoFixture.reading.answer}</p><p className="demo-explanation">{demoFixture.reading.explanation}</p></div></article>}

          {activeTab === "writing" && <article className="demo-result-panel" aria-labelledby="demo-writing-title"><div className="demo-panel-heading"><div><span className="demo-label">写作 · IELTS</span><h3 id="demo-writing-title">{demoFixture.writing.title}</h3></div><div className="demo-score"><strong>{demoFixture.writing.score}</strong><small>/ 9.0</small></div></div><div className="demo-writing-body"><p className="demo-writing-prompt"><strong>题目</strong>{demoFixture.writing.prompt}</p><p className="demo-writing-sample"><strong>用户文章示例</strong>{demoFixture.writing.sample}</p><div className="demo-score-details">{demoFixture.writing.dimensions.map((item) => <div key={item.name}><span>{item.name}</span><strong>{item.score}</strong><small>{item.detail}</small></div>)}</div><div className="demo-suggestions"><strong>改进建议</strong><ul>{demoFixture.writing.suggestions.map((suggestion) => <li key={suggestion}>{suggestion}</li>)}</ul></div></div></article>}
        </section>

        <footer className="demo-footer-note"><Languages size={16} /> <span>演示数据全部来自本地预置内容，剧场和阅读均可直接试听。进入真实模式后，生成任务才会根据账户配置消耗点数。</span><RotateCcw size={15} /></footer>
      </div>
    </main>
  );
}
