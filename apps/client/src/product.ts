export type ReleaseNote = {
  version: string;
  releasedOn: string;
  title: string;
  summary: string;
  highlights: string[];
};

// 面向用户展示的版本功能介绍；这里只保留已对外说明的两个产品版本。
export const currentProductVersion = "1.0.1";

export const releaseNotes: ReleaseNote[] = [
  {
    version: "1.0.1",
    releasedOn: "2026-09-30",
    title: "专项训练与模拟考试升级",
    summary: "围绕真实学习场景补充听力、口语和模拟考试，让英语训练从单项练习延伸到完整的能力检验。",
    highlights: [
      "新增 IELTS 听力专项训练，覆盖对话、学术讨论和学术讲座，并提供配套音频与题目练习。",
      "新增 AI 考官口语模拟，支持分阶段回答、会话恢复和针对性的表达改进建议。",
      "新增 IELTS、CET-4、CET-6 模拟考试，可重复进入练习、提交答案并查看成绩反馈。",
      "阅读、听力和写作材料支持生成进度、历史记录、筛选和重复训练，学习过程更容易连续进行。",
      "优化粤语剧场的双人对话、角色音色、语速控制和语音播放体验。"
    ]
  },
  {
    version: "1.0.0",
    releasedOn: "2026-08-02",
    title: "LinguaQuest 学习空间上线",
    summary: "以英语和粤语真实表达为核心，提供从材料学习、语音练习到写作反馈的一体化学习体验。",
    highlights: [
      "提供英语与粤语双路线学习，支持 AI 小剧场、阅读材料和随文语音练习。",
      "剧场角色扮演支持中文、粤语和英语语音沟通，可结合语音识别、AI 对话与语音回复进行练习。",
      "英语写作支持 IELTS、CET-4、CET-6 主题、限时写作、多维评分和具体修改建议。",
      "角色音色库支持创建、试听和管理角色声线，并将音色分配到剧场角色。",
      "支持用户名或邮箱登录、邮箱验证、密码重置和用户名找回，让学习记录更安全地保存。"
    ]
  }
];
