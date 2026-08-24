import { FormEvent, useEffect, useState } from "react";
import { ArrowLeft, Sparkles, Volume2, Waves } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { createVoiceProfile, getTTSConfig } from "../api";
import { AICreditCostNotice } from "../components/AICreditCostNotice";
import type { TTSConfig } from "../types";

type VoiceDesignPreset = {
  name: string;
  prompt: string;
};

const voiceDesignPresets: Record<"CANTONESE" | "ENGLISH", VoiceDesignPreset[]> = {
  CANTONESE: [
    { name: "阿晴 · 茶餐厅店员", prompt: "二十多岁香港女生，开朗亲切，发音自然清晰，带一点俏皮感；像茶餐厅店员与熟客聊天，保持自然香港粤语语流。" },
    { name: "子轩 · 电台学长", prompt: "二十多岁香港男生，低沉温暖，语速自然，不拖长句尾；像校园电台学长陪伴同学练习日常粤语。" },
    { name: "嘉敏 · 知性前辈", prompt: "三十岁左右香港女性，知性沉静，温和有耐心，咬字清楚；像职场前辈自然解释生活与工作场景。" },
    { name: "阿朗 · 运动同学", prompt: "二十岁左右香港男生，精神饱满、阳光开朗，节奏轻快但不夸张；像约朋友去运动的日常对话。" },
    { name: "心怡 · 温柔姐姐", prompt: "二十多岁香港女性，温柔松弛、富有亲和力，句间短暂停顿；像耐心安慰朋友的自然粤语。" },
    { name: "志豪 · 稳重导师", prompt: "三十多岁香港男性，沉稳可靠，节奏清晰，语气不生硬；像语言导师给出温和而直接的建议。" },
    { name: "小雯 · 活力同学", prompt: "十八到二十岁香港女生，清新明亮、带互动感，语速适中；像在校园里邀请同学一起吃午饭。" },
    { name: "家辉 · 街坊大哥", prompt: "三十岁左右香港男性，爽朗自然、略带幽默，吐字利落；像社区街坊热心帮忙时的日常粤语。" },
    { name: "思敏 · 冷静同事", prompt: "二十多岁香港女性，干练清楚、语调平稳，带一点专业感；像与同事确认工作安排的自然对话。" },
    { name: "浩然 · 文艺男生", prompt: "二十多岁香港男生，声音清澈温和、略带文艺感，语速从容；像在书店和朋友聊电影。" },
    { name: "颖儿 · 俏皮闺蜜", prompt: "二十多岁香港女生，俏皮轻快但不夸张，富有笑意；像和好朋友分享周末见闻的自然粤语。" },
    { name: "柏文 · 可靠邻居", prompt: "四十岁左右香港男性，声音成熟温暖、稳重友善，停顿自然；像邻居耐心指路和提供帮助。" }
  ],
  ENGLISH: [
    { name: "Mia · Café Friend", prompt: "A bright young woman with a warm, natural voice and clear English diction; she sounds like a friendly café regular in a relaxed conversation." },
    { name: "Leo · Radio Senior", prompt: "A young man with a low, warm, relaxed voice; natural pacing and short pauses, like a supportive campus radio host." },
    { name: "Nora · Calm Mentor", prompt: "A thoughtful woman in her thirties, calm, articulate and encouraging; suited to clear learning feedback without a broadcast tone." },
    { name: "Ethan · Active Classmate", prompt: "An energetic young man with an open, friendly tone; slightly lively pacing but never rushed or exaggerated." },
    { name: "Luna · Gentle Sister", prompt: "A gentle young woman with a soft, reassuring voice and natural pauses; friendly everyday English, never overly dramatic." },
    { name: "Daniel · Steady Coach", prompt: "A mature man with a steady, patient, clear voice; sounds like a helpful language coach in an everyday situation." },
    { name: "Ivy · Playful Peer", prompt: "A lively young woman with a playful, conversational tone; light and engaging without sounding childish or overly sweet." },
    { name: "Miles · Neighbour", prompt: "A friendly man in his thirties with a practical, relaxed speaking style; warm and approachable like a helpful neighbour." },
    { name: "Sofia · Clear Colleague", prompt: "A confident young woman with precise, professional but natural speech; ideal for workplace conversations and planning." },
    { name: "Noah · Bookshop Friend", prompt: "A clear, thoughtful young man with a slightly artistic, unhurried tone; like chatting about films in a bookshop." },
    { name: "Chloe · Weekend Friend", prompt: "A cheerful young woman with a smile in her voice and easy rhythm; suitable for sharing casual weekend plans." },
    { name: "Henry · Trusted Guide", prompt: "A mature, reliable man with a warm, composed voice; calm pacing and a quietly helpful presence." }
  ]
};

