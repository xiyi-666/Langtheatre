import { ArrowRight, BookOpenText, Check, CheckCircle2, Clapperboard, Copy, FilePenLine, Headphones, Languages, Pause, Play, RotateCcw, Sparkles, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { playClip, speakText, stopAudioPlayback } from "../audio";

const cantoneseAudio = [
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/68528b9f35440c9bd95637850f839e0acc02b76d38d10e0ff665d67a2d3916c7.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/cf7f1d4683105bd0861acaad2cc7f9664fcf10c8be713a13e6e883946aba17c9.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/cf4091d8d8dcb4e6f4d38f67049f60a37f4bc2173d546e615eed0fe31a953a4b.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/b88fef3303aeee253aecbc0641503962d6bd18cd74b9f84744e7ab9a43bb7377.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/52d8c6fe3d53fc131f40769221695ce8751bea42366b07c24964897a84400cfa.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/4ae908c2cb2e1e50f9424e604c2da19ef78be04637d48cf4d81f3fdf9e18ac3b.mp3"
] as const;

export const demoFixture = {
  // 保留旧版 fixture 字段，便于已有测试和外部演示组件兼容。
  dialogue: [
    { speaker: "阿明", role: "男声", text: "喂，你今日收工之后得唔得閒飲杯奶茶？", translation: "你今天下班后有空喝杯奶茶吗？" }
  ],
  theaters: {
    cantonese: {
      title: "茶餐厅后的约会",
      subtitle: "粤语 · 双人多轮对话",
      speakers: "阿明（男声）与阿May（女声）",
      lines: [
        { speaker: "阿明", role: "男声", text: "喂，你今日收工之后得唔得閒飲杯奶茶？", translation: "你今天下班后有空喝杯奶茶吗？", audio: cantoneseAudio[0] },
        { speaker: "阿May", role: "女声", text: "得呀，六点半喺地铁站出口等你。", translation: "可以呀，六点半在地铁站出口等你。", audio: cantoneseAudio[1] },
        { speaker: "阿明", role: "男声", text: "好啊，不过我今日可能会迟少少。", translation: "好啊，不过我今天可能会晚一点。", audio: cantoneseAudio[2] },
        { speaker: "阿May", role: "女声", text: "唔紧要，我顺便去隔离间书店行吓。", translation: "没关系，我正好去旁边的书店逛逛。", audio: cantoneseAudio[3] },
        { speaker: "阿明", role: "男声", text: "咁我哋一阵见，记得帮我少甜呀。", translation: "那我们一会儿见，记得帮我点少糖的。", audio: cantoneseAudio[4] },
        { speaker: "阿May", role: "女声", text: "知道喇，凍奶茶少甜，唔会搞错。", translation: "知道啦，少糖冻奶茶，不会弄错。", audio: cantoneseAudio[5] }
      ]
    },
    english: {
      title: "A thoughtful change of plans",
      subtitle: "English · Two-person conversation",
      speakers: "Maya and Daniel · browser voice preview",
      lines: [
        { speaker: "Maya", role: "Female voice", text: "Are you still coming to the study group after work?", translation: "你下班后还来学习小组吗？" },
        { speaker: "Daniel", role: "Male voice", text: "I am, but I may arrive ten minutes late because of the rain.", translation: "我会来，不过下雨可能会迟到十分钟。" },
        { speaker: "Maya", role: "Female voice", text: "That is fine. I will save you a seat near the window.", translation: "没关系，我会在窗边帮你留一个座位。" },
        { speaker: "Daniel", role: "Male voice", text: "Thanks. I have prepared a few questions about the article.", translation: "谢谢，我准备了几个关于文章的问题。" }
      ]
    }
  },
  reading: {
    title: "A small habit, a lasting change", level: "CEFR B1 · 预置阅读材料",
    paragraphs: [
      "Small daily routines make learning feel manageable. Instead of waiting for a free afternoon, many learners choose one short activity that they can repeat every day.",
      "For example, a learner may read a short article during breakfast, notice one useful phrase, and use it in a real conversation later. The activity is simple, but the repeated connection between reading and speaking makes progress easier to sustain.",
      "The most effective habit is not always the most ambitious one. It is the habit that fits naturally into a learner's life and can continue when motivation is low."
    ],
    question: "Why can a small daily activity be effective for language learners?",
    options: ["It removes the need to speak with other people.", "It connects repeated practice with real-life use.", "It guarantees a free afternoon for studying."],
    answer: "It connects repeated practice with real-life use.",
    explanation: "第二段说明，阅读短文、记录表达并在真实对话中使用，能把输入和输出连接起来，因此 B 是正确答案。"
  },
  writing: {
    title: "IELTS · Discuss both views",
    prompt: "Some people think schools should focus more on practical skills than academic subjects. Discuss both views and give your own opinion.",
    sample: "Schools have traditionally placed academic subjects at the centre of education. While these subjects build a strong foundation, practical skills also deserve more attention because they prepare students for everyday decisions and future work. In my view, a balanced curriculum is the most useful approach.",
    score: 7.5,
    dimensions: [
      { name: "任务回应", score: "7.5", detail: "回应了双方观点，并清楚表达个人立场。" },
      { name: "连贯与衔接", score: "7.0", detail: "整体结构清晰，但第二段可以增加更自然的过渡。" },
      { name: "词汇资源", score: "7.5", detail: "词汇准确，能使用 foundation、curriculum 等学术表达。" },
      { name: "语法准确性", score: "8.0", detail: "句式有变化，复杂句使用准确，明显语法错误较少。" }
    ],
    suggestions: ["补充一个具体例子，支持 practical skills 的观点。", "用 However / As a result 等连接句强化段落关系。", "结尾可以再次概括 balanced curriculum 的实际好处。"]
  }
} as const;

// 这个账号只用于演示入口，所有演示结果均来自前端 fixture。
export const demoAccount = { username: "lingua_demo_0903", password: "LqDemo2026!" } as const;

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
