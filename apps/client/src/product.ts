export type ReleaseNote = {
  version: string;
  releasedOn: string;
  title: string;
  summary: string;
  highlights: string[];
};

// 发布新版本时只需在这里置顶添加一条记录，并同步更新 package.json 的版本号。
export const currentProductVersion = "1.0.1";

export const releaseNotes: ReleaseNote[] = [
  {
    version: "1.0.1",
    releasedOn: "2026-08-02",
    title: "内测数据优化基础",
    summary: "新增仅后端可见的匿名日聚合统计，用于持续优化学习体验和服务稳定性。",
    highlights: [
      "记录模型 Token、核心功能使用与导航点击的按日汇总数据。",
      "不保存提示词、文章、音频、用户身份或原始点击流水。",
      "统计不会出现在产品页面，只有配置独立密钥的运维接口可以读取。"
    ]
  },
  {
    version: "1.0.0",
    releasedOn: "2026-08-02",
    title: "LinguaQuest 产品处于内测",
    summary: "围绕粤语与英语的真实表达训练：从生成材料、语音练习到写作反馈，形成一条完整学习路径。",
    highlights: [
      "英粤双路线课程、AI 小剧场、阅读材料与随文语音练习。",
      "剧场角色扮演支持中文、粤语和英语语音输入；可接入 ASR、AI 续聊与 TTS 回复。",
      "英语写作支持 IELTS、CET-4、CET-6 主题生成、限时写作和多维 AI 评分建议。",
      "角色音色库支持按提示词设计、试听和分配角色声线。",
      "账号支持用户名或邮箱登录、邮箱验证，以及密码与用户名找回。",
      "产品处于内测：小程序版可免费使用，每日前几次免广告，后续通过广告支持服务持续运行。"
    ]
  }
];