export function VoiceDesignPage() {
  const navigate = useNavigate();
  const [ttsConfig, setTTSConfig] = useState<TTSConfig>();
  const [name, setName] = useState("");
  const [prompt, setPrompt] = useState("");
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
  const [creating, setCreating] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    void getTTSConfig()
      .then(setTTSConfig)
      .catch((error) => {
        console.error("load tts config failed", error);
        setMessage("无法读取线上语音服务状态，请稍后重试。");
      });
  }, []);

  const canCreate = ttsConfig?.provider === "XIAOMI" && ttsConfig.hasApiKey;
  const presets = voiceDesignPresets[language];

  function applyPreset(preset: VoiceDesignPreset) {
    setName(preset.name);
    setPrompt(preset.prompt);
    setMessage(`已填入“${preset.name}”的设计描述，确认后即可创建。`);
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!canCreate) {
      setMessage("当前线上语音服务暂未就绪，请稍后再试。");
      return;
    }
    setCreating(true);
    setMessage("");
    try {
      await createVoiceProfile({ name: name.trim(), prompt: prompt.trim(), language });
      navigate("/voices", { state: { message: "音色已加入后台创建队列；完成后可在这里试听并用于剧场。" } });
    } catch (error) {
      console.error("create voice profile failed", error);
      setMessage((error as Error).message || "创建失败，请检查音色描述后稍后重试。");
    } finally {
      setCreating(false);
    }
  }

  return (
    <main className="page voice-design-page">
      <nav className="voice-module-breadcrumb" aria-label="音色库导航">
        <Link to="/voices"><ArrowLeft size={15} /> 角色音色库</Link>
        <span aria-hidden>／</span>
        <strong>创建音色</strong>
      </nav>

      <section className="card voice-design-card">
        <header className="voice-design-heading">
          <span className="eyebrow"><Waves size={15} /> Voice studio</span>
          <h2>创建音色</h2>
          <p>用角色提示词设计一条可复用的声线。生成完成后先在音色库试听检查，确认保存后才会出现在剧场的音色选择中。</p>
        </header>
        <form onSubmit={handleSubmit} className="voice-design-form">
          <div className="voice-design-fields">
            <label>
              <span>音色名称</span>
              <input value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：阿晴 · 温柔店员" maxLength={40} required />
            </label>
            <label>
              <span>主要语言</span>
              <select value={language} onChange={(event) => setLanguage(event.target.value as typeof language)}>
                <option value="CANTONESE">粤语</option>
                <option value="ENGLISH">英语</option>
              </select>
            </label>
          </div>
          <label>
            <span>音色描述</span>
            <textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="例如：二十多岁香港女生，明朗友善，语速自然，带一点俏皮感；适合茶餐厅店员的日常粤语对话。" rows={7} maxLength={500} minLength={8} required />
          </label>
          <section className="voice-preset-gallery" aria-label="推荐角色音色">
            <div className="voice-preset-heading">
              <div><strong>12 款角色音色灵感</strong><p>点击即可填入。每次创建会生成一条独立的可复用角色音色。</p></div>
              <span>{language === "CANTONESE" ? "香港粤语" : "English"}</span>
            </div>
            <div className="voice-preset-grid">
              {presets.map((preset, index) => (
                <button key={preset.name} type="button" className="voice-preset-card" onClick={() => applyPreset(preset)}>
                  <small>#{String(index + 1).padStart(2, "0")}</small>
                  <strong>{preset.name}</strong>
                  <span>{preset.prompt}</span>
                </button>
              ))}
            </div>
          </section>
          <aside className="voice-design-tip"><Volume2 size={18} /><span>建议写明年龄感、性别呈现、地区口音、语速、情绪和角色场景，生成效果会更稳定。</span></aside>
          <AICreditCostNotice action="VOICE_DESIGN" />
          <div className="voice-design-submit">
            <button type="submit" disabled={creating || !canCreate}><Sparkles size={16} /> {creating ? "正在创建…" : "创建音色"}</button>
            <small>支持 8–500 字的角色描述，生成过程不阻塞其他服务；生成结果需要试听确认。</small>
          </div>
          {!canCreate ? <p className="error">线上音色服务暂未就绪，请稍后再试。</p> : null}
          {message ? <p className="muted-note" role="status">{message}</p> : null}
        </form>
      </section>
    </main>
  );
}
