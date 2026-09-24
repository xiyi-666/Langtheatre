const cantoneseAudio = [
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/68528b9f35440c9bd95637850f839e0acc02b76d38d10e0ff665d67a2d3916c7.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/cf7f1d4683105bd0861acaad2cc7f9664fcf10c8be713a13e6e883946aba17c9.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/cf4091d8d8dcb4e6f4d38f67049f60a37f4bc2173d546e615eed0fe31a953a4b.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/b88fef3303aeee253aecbc0641503962d6bd18cd74b9f84744e7ab9a43bb7377.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/52d8c6fe3d53fc131f40769221695ce8751bea42366b07c24964897a84400cfa.mp3",
  "/media/tts/theater/efc3cd91-e319-4939-97d7-5f9a93efa454/4ae908c2cb2e1e50f9424e604c2da19ef78be04637d48cf4d81f3fdf9e18ac3b.mp3"
] as const;

// 这个账号只用于演示入口，所有演示结果均来自前端 fixture。
export const demoAccount = { username: "lingua_demo_0903", password: "LqDemo2026!" } as const;

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
