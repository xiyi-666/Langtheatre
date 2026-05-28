import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
import { motion } from "framer-motion";
import { BadgeCheck, ChevronDown, ChevronUp, Eye, EyeOff, IdCard, Mail, UserRound } from "lucide-react";
import { getModelConfig, getTTSConfig, me, updateModelConfig, updateProfile, updateTTSConfig } from "../api";
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
import type { ModelConfig, TTSConfig } from "../types";

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("read file failed"));
    reader.onload = () => resolve(typeof reader.result === "string" ? reader.result : "");
    reader.readAsDataURL(file);
  });
}

export function ProfilePage() {
  const user = useAppStore((s) => s.user);
  const setUser = useAppStore((s) => s.setUser);
  const [nickname, setNickname] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [bio, setBio] = useState("");
  const [profileMessage, setProfileMessage] = useState("");
  const [modelMessage, setModelMessage] = useState("");
  const [ttsMessage, setTTSMessage] = useState("");
  const [avatarLoadError, setAvatarLoadError] = useState(false);
  const [showEmail, setShowEmail] = useState(false);
  const [modelConfig, setModelConfig] = useState<ModelConfig>();
  const [modelSectionOpen, setModelSectionOpen] = useState(true);
  const [modelProvider, setModelProvider] = useState<ModelProviderId>("OPENAI");
  const [modelName, setModelName] = useState("");
  const [modelBaseURL, setModelBaseURL] = useState("");
  const [modelAPIKey, setModelAPIKey] = useState("");
  const [ttsConfig, setTTSConfig] = useState<TTSConfig>();
  const [ttsSectionOpen, setTTSSectionOpen] = useState(true);
  const [ttsProvider, setTTSProvider] = useState<TTSProviderId>("XIAOMI");
  const [ttsModel, setTTSModel] = useState("");
  const [ttsBaseURL, setTTSBaseURL] = useState("");
  const [ttsVoice, setTTSVoice] = useState("");
  const [ttsAPIKey, setTTSAPIKey] = useState("");
  const [ttsVoiceFileLabel, setTTSVoiceFileLabel] = useState("");
  const currentModelPreset = useMemo(() => getModelProviderPreset(modelProvider), [modelProvider]);
  const currentTTSPreset = useMemo(() => getTTSProviderPreset(ttsProvider), [ttsProvider]);
  const currentXiaomiModelPreset = useMemo(() => getXiaomiTTSModelPreset(ttsModel), [ttsModel]);

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
        const [profile, config, tts] = await Promise.all([me(), getModelConfig(), getTTSConfig()]);
        setUser(profile);
        setNickname(profile.nickname ?? "");
        setAvatarUrl(profile.avatarUrl ?? "");
        setBio(profile.bio ?? "");
        setModelConfig(config);
        setModelProvider(getModelProviderPreset(config.provider).id);
        setModelName(config.model ?? "");
        setModelBaseURL(config.baseURL ?? "");
        applyTTSConfig(tts);
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
    try {
      const dataUrl = await readFileAsDataUrl(file);
      if (!isAudioDataUrl(dataUrl)) {
        setTTSMessage("当前文件不是可识别的音频格式，请重新上传。");
        event.target.value = "";
        return;
      }
      setTTSVoice(dataUrl);
      setTTSVoiceFileLabel(file.name);
      setTTSMessage("参考音频已载入，点击“保存 TTS 配置”后生效。");
    } catch (e) {
      console.error("read xiaomi voice clone file failed", e);
      setTTSMessage("参考音频读取失败，请换一个文件再试。");
    } finally {
      event.target.value = "";
    }
  }

  return (
    <main className="page-center">
      <motion.section className="card auth-shell" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}>
        <div className="auth-main">
          <h2>个人中心</h2>
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

          <form onSubmit={handleSubmit}>
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

          <form onSubmit={handleModelSubmit} style={{ marginTop: 18 }}>
            <article className="stage-banner profile-section-banner" style={{ marginBottom: 12 }}>
              <div>
                <strong>模型管理</strong>
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

          <form onSubmit={handleTTSSubmit} style={{ marginTop: 18 }}>
            <article className="stage-banner profile-section-banner" style={{ marginBottom: 12 }}>
              <div>
                <strong>TTS 管理</strong>
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
                          <input type="file" accept="audio/*" onChange={handleXiaomiVoiceCloneUpload} />
                        </label>
                        <p style={{ margin: "-4px 0 0", color: "var(--ink-700)" }}>
                          {ttsVoiceFileLabel || (isAudioDataUrl(ttsVoice) ? "已保存参考音频样本" : "尚未上传参考音频")}。支持 10MB 以内音频样本，保存后用于 VoiceClone。
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
                        <p style={{ margin: "-4px 0 0", color: "var(--ink-700)" }}>
                          选择 <code>mimo_default</code> 时，中国集群默认是冰糖，其他集群默认是 Mia。
                        </p>
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
                <p>当前音频格式：{ttsConfig?.audioFormat || "wav"}</p>
                <p>最近更新时间：{ttsConfig?.updatedAt || "暂无记录"}</p>
              </div>
            ) : null}
          </form>
        </div>

        <aside className="floating-panel auth-side">
          <h3>成长轨迹</h3>
          <p>你可以在这里维护学习身份信息，便于复练与分享时展示。</p>
          <div className="mini-progress" aria-hidden>
            <span style={{ width: `${Math.min(100, Math.max(8, (user?.totalXP ?? 0) / 10))}%` }} />
          </div>
          <p>当前总 XP：{user?.totalXP ?? 0}</p>
        </aside>
      </motion.section>
    </main>
  );
}
