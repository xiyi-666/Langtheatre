export type ASRProviderId = "XIAOMI" | "ALIYUN" | "DOUBAO" | "GEMINI" | "MINIMAX" | "OPENAI" | "OPENAI_COMPATIBLE";

export interface ASRProviderPreset {
  id: ASRProviderId;
  label: string;
  description: string;
  defaults: {
    baseURL: string;
    model: string;
  };
  requiresAppID?: boolean;
}

export const ASR_PROVIDER_PRESETS: ASRProviderPreset[] = [
  {
    id: "XIAOMI",
    label: "小米 MiMo ASR",
    description: "专用语音识别，支持中文、英文与自动识别。",
    defaults: { baseURL: "https://api.xiaomimimo.com/v1", model: "mimo-v2.5-asr" }
  },
  {
    id: "ALIYUN",
    label: "阿里云 DashScope ASR",
    description: "通过 DashScope 的 Qwen3-ASR OpenAI 兼容接口转写短音频。",
    defaults: { baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", model: "qwen3-asr-flash" }
  },
  {
    id: "DOUBAO",
    label: "豆包大模型 ASR",
    description: "火山引擎大模型语音识别，需要 App ID 和 Access Token。",
    defaults: { baseURL: "https://openspeech.bytedance.com/api/v3/auc/bigmodel/recognize/flash", model: "bigmodel" },
    requiresAppID: true
  },
  {
    id: "GEMINI",
    label: "Gemini 音频转写",
    description: "使用 Gemini 的内联音频理解能力完成短录音转写。",
    defaults: { baseURL: "https://generativelanguage.googleapis.com/v1beta", model: "gemini-2.5-flash" }
  },
  {
    id: "MINIMAX",
    label: "MiniMax Transcriptions",
    description: "使用 MiniMax 兼容的 Audio Transcriptions 接口。",
    defaults: { baseURL: "https://api.minimax.io/v1", model: "speech-2.8-hd" }
  },
  {
    id: "OPENAI",
    label: "OpenAI Transcriptions",
    description: "使用 OpenAI 标准 multipart 转写协议。",
    defaults: { baseURL: "https://api.openai.com/v1", model: "gpt-4o-mini-transcribe" }
  },
  {
    id: "OPENAI_COMPATIBLE",
    label: "OpenAI 兼容接口",
    description: "适用于实现 /audio/transcriptions 的自建或第三方网关。",
    defaults: { baseURL: "https://api.openai.com/v1", model: "gpt-4o-mini-transcribe" }
  }
];

export function getASRProviderPreset(provider: string): ASRProviderPreset {
  return ASR_PROVIDER_PRESETS.find((item) => item.id === provider) ?? ASR_PROVIDER_PRESETS[ASR_PROVIDER_PRESETS.length - 1];
}
