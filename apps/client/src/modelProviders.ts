export type ModelProviderId =
  | "OPENAI_COMPATIBLE"
  | "OPENAI"
  | "CLAUDE"
  | "GEMINI"
  | "GLM"
  | "MINIMAX"
  | "DEEPSEEK"
  | "DOUBAO"
  | "QWEN";

export type ModelProviderPreset = {
  id: ModelProviderId;
  label: string;
  description: string;
  logo: string;
  monogram?: string;
  defaults: {
    baseURL: string;
    model: string;
  };
  modelOptions: string[];
  endpointHint: string;
  authHint: string;
  compatibilityNote: string;
};

export const MODEL_PROVIDER_PRESETS: ModelProviderPreset[] = [
  {
    id: "OPENAI_COMPATIBLE",
    label: "OpenAI Compatible",
    description: "自定义兼容 OpenAI Chat Completions 的网关",
    logo: "/model-provider-logos/openai.png",
    monogram: "OC",
    defaults: {
      baseURL: "http://43.172.5.210:3000/v1",
      model: "gpt-5.4"
    },
    modelOptions: [],
    endpointHint: "自定义兼容端点，根域名会自动补成 /v1/chat/completions",
    authHint: "默认使用 Authorization: Bearer，并附带 x-api-key",
    compatibilityNote: "适合自建网关或第三方 OpenAI Compatible 服务。"
  },
  {
    id: "OPENAI",
    label: "OpenAI",
    description: "官方 OpenAI API",
    logo: "/model-provider-logos/openai.png",
    defaults: {
      baseURL: "http://43.172.5.210:3000/v1",
      model: "gpt-5.4"
    },
    modelOptions: ["gpt-5.4", "gpt-4.1-mini", "gpt-4o-mini"],
    endpointHint: "https://api.openai.com/v1/chat/completions",
    authHint: "Authorization: Bearer OPENAI_API_KEY",
    compatibilityNote: "这里展示的是 OpenAI 官方接口说明；默认 Base URL 仍可使用你当前配置的代理网关。"
  },
  {
    id: "CLAUDE",
    label: "Claude",
    description: "Anthropic OpenAI SDK compatibility",
    logo: "/model-provider-logos/claude.ico",
    defaults: {
      baseURL: "https://api.anthropic.com/v1",
      model: "claude-sonnet-4-6"
    },
    modelOptions: ["claude-sonnet-4-6", "claude-opus-4-7", "claude-haiku-4-5"],
    endpointHint: "https://api.anthropic.com/v1/chat/completions",
    authHint: "Authorization: Bearer ANTHROPIC_API_KEY",
    compatibilityNote: "使用 Anthropic 官方 OpenAI 兼容层，不是原生 Messages API。"
  },
  {
    id: "GEMINI",
    label: "Gemini",
    description: "Google Gemini OpenAI compatibility",
    logo: "/model-provider-logos/gemini.png",
    defaults: {
      baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
      model: "gemini-2.5-flash"
    },
    modelOptions: ["gemini-2.5-flash", "gemini-2.5-pro"],
    endpointHint: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
    authHint: "Authorization: Bearer GEMINI_API_KEY",
    compatibilityNote: "Google 官方兼容层，推荐使用 Gemini 兼容模型名。"
  },
  {
    id: "GLM",
    label: "GLM",
    description: "智谱 GLM 官方对话补全接口",
    logo: "/model-provider-logos/glm.png",
    defaults: {
      baseURL: "https://open.bigmodel.cn/api/paas/v4",
      model: "glm-5.1"
    },
    modelOptions: ["glm-5.1", "glm-4.5-air"],
    endpointHint: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
    authHint: "Authorization: Bearer ZHIPU_API_KEY",
    compatibilityNote: "使用智谱官方 Chat Completions 接口。"
  },
  {
    id: "MINIMAX",
    label: "MiniMax",
    description: "MiniMax Compatible OpenAI API",
    logo: "/model-provider-logos/minimax.svg",
    defaults: {
      baseURL: "https://api.minimax.io/v1",
      model: "MiniMax-M2.7"
    },
    modelOptions: ["MiniMax-M2.7", "MiniMax-M2.5"],
    endpointHint: "https://api.minimax.io/v1/chat/completions",
    authHint: "Authorization: Bearer MINIMAX_API_KEY",
    compatibilityNote: "使用 MiniMax 官方兼容 OpenAI 接口。"
  },
  {
    id: "DEEPSEEK",
    label: "DeepSeek",
    description: "DeepSeek 官方 Chat Completions",
    logo: "/model-provider-logos/deepseek.png",
    defaults: {
      baseURL: "https://api.deepseek.com",
      model: "deepseek-v4-flash"
    },
    modelOptions: ["deepseek-v4-flash", "deepseek-v4-pro", "deepseek-chat"],
    endpointHint: "https://api.deepseek.com/v1/chat/completions",
    authHint: "Authorization: Bearer DEEPSEEK_API_KEY",
    compatibilityNote: "默认使用 Chat Completions；如需测试功能，可手动将 Base URL 改为 /beta。"
  },
  {
    id: "DOUBAO",
    label: "DouBao",
    description: "火山方舟对话 Chat API",
    logo: "/model-provider-logos/doubao.svg",
    defaults: {
      baseURL: "https://ark.cn-beijing.volces.com/api/v3",
      model: "doubao-seed-2-0-lite-260428"
    },
    modelOptions: ["doubao-seed-2-0-lite-260428"],
    endpointHint: "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
    authHint: "Authorization: Bearer ARK_API_KEY",
    compatibilityNote: "默认使用火山方舟北京地域标准 Chat 接口。"
  },
  {
    id: "QWEN",
    label: "Qwen",
    description: "阿里云百炼 OpenAI 兼容接口",
    logo: "/model-provider-logos/qwen.apng",
    defaults: {
      baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
      model: "qwen3.6-plus"
    },
    modelOptions: ["qwen3.6-plus", "qwen-plus"],
    endpointHint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
    authHint: "Authorization: Bearer DASHSCOPE_API_KEY",
    compatibilityNote: "默认使用阿里云百炼北京地域兼容端点。"
  }
];

const fallbackPreset = MODEL_PROVIDER_PRESETS[0];

export function getModelProviderPreset(provider: string): ModelProviderPreset {
  const normalized = provider.trim().toUpperCase();
  return MODEL_PROVIDER_PRESETS.find((item) => item.id === normalized) ?? fallbackPreset;
}
