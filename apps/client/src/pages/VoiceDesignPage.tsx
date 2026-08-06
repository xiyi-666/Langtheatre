import { FormEvent, useEffect, useState } from "react";
import { ArrowLeft, Sparkles, Volume2, Waves } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { createVoiceProfile, getTTSConfig } from "../api";
import { AICreditCostNotice } from "../components/AICreditCostNotice";
import type { TTSConfig } from "../types";

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
        setMessage("无法读取 TTS 配置，请返回个人中心检查设置。");
      });
  }, []);

  const canCreate = ttsConfig?.provider === "XIAOMI" && ttsConfig.hasApiKey;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!canCreate) {
      setMessage("请先在个人中心配置带 API Key 的小米 TTS。");
      return;
    }
    setCreating(true);
    setMessage("");
    try {
      await createVoiceProfile({ name: name.trim(), prompt: prompt.trim(), language });
      navigate("/voices", { state: { message: "音色已加入后台创建队列；完成后可在这里试听并用于剧场。" } });
    } catch (error) {
      console.error("create voice profile failed", error);
      setMessage((error as Error).message || "创建失败，请检查提示词与 TTS 配置。");
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
          <p>用角色提示词设计一条可复用的声线。任务在后台完成后，会自动回到音色库供试听和分配。</p>
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
          <aside className="voice-design-tip"><Volume2 size={18} /><span>建议写明年龄感、性别呈现、地区口音、语速、情绪和角色场景，生成效果会更稳定。</span></aside>
          <AICreditCostNotice action="VOICE_DESIGN" />
          <div className="voice-design-submit">
            <button type="submit" disabled={creating || !canCreate}><Sparkles size={16} /> {creating ? "正在创建…" : "创建音色"}</button>
            <small>支持 8–500 字的角色描述，生成过程不阻塞其他服务。</small>
          </div>
          {!canCreate ? <p className="error">当前需要已配置 API Key 的小米 TTS 才能创建音色。</p> : null}
          {message ? <p className="muted-note" role="status">{message}</p> : null}
        </form>
      </section>
    </main>
  );
}
