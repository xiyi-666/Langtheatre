export type TTSProviderId = "CUSTOM" | "XIAOMI" | "MINIMAX" | "ALIYUN";
export type XiaomiTTSModelId = "mimo-v2.5-tts" | "mimo-v2.5-tts-voicedesign" | "mimo-v2.5-tts-voiceclone";

export type TTSProviderPreset = {
  id: TTSProviderId;
  label: string;
  description: string;
  logo: string;
  defaults: {
    baseURL: string;
    model: string;
    voice: string;
  };
  modelOptions: string[];
  voiceOptions: string[];
  endpointHint: string;
  authHint: string;
};

export type XiaomiTTSModelPreset = {
  id: XiaomiTTSModelId;
  label: string;
  description: string;
  defaultVoice: string;
  voiceFieldLabel: string;
  voiceFieldPlaceholder: string;
};

const xiaomiPresetVoiceOptions = ["mimo_default", "冰糖", "茉莉", "苏打", "白桦", "Mia", "Chloe", "Milo", "Dean"] as const;

export const XIAOMI_TTS_MODEL_PRESETS: XiaomiTTSModelPreset[] = [
  {
    id: "mimo-v2.5-tts",
    label: "预制音色",
    description: "直接选择 MiMo 官方预设音色，适合稳定批量生成。",
    defaultVoice: "mimo_default",
    voiceFieldLabel: "音色",
    voiceFieldPlaceholder: "请选择官方预设音色"
  },
  {
    id: "mimo-v2.5-tts-voicedesign",
    label: "自动生成音色",
    description: "用文字描述音色特征，由 VoiceDesign 自动生成专属音色。",
    defaultVoice: "20 多岁女性，声音亲和自然，吐字清晰，适合粤语和英文学习内容。",
    voiceFieldLabel: "音色描述",
    voiceFieldPlaceholder: "例如：20 多岁女性，温柔自然，适合粤语教学与英文例句朗读。"
  },
  {
    id: "mimo-v2.5-tts-voiceclone",
    label: "参考音频克隆",
    description: "上传一段参考音频，使用 VoiceClone 复刻相近音色。",
    defaultVoice: "",
    voiceFieldLabel: "参考音频",
    voiceFieldPlaceholder: "请上传 10MB 以内的音频样本"
  }
];

export const TTS_PROVIDER_PRESETS: TTSProviderPreset[] = [
  {
    id: "CUSTOM",
    label: "通用接口",
    description: "兼容现有自定义 TTS 接口",
    logo: "/model-provider-logos/openai.png",
    defaults: {
      baseURL: "",
      model: "",
      voice: "female-1"
    },
    modelOptions: [],
    voiceOptions: ["female-1"],
    endpointHint: "自定义 HTTP TTS 接口",
    authHint: "兼容现有 Bearer / x-api-key 逻辑"
  },
  {
    id: "XIAOMI",
    label: "XiaoMi MiMo",
    description: "MiMo TTS，chat/completions 兼容接口",
    logo: "/provider-logos/xiaomi.png",
    defaults: {
      baseURL: "https://api.xiaomimimo.com/v1",
      model: "mimo-v2.5-tts",
      voice: "mimo_default"
    },
    modelOptions: XIAOMI_TTS_MODEL_PRESETS.map((item) => item.id),
    voiceOptions: [...xiaomiPresetVoiceOptions],
    endpointHint: "https://api.xiaomimimo.com/v1",
    authHint: "Header: api-key"
  },
  {
    id: "MINIMAX",
    label: "MiniMax",
    description: "官方 HTTP T2A，同步返回音频数据",
    logo: "/provider-logos/minimax.svg",
    defaults: {
      baseURL: "https://api.minimax.io/v1/t2a_v2",
      model: "speech-2.8-hd",
      voice: "Cantonese_GentleLady"
    },
    modelOptions: ["speech-2.8-hd", "speech-2.8-turbo", "speech-02-hd", "speech-02-turbo"],
    voiceOptions: ["Cantonese_GentleLady", "Cantonese_KindWoman", "English_expressive_narrator"],
    endpointHint: "https://api.minimax.io/v1/t2a_v2",
    authHint: "Header: Authorization Bearer"
  },
  {
    id: "ALIYUN",
    label: "Aliyun",
    description: "百炼 CosyVoice HTTP 非流式合成",
    logo: "/model-provider-logos/qwen.apng",
    defaults: {
      baseURL: "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/SpeechSynthesizer",
      model: "cosyvoice-v3-flash",
      voice: "longjiaxin_v3"
    },
    modelOptions: ["cosyvoice-v3.5-flash", "cosyvoice-v3-flash", "cosyvoice-v2"],
    voiceOptions: ["longjiaxin_v3", "longanyang", "longxiaochun_v2"],
    endpointHint: "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/SpeechSynthesizer",
    authHint: "Header: Authorization Bearer"
  }
];

const fallbackPreset = TTS_PROVIDER_PRESETS[0];

export function getTTSProviderPreset(provider: string): TTSProviderPreset {
  const normalized = provider.trim().toUpperCase();
  return TTS_PROVIDER_PRESETS.find((item) => item.id === normalized) ?? fallbackPreset;
}

const fallbackXiaomiModelPreset = XIAOMI_TTS_MODEL_PRESETS[0];

export function getXiaomiTTSModelPreset(model: string): XiaomiTTSModelPreset {
  const normalized = model.trim().toLowerCase();
  return XIAOMI_TTS_MODEL_PRESETS.find((item) => item.id.toLowerCase() === normalized) ?? fallbackXiaomiModelPreset;
}

export function isXiaomiVoiceDesignModel(model: string): boolean {
  return getXiaomiTTSModelPreset(model).id === "mimo-v2.5-tts-voicedesign";
}

export function isXiaomiVoiceCloneModel(model: string): boolean {
  return getXiaomiTTSModelPreset(model).id === "mimo-v2.5-tts-voiceclone";
}

export function isXiaomiPresetVoice(voice: string): boolean {
  return xiaomiPresetVoiceOptions.includes(voice.trim() as (typeof xiaomiPresetVoiceOptions)[number]);
}

export function isAudioDataUrl(value: string): boolean {
  return value.trim().toLowerCase().startsWith("data:audio/");
}

export function shouldSwapToProviderDefault(currentValue: string, previousDefault: string): boolean {
  const trimmed = currentValue.trim();
  return trimmed === "" || trimmed === previousDefault.trim();
}
