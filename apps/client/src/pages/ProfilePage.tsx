import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
import { motion } from "framer-motion";
import { AudioLines, BadgeCheck, BrainCircuit, ChevronDown, ChevronUp, Crown, Eye, EyeOff, History, IdCard, LogOut, Mail, SlidersHorizontal, UserRound, Waves } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { isCommercialEdition, isMiniProgramEdition } from "../edition";
import { getASRConfig, getModelConfig, getTTSConfig, logout, me, updateASRConfig, updateModelConfig, updateProfile, updateTTSConfig } from "../api";
import { ASR_PROVIDER_PRESETS, getASRProviderPreset, type ASRProviderId } from "../asrProviders";
import { getModelProviderPreset, MODEL_PROVIDER_PRESETS, type ModelProviderId } from "../modelProviders";
import { useAppStore } from "../store";
import {
  getTTSProviderPreset,
  getXiaomiTTSModelPreset,
  isAudioDataUrl,
  isXiaomiPresetVoice,
  isXiaomiVoiceCloneModel,
  isXiaomiVoiceDesignModel,
  shouldSwapToProviderDefault,
  TTS_PROVIDER_PRESETS,
  XIAOMI_TTS_MODEL_PRESETS,
  type TTSProviderId,
  type XiaomiTTSModelId
} from "../ttsProviders";
import type { ASRConfig, ModelConfig, TTSConfig } from "../types";

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("read file failed"));
    reader.onload = () => resolve(typeof reader.result === "string" ? reader.result : "");
    reader.readAsDataURL(file);
  });
}

async function inspectAudioSample(file: File): Promise<{ duration: number; rms: number; peak: number }> {
  const audioContext = new AudioContext();
  try {
    const buffer = await audioContext.decodeAudioData(await file.arrayBuffer());
    const stride = Math.max(1, Math.floor(buffer.length / 100000));
    let sumSquares = 0;
    let samples = 0;
    let peak = 0;
    for (let channel = 0; channel < buffer.numberOfChannels; channel += 1) {
      const data = buffer.getChannelData(channel);
      for (let index = 0; index < data.length; index += stride) {
        const amplitude = Math.abs(data[index]);
        sumSquares += amplitude * amplitude;
        peak = Math.max(peak, amplitude);
        samples += 1;
      }
    }
    return {
      duration: buffer.duration,
      rms: samples > 0 ? Math.sqrt(sumSquares / samples) : 0,
      peak
    };
  } finally {
    await audioContext.close().catch(() => undefined);
  }
}

export function ProfilePage() {
	const navigate = useNavigate();
  const user = useAppStore((s) => s.user);
  const setUser = useAppStore((s) => s.setUser);
  const clearSession = useAppStore((s) => s.clearSession);
  const [nickname, setNickname] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [bio, setBio] = useState("");
  const [profileMessage, setProfileMessage] = useState("");
  const [modelMessage, setModelMessage] = useState("");
  const [ttsMessage, setTTSMessage] = useState("");
  const [loggingOut, setLoggingOut] = useState(false);

  const [asrMessage, setASRMessage] = useState("");
  const [avatarLoadError, setAvatarLoadError] = useState(false);
  const [showEmail, setShowEmail] = useState(false);
  const [modelConfig, setModelConfig] = useState<ModelConfig>();
  const [modelSectionOpen, setModelSectionOpen] = useState(false);
  const [modelProvider, setModelProvider] = useState<ModelProviderId>("OPENAI");
  const [modelName, setModelName] = useState("");
  const [modelBaseURL, setModelBaseURL] = useState("");
  const [modelAPIKey, setModelAPIKey] = useState("");
  const [ttsConfig, setTTSConfig] = useState<TTSConfig>();
  const [ttsSectionOpen, setTTSSectionOpen] = useState(false);
  const [ttsProvider, setTTSProvider] = useState<TTSProviderId>("XIAOMI");
  const [ttsModel, setTTSModel] = useState("");
  const [ttsBaseURL, setTTSBaseURL] = useState("");
  const [ttsVoice, setTTSVoice] = useState("");
  const [ttsAPIKey, setTTSAPIKey] = useState("");

  const [asrConfig, setASRConfig] = useState<ASRConfig>();
  const [asrSectionOpen, setASRSectionOpen] = useState(false);
  const [asrProvider, setASRProvider] = useState<ASRProviderId>("XIAOMI");
  const [asrModel, setASRModel] = useState("");
  const [asrBaseURL, setASRBaseURL] = useState("");
  const [asrAPIKey, setASRAPIKey] = useState("");
  const [asrAppID, setASRAppID] = useState("");
  const [ttsVoiceFileLabel, setTTSVoiceFileLabel] = useState("");
  const currentModelPreset = useMemo(() => getModelProviderPreset(modelProvider), [modelProvider]);
  const currentTTSPreset = useMemo(() => getTTSProviderPreset(ttsProvider), [ttsProvider]);
  const currentXiaomiModelPreset = useMemo(() => getXiaomiTTSModelPreset(ttsModel), [ttsModel]);
  const currentASRPreset = useMemo(() => getASRProviderPreset(asrProvider), [asrProvider]);
  const hasASRKeyForSelectedProvider = asrConfig?.provider === asrProvider && asrConfig.hasApiKey;

  function applyTTSConfig(next: TTSConfig) {
    const normalizedProvider = getTTSProviderPreset(next.provider).id;
    const nextModel = next.model ?? "";
    const nextVoice = next.voice ?? "";
    setTTSConfig(next);
    setTTSProvider(normalizedProvider);
    setTTSModel(nextModel);
    setTTSBaseURL(next.baseURL ?? "");
    setTTSVoice(nextVoice);
    setTTSVoiceFileLabel(
      normalizedProvider === "XIAOMI" && isXiaomiVoiceCloneModel(nextModel) && isAudioDataUrl(nextVoice)
        ? "已保存参考音频样本"
        : ""
    );
  }

  const safeAvatarUrl = useMemo(() => {
    const value = avatarUrl.trim();
    if (!value) return "";
    try {
      const parsed = new URL(value);
      return parsed.protocol === "http:" || parsed.protocol === "https:" ? value : "";
    } catch {
      return "";
    }
  }, [avatarUrl]);


  useEffect(() => {
    void (async () => {
      try {
		const [profile, config, tts, asr] = await Promise.all([me(), getModelConfig(), getTTSConfig(), getASRConfig()]);
        setUser(profile);
        setNickname(profile.nickname ?? "");
        setAvatarUrl(profile.avatarUrl ?? "");
        setBio(profile.bio ?? "");
        setModelConfig(config);
        setModelProvider(getModelProviderPreset(config.provider).id);
        setModelName(config.model ?? "");
        setModelBaseURL(config.baseURL ?? "");
        applyTTSConfig(tts);
        setASRConfig(asr); setASRProvider(getASRProviderPreset(asr.provider).id); setASRModel(asr.model); setASRBaseURL(asr.baseURL); setASRAppID(asr.appId ?? "");
      } catch (e) {
        console.error("load profile, model config, or tts config failed", e);
      }
    })();
  }, [setUser]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setProfileMessage("");
    try {
      const updated = await updateProfile({ nickname, avatarUrl, bio });
      setUser(updated);
      setProfileMessage("资料已更新");
    } catch (e) {
      console.error("update profile failed", e);
    }
  }

  async function handleLogout() {
    setLoggingOut(true);
    try {
      await logout();
    } catch (error) {
      // 本地会话仍需清理，确保后端短暂不可用时用户也能安全退出。
      console.warn("remote logout failed; clearing local session", error);
    } finally {
      localStorage.removeItem("accessToken");
			localStorage.removeItem("refreshToken");
      clearSession();
      navigate("/login", { replace: true });
    }
  }

  async function handleModelSubmit(event: FormEvent) {
    event.preventDefault();
    setModelMessage("");
    try {
      const updated = await updateModelConfig({
        provider: modelProvider,
        model: modelName.trim(),
        baseURL: modelBaseURL.trim(),
        apiKey: modelAPIKey.trim()
      });
      setModelConfig(updated);
      setModelProvider(getModelProviderPreset(updated.provider).id);
      setModelName(updated.model ?? "");
      setModelBaseURL(updated.baseURL ?? "");
      setModelAPIKey("");
      setModelMessage("模型配置已更新，后续生成会立即使用新配置。");
    } catch (e) {
      console.error("update model config failed", e);
    }
  }

  async function handleTTSSubmit(event: FormEvent) {
    event.preventDefault();
    setTTSMessage("");
    if (ttsProvider === "XIAOMI" && isXiaomiVoiceCloneModel(ttsModel) && !isAudioDataUrl(ttsVoice)) {
      setTTSMessage("VoiceClone 需要先上传参考音频，再保存配置。");
      return;
    }
    try {
      const updated = await updateTTSConfig({
        provider: ttsProvider,
        model: ttsModel.trim(),
        baseURL: ttsBaseURL.trim(),
        voice: ttsVoice.trim(),
        apiKey: ttsAPIKey.trim()
      });
      applyTTSConfig(updated);
      setTTSAPIKey("");
      setTTSMessage("TTS 配置已更新，后续剧场和阅读音频会立即使用新配置。");
    } catch (e) {
      console.error("update tts config failed", e);
      setTTSMessage("TTS 配置更新失败，请检查 Base URL 和 Key。");
    }
  }

  async function handleASRSubmit(event: FormEvent) {
    event.preventDefault(); setASRMessage("");
    try {
      const updated = await updateASRConfig({ provider: asrProvider, model: asrModel.trim(), baseURL: asrBaseURL.trim(), apiKey: asrAPIKey.trim(), appId: asrAppID.trim() });
      setASRConfig(updated); setASRProvider(getASRProviderPreset(updated.provider).id); setASRModel(updated.model); setASRBaseURL(updated.baseURL); setASRAppID(updated.appId ?? ""); setASRAPIKey("");
      setASRMessage("ASR 配置已更新，剧场语音练习会在下一轮使用新配置。");
    } catch (error) { console.error("update asr config failed", error); setASRMessage((error as Error).message || "ASR 配置更新失败，请检查地址和 Key。"); }
  }

  function handleASRProviderChange(nextProvider: ASRProviderId) {
    const previousPreset = getASRProviderPreset(asrProvider);
    const nextPreset = getASRProviderPreset(nextProvider);
    setASRProvider(nextProvider);
    setASRBaseURL((previous) => (
      shouldSwapToProviderDefault(previous, previousPreset.defaults.baseURL)
        ? nextPreset.defaults.baseURL
        : previous
    ));
    setASRModel((previous) => (
      shouldSwapToProviderDefault(previous, previousPreset.defaults.model)
        ? nextPreset.defaults.model
        : previous
    ));
    if (!nextPreset.requiresAppID) {
      setASRAppID("");
    }
  }

  function handleModelProviderChange(nextProvider: ModelProviderId) {
    const previousPreset = getModelProviderPreset(modelProvider);
    const nextPreset = getModelProviderPreset(nextProvider);
    setModelProvider(nextProvider);
    setModelBaseURL((prev) => (
      shouldSwapToProviderDefault(prev, previousPreset.defaults.baseURL)
        ? nextPreset.defaults.baseURL
        : prev
    ));
    setModelName((prev) => (
      shouldSwapToProviderDefault(prev, previousPreset.defaults.model)
        ? nextPreset.defaults.model
        : prev
    ));
  }

  function handleTTSProviderChange(nextProvider: TTSProviderId) {
    const previousPreset = getTTSProviderPreset(ttsProvider);
    const nextPreset = getTTSProviderPreset(nextProvider);
    const leavingXiaomiAdvancedModel = previousPreset.id === "XIAOMI" && ttsModel.trim() !== previousPreset.defaults.model;
    const providerActuallyChanged = previousPreset.id !== nextProvider;
    setTTSProvider(nextProvider);
    setTTSBaseURL((prev) => (
      shouldSwapToProviderDefault(prev, previousPreset.defaults.baseURL)
        ? nextPreset.defaults.baseURL
        : prev
    ));
    setTTSModel((prev) => (
      leavingXiaomiAdvancedModel ||
      shouldSwapToProviderDefault(prev, previousPreset.defaults.model) ||
      (providerActuallyChanged && nextPreset.modelOptions.length > 0 && !nextPreset.modelOptions.includes(prev.trim()))
        ? nextPreset.defaults.model
        : prev
    ));
    setTTSVoice((prev) => (
      leavingXiaomiAdvancedModel ||
      prev.trim() === "female-1" ||
      shouldSwapToProviderDefault(prev, previousPreset.defaults.voice) ||
      (providerActuallyChanged && nextPreset.voiceOptions.length > 0 && !nextPreset.voiceOptions.includes(prev.trim())) ||
      (nextProvider === "XIAOMI" && !isXiaomiPresetVoice(prev))
        ? nextPreset.defaults.voice
        : prev
    ));
    if (nextProvider !== "XIAOMI") {
      setTTSVoiceFileLabel("");
    }
  }

  function handleXiaomiModelChange(nextModel: XiaomiTTSModelId) {
    const previousModelPreset = getXiaomiTTSModelPreset(ttsModel);
    const nextModelPreset = getXiaomiTTSModelPreset(nextModel);
    setTTSModel(nextModel);
    setTTSVoice((prev) => {
      const trimmed = prev.trim();
      if (nextModel === "mimo-v2.5-tts") {
        return isXiaomiPresetVoice(trimmed) ? trimmed : nextModelPreset.defaultVoice;
      }
      if (nextModel === "mimo-v2.5-tts-voicedesign") {
        if (
          trimmed === "" ||
          isXiaomiPresetVoice(trimmed) ||
          isAudioDataUrl(trimmed) ||
          trimmed === previousModelPreset.defaultVoice
        ) {
          return nextModelPreset.defaultVoice;
        }
        return trimmed;
      }
      return isAudioDataUrl(trimmed) ? trimmed : "";
    });
    setTTSVoiceFileLabel((prev) => (
      nextModel === "mimo-v2.5-tts-voiceclone" && isAudioDataUrl(ttsVoice)
        ? (prev || "已保存参考音频样本")
        : ""
    ));
  }

  async function handleXiaomiVoiceCloneUpload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      setTTSMessage("参考音频需控制在 10MB 以内。");
      event.target.value = "";
      return;
    }
    const fileName = file.name.toLowerCase();
    const supportedFormat = ["audio/mpeg", "audio/mp3", "audio/wav", "audio/x-wav"].includes(file.type) || fileName.endsWith(".mp3") || fileName.endsWith(".wav");
    if (!supportedFormat) {
      setTTSMessage("小米 VoiceClone 目前只接受 MP3 或 WAV 参考音频。");
      event.target.value = "";
      return;
    }
    try {
      const inspection = await inspectAudioSample(file);
      if (inspection.duration < 6) {
        setTTSMessage("参考音频至少需要 6 秒；推荐使用 15–30 秒的单人自然粤语或英语语料。");
        event.target.value = "";
        return;
      }
      if (inspection.duration > 60) {
        setTTSMessage("参考音频建议控制在 60 秒以内，过长会增加处理时间且不一定提升音色稳定性。");
        event.target.value = "";
        return;
      }
      if (inspection.rms < 0.006 || inspection.peak < 0.03) {
        setTTSMessage("参考音频音量过低或接近静音，请换一段清晰、连续的单人语音。");
        event.target.value = "";
        return;
      }
      const dataUrl = await readFileAsDataUrl(file);
      if (!isAudioDataUrl(dataUrl)) {
        setTTSMessage("当前文件不是可识别的音频格式，请重新上传。");
        event.target.value = "";
        return;
      }
      setTTSVoice(dataUrl);
      setTTSVoiceFileLabel(file.name);
      const notes = [
        inspection.duration < 15 ? "时长略短，推荐 15–30 秒" : "时长合适",
        inspection.peak >= 0.995 ? "检测到可能削波，请确认没有爆音" : "音量正常"
      ];
      setTTSMessage(`参考音频已载入（${inspection.duration.toFixed(1)} 秒，${notes.join("；")}），点击“保存 TTS 配置”后生效。`);
    } catch (e) {
      console.error("read xiaomi voice clone file failed", e);
      setTTSMessage("参考音频读取失败，请换一个文件再试。");
    } finally {
      event.target.value = "";
    }
  }

  return (
    <main className="page settings-page">
      <motion.section className="settings-shell" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}>
        <div className="settings-main">
          <header className="settings-page-header">
            <span className="eyebrow"><SlidersHorizontal size={15} /> Workspace settings</span>
            <div>
              <h1>设置与服务</h1>
              <p>管理学习身份、内容模型和语音服务。每项配置独立保存，修改后会立即应用到新的学习任务。</p>
            </div>
          </header>

          <section className="settings-account-card" aria-labelledby="profile-settings-title">
            <div className="settings-section-heading">
              <div>
                <span className="settings-section-kicker">Account</span>
                <h2 id="profile-settings-title">学习身份</h2>
              </div>
              <div className="settings-account-actions">
                <span className="settings-section-hint">公开信息仅用于你的学习档案</span>
                <button type="button" className="profile-logout-button" onClick={() => void handleLogout()} disabled={loggingOut}>
                  <LogOut size={15} />
                  {loggingOut ? "退出中…" : "退出登录"}
                </button>
              </div>
            </div>
          <article className="profile-hero">
            {safeAvatarUrl ? (
              <img
                className="profile-hero-avatar"
                src={safeAvatarUrl}
                alt="头像预览"
                onError={() => setAvatarLoadError(true)}
                onLoad={() => setAvatarLoadError(false)}
              />
            ) : (
              <div className="profile-hero-avatar profile-hero-avatar-fallback">
                {(nickname.trim() || user?.email?.slice(0, 1) || "U").slice(0, 1).toUpperCase()}
              </div>
            )}
            <div className="profile-hero-meta">
              <strong>{nickname.trim() || "未设置昵称"}</strong>
              <small>当前总 XP：{user?.totalXP ?? 0}</small>
              <p>{bio.trim() || "还没有填写简介，写一句你的学习目标吧。"}</p>
            </div>
          </article>

          <form className="settings-profile-form" onSubmit={handleSubmit}>
            <div className="profile-email">
              <span className="profile-email-label"><Mail size={14} /> 邮箱</span>
              <span className="profile-email-value">{showEmail ? user?.email ?? "--" : "已隐藏"}</span>
              <button
                type="button"
                className="profile-email-toggle"
                onClick={() => setShowEmail((prev) => !prev)}
                aria-label={showEmail ? "隐藏邮箱" : "显示邮箱"}
              >
                {showEmail ? <EyeOff size={14} /> : <Eye size={14} />}
                {showEmail ? "隐藏" : "显示"}
              </button>
            </div>
            <label>
              <span><UserRound size={14} /> 昵称</span>
              <input value={nickname} onChange={(e) => setNickname(e.target.value)} />
            </label>
            <label>
              <span><BadgeCheck size={14} /> 简介</span>
              <input value={bio} onChange={(e) => setBio(e.target.value)} />
            </label>
            <label>
              <span><IdCard size={14} /> 头像 URL</span>
              <input value={avatarUrl} onChange={(e) => setAvatarUrl(e.target.value)} />
            </label>
            <button type="submit">保存资料</button>
            {profileMessage ? <p>{profileMessage}</p> : null}
            {avatarUrl.trim() && !safeAvatarUrl ? <p className="error">头像链接无效，仅支持 http/https 图片链接。</p> : null}
            {avatarLoadError && safeAvatarUrl ? <p className="error">头像加载失败，请确认图片链接可公开访问。</p> : null}
          </form>
          </section>

          {isMiniProgramEdition ? (
            <section className="settings-service-stack" aria-label="线上 AI 服务">
              <article className="stage-banner profile-section-banner settings-service-header model-service">
                <div>
                  <strong><BrainCircuit size={17} /> 线上 AI 服务</strong>
                  <p style={{ margin: "6px 0 0" }}>当前为小程序免费版，模型、TTS 和 ASR 由线上服务统一配置，你无需填写任何模型或厂商信息。</p>
                </div>
                <span className="settings-section-hint">已连接线上服务</span>
              </article>
            </section>
          ) : null}
          {!isMiniProgramEdition ? <section className="settings-service-stack" aria-label="AI 服务设置">
          <form className="settings-service-card" onSubmit={handleModelSubmit}>
            <article className="stage-banner profile-section-banner settings-service-header model-service">
              <div>
                <strong><BrainCircuit size={17} /> 内容模型</strong>
                <p style={{ margin: "6px 0 0" }}>这里修改的是当前服务端生成模型配置，保存后剧场生成、阅读分析和角色扮演会立即使用新配置。</p>
              </div>
              <button
                type="button"
                className="section-toggle"
                onClick={() => setModelSectionOpen((prev) => !prev)}
                aria-expanded={modelSectionOpen}
                aria-controls="model-management-panel"
              >
                {modelSectionOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                {modelSectionOpen ? "隐藏" : "展开"}
              </button>
            </article>
            {modelSectionOpen ? (
              <div id="model-management-panel" className="profile-section-content">
                <label>
                  <span>模型提供商</span>
                  <div className="provider-option-grid" role="radiogroup" aria-label="模型提供商">
                    {MODEL_PROVIDER_PRESETS.map((option) => {
                      const active = option.id === currentModelPreset.id;
                      return (
                        <button
                          key={option.id}
                          type="button"
                          className={active ? "provider-option-card active" : "provider-option-card"}
                          onClick={() => handleModelProviderChange(option.id)}
                          aria-pressed={active}
                        >
                          <div className="provider-option-head">
                            {option.logo ? (
                              <img className="provider-option-logo" src={option.logo} alt="" aria-hidden />
                            ) : (
                              <span className="provider-option-fallback" aria-hidden>{option.monogram ?? option.label.slice(0, 1)}</span>
                            )}
                            <strong>{option.label}</strong>
                          </div>
                          <small>{option.description}</small>
                        </button>
                      );
                    })}
                  </div>
                </label>
                <article className="profile-provider-note">
                  <div className="provider-option-head">
                    {currentModelPreset.logo ? (
                      <img className="provider-option-logo" src={currentModelPreset.logo} alt="" aria-hidden />
                    ) : (
                      <span className="provider-option-fallback" aria-hidden>{currentModelPreset.monogram ?? currentModelPreset.label.slice(0, 1)}</span>
                    )}
                    <strong>{currentModelPreset.label}</strong>
                  </div>
                  <p style={{ margin: "6px 0 0" }}>接口地址：{currentModelPreset.endpointHint}</p>
                  <p style={{ margin: "6px 0 0" }}>鉴权方式：{currentModelPreset.authHint}</p>
                  <p style={{ margin: "6px 0 0" }}>{currentModelPreset.compatibilityNote}</p>
                </article>
                <label>
                  <span>模型名称</span>
                  <input
                    list="model-provider-model-options"
                    value={modelName}
                    onChange={(e) => setModelName(e.target.value)}
                    placeholder={currentModelPreset.defaults.model || "例如 gpt-5.4"}
                  />
                </label>
                {currentModelPreset.modelOptions.length > 0 ? (
                  <datalist id="model-provider-model-options">
                    {currentModelPreset.modelOptions.map((item) => <option key={item} value={item} />)}
                  </datalist>
                ) : null}
                <label>
                  <span>Base URL</span>
                  <input
                    value={modelBaseURL}
                    onChange={(e) => setModelBaseURL(e.target.value)}
                    placeholder={currentModelPreset.defaults.baseURL || "http://43.172.5.210:3000/v1"}
                  />
                </label>
                <label>
                  <span>API Key</span>
                  <input
                    type="password"
                    value={modelAPIKey}
                    onChange={(e) => setModelAPIKey(e.target.value)}
                    placeholder={modelConfig?.hasApiKey ? "留空则保持当前 Key 不变" : "请输入新的 API Key"}
                  />
                </label>
                <button type="submit">保存模型配置</button>
                {modelMessage ? <p>{modelMessage}</p> : null}
                <p>当前 Key 状态：{modelConfig?.hasApiKey ? `已配置（${modelConfig.apiKeyPreview || "已隐藏"}）` : "未配置"}</p>
                <p>最近更新时间：{modelConfig?.updatedAt || "暂无记录"}</p>
              </div>
            ) : null}
          </form>

          <form className="settings-service-card" onSubmit={handleTTSSubmit}>
            <article className="stage-banner profile-section-banner settings-service-header tts-service">
              <div>
                <strong><AudioLines size={17} /> 语音合成</strong>
                <p style={{ margin: "6px 0 0" }}>可切换不同 TTS 厂商预设。当前预设会自动带入推荐地址、模型与音色，你也可以继续手动调整。</p>
              </div>
              <button
                type="button"
                className="section-toggle"
                onClick={() => setTTSSectionOpen((prev) => !prev)}
                aria-expanded={ttsSectionOpen}
                aria-controls="tts-management-panel"
              >
                {ttsSectionOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                {ttsSectionOpen ? "隐藏" : "展开"}
              </button>
            </article>
            {ttsSectionOpen ? (
              <div id="tts-management-panel" className="profile-section-content">
                <label>
                  <span>TTS 提供商</span>
                  <div className="provider-option-grid" role="radiogroup" aria-label="TTS 提供商">
                    {TTS_PROVIDER_PRESETS.map((option) => {
                      const active = option.id === currentTTSPreset.id;
                      return (
                        <button
                          key={option.id}
                          type="button"
                          className={active ? "provider-option-card active" : "provider-option-card"}
                          onClick={() => handleTTSProviderChange(option.id)}
                          aria-pressed={active}
                        >
                          <div className="provider-option-head">
                            {option.logo ? (
                              <img className="provider-option-logo" src={option.logo} alt="" aria-hidden />
                            ) : (
                              <span className="provider-option-fallback" aria-hidden>{option.label.slice(0, 1)}</span>
                            )}
                            <strong>{option.label}</strong>
                          </div>
                          <small>{option.description}</small>
                        </button>
                      );
                    })}
                  </div>
                </label>
                <article className="profile-provider-note">
                  <div className="provider-option-head">
                    {currentTTSPreset.logo ? (
                      <img className="provider-option-logo" src={currentTTSPreset.logo} alt="" aria-hidden />
                    ) : (
                      <span className="provider-option-fallback" aria-hidden>{currentTTSPreset.label.slice(0, 1)}</span>
                    )}
                    <strong>{currentTTSPreset.label}</strong>
                  </div>
                  <p style={{ margin: "6px 0 0" }}>接口地址：{currentTTSPreset.endpointHint}</p>
                  <p style={{ margin: "6px 0 0" }}>鉴权方式：{currentTTSPreset.authHint}</p>
                </article>
                <label>
                  <span>{currentTTSPreset.id === "XIAOMI" ? "Base URL" : "API URL"}</span>
                  <input
                    value={ttsBaseURL}
                    onChange={(e) => setTTSBaseURL(e.target.value)}
                    placeholder={currentTTSPreset.defaults.baseURL || "https://your-tts-endpoint"}
                  />
                </label>
                {currentTTSPreset.id === "XIAOMI" ? (
                  <>
                    <label>
                      <span>MiMo 模式</span>
                      <div className="provider-option-grid" role="radiogroup" aria-label="MiMo TTS 模式">
                        {XIAOMI_TTS_MODEL_PRESETS.map((option) => {
                          const active = option.id === currentXiaomiModelPreset.id;
                          return (
                            <button
                              key={option.id}
                              type="button"
                              className={active ? "provider-option-card active" : "provider-option-card"}
                              onClick={() => handleXiaomiModelChange(option.id)}
                              aria-pressed={active}
                            >
                              <div className="provider-option-head">
                                <strong>{option.label}</strong>
                              </div>
                              <small>{option.description}</small>
                            </button>
                          );
                        })}
                      </div>
                    </label>
                    <p style={{ margin: "-4px 0 0", color: "var(--ink-700)" }}>
                      当前模型：<code>{currentXiaomiModelPreset.id}</code>
                    </p>
                    {isXiaomiVoiceDesignModel(ttsModel) ? (
                      <label>
                        <span>{currentXiaomiModelPreset.voiceFieldLabel}</span>
                        <textarea
                          value={ttsVoice}
                          onChange={(e) => setTTSVoice(e.target.value)}
                          placeholder={currentXiaomiModelPreset.voiceFieldPlaceholder}
                          rows={4}
                        />
                      </label>
                    ) : null}
                    {isXiaomiVoiceCloneModel(ttsModel) ? (
                      <>
                        <label>
                          <span>{currentXiaomiModelPreset.voiceFieldLabel}</span>
                          <input type="file" accept=".mp3,.wav,audio/mpeg,audio/wav" onChange={handleXiaomiVoiceCloneUpload} />
                        </label>
                        <p style={{ margin: "-4px 0 0", color: "var(--ink-700)" }}>
                          {ttsVoiceFileLabel || (isAudioDataUrl(ttsVoice) ? "已保存参考音频样本" : "尚未上传参考音频")}。支持 MP3/WAV、10MB 以内音频；推荐单人、无音乐、15–30 秒的自然语音，保存后用于 VoiceClone。
                        </p>
                      </>
                    ) : null}
                    {!isXiaomiVoiceDesignModel(ttsModel) && !isXiaomiVoiceCloneModel(ttsModel) ? (
                      <>
                        <label>
                          <span>{currentXiaomiModelPreset.voiceFieldLabel}</span>
                          <select
                            value={isXiaomiPresetVoice(ttsVoice) ? ttsVoice : currentXiaomiModelPreset.defaultVoice}
                            onChange={(e) => setTTSVoice(e.target.value)}
                          >
                            {currentTTSPreset.voiceOptions.map((item) => (
                              <option key={item} value={item}>{item}</option>
                            ))}
                          </select>
                        </label>
                      </>
                    ) : null}
                  </>
                ) : (
                  <>
                    <label>
                      <span>模型</span>
                      <input
                        list="tts-model-options"
                        value={ttsModel}
                        onChange={(e) => setTTSModel(e.target.value)}
                        placeholder={currentTTSPreset.defaults.model || "通用接口可留空"}
                      />
                    </label>
                    {currentTTSPreset.modelOptions.length > 0 ? (
                      <datalist id="tts-model-options">
                        {currentTTSPreset.modelOptions.map((item) => <option key={item} value={item} />)}
                      </datalist>
                    ) : null}
                    <label>
                      <span>音色</span>
                      <input
                        list="tts-voice-options"
                        value={ttsVoice}
                        onChange={(e) => setTTSVoice(e.target.value)}
                        placeholder={currentTTSPreset.defaults.voice}
                      />
                    </label>
                    {currentTTSPreset.voiceOptions.length > 0 ? (
                      <datalist id="tts-voice-options">
                        {currentTTSPreset.voiceOptions.map((item) => <option key={item} value={item} />)}
                      </datalist>
                    ) : null}
                  </>
                )}
                <label>
                  <span>API Key</span>
                  <input
                    type="password"
                    value={ttsAPIKey}
                    onChange={(e) => setTTSAPIKey(e.target.value)}
                    placeholder={ttsConfig?.hasApiKey ? "留空则保持当前 Key 不变" : "请输入 TTS API Key"}
                  />
                </label>
                <button type="submit">保存 TTS 配置</button>
                {ttsMessage ? <p>{ttsMessage}</p> : null}
                <p>当前 Key 状态：{ttsConfig?.hasApiKey ? `已配置（${ttsConfig.apiKeyPreview || "已隐藏"}）` : "未配置"}</p>
                <p>当前音频格式：{ttsConfig?.audioFormat || "mp3"}</p>
                <p>最近更新时间：{ttsConfig?.updatedAt || "暂无记录"}</p>
              </div>
            ) : null}
          </form>

          <form className="settings-service-card" onSubmit={handleASRSubmit}>
            <article className="stage-banner profile-section-banner settings-service-header asr-service">
              <div><strong>ASR 语音识别管理</strong><p style={{ margin: "6px 0 0" }}>用于剧场库语音练习：录音会先转为 WAV，再进入后台识别、AI 续聊和 TTS 语音回复。</p></div>
              <button type="button" className="section-toggle" onClick={() => setASRSectionOpen((prev) => !prev)} aria-expanded={asrSectionOpen}>{asrSectionOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}{asrSectionOpen ? "隐藏" : "展开"}</button>
            </article>
            {asrSectionOpen ? <div className="profile-section-content asr-config-grid">
              <label>
                <span>ASR 接入方式</span>
                <select value={asrProvider} onChange={(event) => handleASRProviderChange(event.target.value as ASRProviderId)}>
                  {ASR_PROVIDER_PRESETS.map((preset) => <option key={preset.id} value={preset.id}>{preset.label}</option>)}
                </select>
              </label>
              <p className="profile-provider-note asr-provider-description">{currentASRPreset.description}</p>
              <label>
                <span>模型</span>
                <input value={asrModel} onChange={(event) => setASRModel(event.target.value)} placeholder={currentASRPreset.defaults.model} />
              </label>
              <label>
                <span>Base URL</span>
                <input value={asrBaseURL} onChange={(event) => setASRBaseURL(event.target.value)} placeholder={currentASRPreset.defaults.baseURL} />
              </label>
              {currentASRPreset.requiresAppID ? <label>
                <span>豆包 App ID</span>
                <input value={asrAppID} onChange={(event) => setASRAppID(event.target.value)} placeholder="请输入火山引擎 App ID" required />
              </label> : null}
              <label>
                <span>{asrProvider === "DOUBAO" ? "Access Token" : "API Key"}</span>
                <input type="password" value={asrAPIKey} onChange={(event) => setASRAPIKey(event.target.value)} placeholder={hasASRKeyForSelectedProvider ? "留空则保持当前 Key 不变" : asrProvider === "DOUBAO" ? "请输入火山引擎 Access Token" : "请输入 ASR API Key"} />
              </label>
              <div className="asr-config-footer">
                <button type="submit">保存 ASR 配置</button>
                {asrMessage ? <p>{asrMessage}</p> : null}
                <small>当前 Key：{hasASRKeyForSelectedProvider ? `已配置（${asrConfig?.apiKeyPreview || "已隐藏"}）` : "未配置"}</small>
              </div>
            </div> : null}
          </form>
          </section> : null}

			{isCommercialEdition && !isMiniProgramEdition ? <article className="stage-banner profile-voice-library-link settings-voice-library-card membership-profile-link">
				<div><strong><Crown size={16} /> 会员与 AI 点数</strong><p style={{ margin: "6px 0 0" }}>免费用户每日 20 点；开通会员可去广告并提升生成、评分和语音服务额度。</p></div>
				<button type="button" onClick={() => navigate("/membership")}>查看会员</button>
			</article> : null}

			<article className="stage-banner profile-voice-library-link settings-voice-library-card">
				<div><strong><Waves size={16} /> 角色音色库</strong><p style={{ margin: "6px 0 0" }}>在独立页面创建、试听、筛选和管理角色声线；完成后可分配给剧场角色。</p></div>
				<button type="button" onClick={() => navigate("/voices")}>打开音色库</button>
			</article>

			<article className="stage-banner profile-voice-library-link settings-voice-library-card release-notes-profile-link">
				<div><strong><History size={16} /> 产品更新日志</strong><p style={{ margin: "6px 0 0" }}>当前 V1.0.1：查看已上线功能和之后每一次的版本更新。</p></div>
				<button type="button" onClick={() => navigate("/updates")}>查看更新</button>
			</article>
        </div>

        <aside className="settings-side-panel">
          <span className="eyebrow"><Waves size={15} /> Learning space</span>
          <h3>成长轨迹</h3>
          <p>在此维护学习身份和服务配置，让每一次练习更贴近你的目标。</p>
          <div className="mini-progress" aria-hidden>
            <span style={{ width: `${Math.min(100, Math.max(8, (user?.totalXP ?? 0) / 10))}%` }} />
          </div>
			<p>{user?.rankLabel ?? "初学探索者"} · Lv.{user?.level ?? 1}</p>
			<p>当前总 XP：{user?.totalXP ?? 0}</p>
        </aside>
      </motion.section>
    </main>
  );
}
