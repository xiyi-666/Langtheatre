import { expect, test } from "@playwright/test";

test("listening specialty: background, answers, replay, report, mobile and deletion", async ({ page }, testInfo) => {
  const state = { status: "GENERATING", exists: false, answers: Array(10).fill(""), saved: 0, finished: 0, slowSave: false };
  const keys = ["library", ...Array(9).fill("B")];
  const questions = keys.map((answerKey, i) => ({ question: i === 0 ? "Complete the sentence: Meet at the ____. NO MORE THAN ONE WORD." : `Question ${i + 1}. Where should visitors go next?`, type: i === 0 ? "sentence_completion" : "multiple_choice", options: i === 0 ? [] : ["A. Station", "B. Library", "C. Museum", "D. Park"], answerKey, evidence: "Visitors should meet at the library.", correct: state.answers[i] === answerKey }));
  const record = (summary = false) => ({ id: "listening-fixture", part: 2, title: "Library visitor briefing", targetBand: 7, status: state.status, message: "正在生成听力音频", createdAt: "2026-09-15T00:00:00Z", estimatedReadySeconds: 0, generationEstimateSamples: 0, instructions: "Listen and answer the questions.", answers: state.answers, audioUrls: !summary && ["IN_PROGRESS", "COMPLETED"].includes(state.status) ? ["/fixture-audio-1.wav", "/fixture-audio-2.wav"] : [], questions: !summary && ["IN_PROGRESS", "COMPLETED"].includes(state.status) ? questions.map((q, i) => ({ ...q, correct: state.status === "COMPLETED" ? state.answers[i] === keys[i] : null, answerKey: state.status === "COMPLETED" ? q.answerKey : null, evidence: state.status === "COMPLETED" ? q.evidence : null })) : [], transcript: state.status === "COMPLETED" ? "Mia: Visitors should meet at the library." : null, correct: state.status === "COMPLETED" ? 2 : null, total: 10, accuracy: state.status === "COMPLETED" ? 20 : null, feedback: "单段训练显示正确率，不换算雅思 Band。", recommendations: ["选择题需要关注信息替换。"] });
  await page.addInitScript(() => { localStorage.setItem("accessToken", "test-only"); HTMLMediaElement.prototype.play = function () { window.__listeningPlays = (window.__listeningPlays || 0) + 1; return Promise.resolve(); }; });
  await page.route("**/telemetry/**", route => route.fulfill({ status: 204 }));
  const audioFixture = Buffer.alloc(44 + 1600);
  audioFixture.write("RIFF", 0); audioFixture.writeUInt32LE(audioFixture.length - 8, 4);
  audioFixture.write("WAVEfmt ", 8); audioFixture.writeUInt32LE(16, 16);
  audioFixture.writeUInt16LE(1, 20); audioFixture.writeUInt16LE(1, 22);
  audioFixture.writeUInt32LE(8000, 24); audioFixture.writeUInt32LE(16000, 28);
  audioFixture.writeUInt16LE(2, 32); audioFixture.writeUInt16LE(16, 34);
  audioFixture.write("data", 36); audioFixture.writeUInt32LE(1600, 40);
  await page.route("**/fixture-audio-*.wav", route => route.fulfill({ status: 200, contentType: "audio/wav", body: audioFixture }));
  await page.route("**/graphql", async route => {
    const { query, variables } = route.request().postDataJSON(); let data = {};
    if (query.includes("startListeningTraining(")) { expect(variables).toEqual({ part: 2, targetBand: 7 }); state.exists = true; data = { startListeningTraining: record() }; }
    else if (query.includes("beginListeningTraining(")) { state.status = "IN_PROGRESS"; data = { beginListeningTraining: record() }; }
    else if (query.includes("saveListeningAnswers(")) { state.saved++; if (state.slowSave) { state.slowSave = false; await new Promise(resolve => setTimeout(resolve, 1200)); } state.answers = variables.answers; data = { saveListeningAnswers: record() }; }
    else if (query.includes("finishListeningTraining(")) { state.finished++; state.answers = variables.answers; expect(state.answers[0]).toBe("library"); state.status = "COMPLETED"; data = { finishListeningTraining: record() }; }
    else if (query.includes("deleteListeningTraining(")) { state.exists = false; data = { deleteListeningTraining: true }; }
    else if (query.includes("listeningTrainingGenerationCost")) data = { listeningTrainingGenerationCost: 10 };
    else if (query.includes("listeningTrainings")) data = { listeningTrainings: state.exists ? [record(true)] : [] };
    else if (query.includes("listeningTraining(")) data = { listeningTraining: record() };
    else if (query.includes("me {")) data = { me: { id: "qa", username: "qa", totalXP: 18 } };
    await route.fulfill({ json: { data } });
  });
  const runtimeErrors = []; page.on("pageerror", e => runtimeErrors.push(e.message));
  page.on("dialog", dialog => dialog.accept());
  await page.goto("/listening");
  await expect(page.getByRole("heading", { name: "雅思听力", exact: true })).toBeVisible();
  await expect(page.getByText("本次生成消耗 10 AI 点数；失败自动退回")).toBeVisible();
  await page.getByRole("button", { name: /PART 02/ }).click();
  await page.getByRole("combobox", { name: "目标难度", exact: true }).selectOption("7");
  await page.getByRole("button", { name: "生成听力训练", exact: true }).click();
  await expect(page.getByText("正在准备你的听力训练")).toBeVisible();
  await page.getByRole("link", { name: "返回训练列表", exact: true }).click();
  await expect(page.getByText("后台生成中", { exact: true })).toBeVisible();
  state.status = "READY";
  await page.getByRole("button", { name: "刷新列表" }).click();
  await page.getByRole("link", { name: "进入训练" }).click();
  await expect(page.locator("audio")).toHaveCount(0);
  await page.getByRole("button", { name: "开始听力训练" }).click();
  await expect(page.getByLabel("第 1 题答案")).toBeVisible();
  expect(await page.evaluate(() => window.__listeningPlays || 0)).toBe(0);
  await page.locator("audio").evaluate(player => player.load());
  await expect.poll(() => page.locator("audio").evaluate(player => player.duration)).toBeCloseTo(0.1, 2);
  await page.getByLabel("播放速度", { exact: true }).selectOption("1.5");
  await expect.poll(() => page.locator("audio").evaluate(a => a.playbackRate)).toBe(1.5);
  await page.locator("audio").dispatchEvent("ended");
  await expect(page.locator("audio")).toHaveAttribute("src", /fixture-audio-2.wav/);
  await expect.poll(() => page.evaluate(() => window.__listeningPlays || 0)).toBe(1);
  await expect(page.getByText("查看完整听力原文")).toHaveCount(0);
  state.slowSave = true;
  await page.getByLabel("第 1 题答案").fill("draft");
  await expect.poll(() => state.saved).toBeGreaterThan(0);
  await page.getByLabel("第 1 题答案").fill("library");
  await page.locator("#listening-question-1").getByRole("radio", { name: "B. Library" }).check();
  await expect.poll(() => state.answers[0]).toBe("library");
  await expect(page.getByLabel("第 1 题答案")).toHaveValue("library");
  await page.reload();
  await expect(page.getByLabel("第 1 题答案")).toHaveValue("library");
  await page.getByRole("button", { name: "提交听力答案" }).click();
  await expect(page.getByRole("region", { name: "听力训练结果" })).toContainText("20");
  expect(state.finished).toBe(1);
  await page.getByLabel("只看错题与未答题").check();
  await expect(page.locator(".listening-question")).toHaveCount(8);
  await page.getByText("查看完整听力原文", { exact: true }).click();
  await expect(page.locator(".listening-transcript")).toContainText("Mia: Visitors should meet at the library.");
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("listening-mobile-report.png"), fullPage: true });
  await page.getByRole("link", { name: "听力训练列表", exact: true }).click();
  await expect(page.getByText("正确率 20%", { exact: false })).toBeVisible();
  await page.getByRole("button", { name: /删除听力训练/ }).click();
  await expect(page.getByRole("heading", { name: "从一段听力开始" })).toBeVisible();
  expect(runtimeErrors).toEqual([]);
});

test("listening specialty: errors, credit gate and mobile library pagination", async ({ page }, testInfo) => {
  let failCost = true;
  await page.addInitScript(() => localStorage.setItem("accessToken", "test-only"));
  await page.route("**/telemetry/**", route => route.fulfill({ status: 204 }));
  await page.route("**/graphql", async route => {
    const { query } = route.request().postDataJSON(); let data = {};
    if (query.includes("listeningTrainingGenerationCost")) {
      if (failCost) { await route.fulfill({ json: { errors: [{ message: "暂时无法确认点数" }] } }); return; }
      data = { listeningTrainingGenerationCost: 0 };
    } else if (query.includes("startListeningTraining(")) { await route.fulfill({ json: { errors: [{ message: "生成服务暂时不可用，请稍后重试" }] } }); return; }
    else if (query.includes("listeningTrainings")) data = { listeningTrainings: Array.from({ length: 8 }, (_, i) => ({ id: `l${i}`, part: i % 4 + 1, status: "READY", title: `Practice number ${i}`, targetBand: 6.5, createdAt: new Date(2026, 8, i + 1).toISOString() })) };
    else if (query.includes("me {")) data = { me: { id: "qa", username: "qa", totalXP: 0 } };
    await route.fulfill({ json: { data } });
  });
  await page.goto("/listening");
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "我知道了", exact: true }).click();
  await expect(page.getByRole("button", { name: "生成听力训练", exact: true })).toBeDisabled();
  failCost = false;
  await page.getByRole("button", { name: "刷新列表" }).click();
  await expect(page.getByText("本次生成不消耗 AI 点数")).toBeVisible();
  await expect(page.locator(".listening-record")).toHaveCount(6);
  await page.getByRole("button", { name: "下一页", exact: true }).click();
  await expect(page.locator(".listening-record")).toHaveCount(2);
  await page.getByLabel("搜索听力训练").fill("Practice number 7");
  await expect(page.locator(".listening-record")).toHaveCount(1);
  await page.getByLabel("搜索听力训练").fill("");
  await page.getByLabel("筛选听力部分").selectOption("1");
  await expect(page.locator(".listening-record")).toHaveCount(2);
  for (const width of [320, 390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    const bounds = await page.locator(".listening-page").boundingBox();
    expect(Math.abs(bounds.x - (width - bounds.x - bounds.width))).toBeLessThan(2);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.screenshot({ path: testInfo.outputPath("listening-desktop-library.png"), fullPage: true });
  await page.getByRole("button", { name: "生成听力训练", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("生成服务暂时不可用");
});

test("CET selection, ETA, background return, results and deletion", async ({ page }) => {
  const { state, qa } = await createMockExam(page);
  await page.goto("http://127.0.0.1:5187/mock-exam");
  await page.getByText("本次生成消耗 100 积分").waitFor();
  await page.getByRole("button", { name: "大学英语四级", exact: true }).click();
  await expect(page.getByRole("radio", { name: "6.5", exact: true })).toHaveCount(0);
  state.estimatedReadySeconds = 180;
  state.generationEstimateSamples = 2;
  await page.getByRole("button", { name: "生成我的模拟试卷" }).click();
  await expect(page.getByText(/预计还需约 3 分钟/)).toBeVisible();
  expect(qa.calls.find(call => call.name === "StartMockExam").variables.exam).toBe("CET4");
  await page.getByRole("button", { name: "返回模拟考试", exact: true }).click();
  await expect(page.getByRole("button", { name: "删除", exact: true })).toBeDisabled();
  state.status = "COMPLETED";
  state.result = { scoreScale: "PERCENTAGE", totalScore: 72.5, estimatedBand: 0, readingScore: 70, listeningScore: 75, writingScore: 74, translationScore: 70, listeningCorrect: 20, listeningTotal: 25, readingCorrect: 22, readingTotal: 30, strengths: [], weaknesses: [], recommendations: [], writingEvaluations: [] };
  await page.getByRole("button", { name: "刷新考试记录" }).click();
  await page.getByRole("button", { name: /大学英语四级.*已完成/ }).click();
  await expect(page.getByText("72.5", { exact: true })).toBeVisible();
  await expect(page.getByText(/不是官方 710/)).toBeVisible();
  await page.getByRole("button", { name: "返回模拟考试", exact: true }).click();
  await page.getByRole("button", { name: "删除", exact: true }).click();
  await expect(page.getByText(/暂无符合条件的考试/)).toBeVisible();
  expect(qa.calls.some(call => call.name === "DeleteMockExam")).toBeTruthy();
});

test("speaking retains failed draft, restores session and shows partial report", async ({ page }) => {
  const spoken = { id: "speaking-fixture", status: "ACTIVE", part: 1, promptIndex: 0, prompts: [{ part: 1, question: "What do you enjoy learning?", cueCard: "", answerSec: 30, preparationSec: 0 }], turns: [] };
  await page.addInitScript(() => localStorage.setItem("accessToken", "test-only"));
  await page.route("**/telemetry/**", route => route.fulfill({ status: 204 }));
  await page.route("**/graphql", async route => {
    const { query, variables } = route.request().postDataJSON();
    let data = {};
    if (query.includes("startSpeakingSession")) data = { startSpeakingSession: spoken };
    else if (query.includes("submitSpeakingTurn")) { expect(variables.promptIndex).toBe(0); spoken.status = "PROCESSING"; data = { submitSpeakingTurn: spoken }; }
    else if (query.includes("finishSpeakingSession")) { spoken.status = "EVALUATING"; data = { finishSpeakingSession: spoken }; }
    else if (query.includes("speakingSession(")) data = { speakingSession: spoken };
    else if (query.includes("aiCreditCosts")) data = { aiCreditCosts: [] };
    else if (query.includes("me {")) data = { me: { id: "qa", username: "qa", totalXP: 0 } };
    await route.fulfill({ json: { data } });
  });
  await page.goto("http://127.0.0.1:5187/speaking");
  await page.getByRole("button", { name: "开始口语模拟", exact: true }).click();
  await page.getByText("改用文字回答（备用练习方式）", { exact: true }).click();
  await page.getByRole("textbox", { name: "英文回答" }).fill("I enjoy learning languages because they connect people.");
  await page.getByRole("button", { name: "提交文字", exact: true }).click();
  spoken.status = "TURN_FAILED";
  spoken.processingMessage = "考官音频未完成，请重试。";
  await expect(page.getByRole("textbox", { name: "英文回答" })).toBeEnabled();
  await expect(page.getByRole("textbox", { name: "英文回答" })).toHaveValue("I enjoy learning languages because they connect people.");
  await page.getByRole("button", { name: "提交文字", exact: true }).click();
  spoken.turns = [{ part: 1, promptIndex: 0, prompt: spoken.prompts[0].question, transcript: "I enjoy learning languages because they connect people." }];
  spoken.promptIndex = 1; spoken.status = "READY_TO_FINISH"; spoken.processingMessage = "";
  await page.getByRole("button", { name: "提交并评估", exact: true }).click();
  spoken.status = "COMPLETED";
  spoken.evaluation = { isPartial: true, textCoherence: 6.5, lexicalResource: 6.5, grammarAccuracy: 6.5, summary: "文字表达清楚", improvements: ["补充具体事例"], evidence: ["I enjoy learning languages"], strengths: [] };
  await expect(page.getByText("文字表达清楚", { exact: true })).toBeVisible();
  await expect(page.getByText("整体 IELTS 分", { exact: true })).toBeVisible();
  await expect(page.getByText("无音频证据，不提供", { exact: true }).last()).toBeVisible();
  await page.reload();
  await expect(page.getByText("文字表达清楚", { exact: true })).toBeVisible();
});

/**
 * Mocked API frontend regressions: two stateful flows, 33 named checkpoints.
 *
 * Run from repository root:
 *   npm run test:mock-exam --workspace apps/client
 *
 * Requires the existing workspace @playwright/test dependency and Chromium:
 *   npx playwright install --with-deps chromium
 *
 * All GraphQL responses are intercepted. Audio uses local silent WAV fixtures;
 * the continuation assertion stubs play() to inspect event handling. These tests
 * do NOT validate real AI generation, TTS quality, billing or backend enforcement.
 *
 * The .pw.mjs suffix intentionally avoids the existing Vitest file globs.
 * Port 5187 is managed by mock-exam-flow.config.mjs; no backend is needed.
 */
async function createMockExam(page) {

 const context = page.context();
 const p = page;
 const sections=Array.from({length:8},(_,i)=>({key:'S'+(i+1),title:i<4?'Listening Section '+(i+1):i<7?'Reading Passage '+(i-3):'Writing Tasks',skill:i<4?'LISTENING':i<7?'READING':'WRITING',durationSeconds:i<4?450:i<7?1200:3600,instructions:'请根据材料完成本部分。',passage:i>=4&&i<7?'First paragraph.\n\nSecond paragraph.':'',audioUrl:i<4?'/qa-audio-1.wav':null,audioUrls:i<4?['/qa-audio-1.wav','/qa-audio-2.wav']:[],questions:i===7?[]:[{question:'1. Choose a destination.',type:'multiple_choice',options:['A. Library','B. Station']},{question:'2. The library is open.',type:'true_false_not_given',options:['TRUE','FALSE','NOT GIVEN']},{question:'3. Complete the sentence. NO MORE THAN THREE WORDS.',type:'sentence_completion',options:[]}],answers:i===7?[]:['A','NOT GIVEN','saved response'],writingPrompts:i===7?[{title:'Task 1',instructions:'Describe the data.',suggestedWordCount:150},{title:'Task 2',instructions:'Discuss both views.',suggestedWordCount:250}]:[],responses:i===7?['Restored first essay','Restored second essay']:[]}));
 const state={id:'qa-ielts-async',exam:'IELTS',status:'GENERATING',currentSection:'正在生成阅读材料，请稍候…',targetBand:7,paperVersion:'qa-v1',totalDurationSeconds:9000,startedAt:'',submittedAt:null,sections,result:null};
 const qa={calls:[],saveDelay:0,saveFail:false,cost:100,costFail:false,polls:0,errors:[],finishFail:false};
 const checks=[];
 const assert = async (value, label) => {
   await test.step(label, async () => { expect(value, label).toBeTruthy(); });
   checks.push(label);
 };
 const snapshot=()=>{const copy=JSON.parse(JSON.stringify(state));if(['GENERATING','READY','FAILED'].includes(copy.status))copy.sections=copy.sections.map(s=>({...s,passage:null,audioUrl:null,audioUrls:null,questions:null,writingPrompts:null,answers:null,responses:null}));return copy;};
 p.on('pageerror',error=>qa.errors.push(error.message));
 p.on('dialog',dialog=>dialog.accept());
 await p.addInitScript(()=>localStorage.setItem('accessToken','qa-local-only'));
 await p.route('**/graphql',async route=>{
   const {query,variables={}}=route.request().postDataJSON();
   const name=(query.match(/(?:query|mutation)\s+(\w+)/)||[])[1];qa.calls.push({name,variables});
   let data={};
   if(name==='MockExamGenerationCost'){if(qa.costFail){await route.fulfill({json:{errors:[{message:'费用查询暂时失败'}]}});return;}data={mockExamGenerationCost:qa.cost};}
   else if(name==='MockExams')data={mockExams:qa.deleted?[]:[snapshot()]};
   else if(name==='MockExam'){qa.polls++;data={mockExam:snapshot()};}
   else if(name==='StartMockExam'){state.exam=variables.exam;state.targetBand=variables.targetBand;data={startMockExam:snapshot()};}
   else if(name==='DeleteMockExam'){qa.deleted=true;data={deleteMockExam:true};}
   else if(name==='BeginMockExam'){state.status='IN_PROGRESS';state.currentSection='S1';state.startedAt=new Date().toISOString();data={beginMockExam:snapshot()};}
   else if(name==='SaveMockExamAnswers'){
     if(qa.saveDelay)await p.waitForTimeout(qa.saveDelay);
     if(qa.saveFail || (state.startedAt && Date.now()>=Date.parse(state.startedAt)+state.totalDurationSeconds*1000)){await route.fulfill({json:{errors:[{message:'保存服务暂时不可用或考试已到时'}]}});return;}
     const section=state.sections.find(s=>s.key===variables.sectionKey);section.answers=variables.answers;section.responses=variables.responses;data={saveMockExamAnswers:snapshot()};
   }
   else if(name==='SubmitMockExamSection'){const idx=state.sections.findIndex(s=>s.key===variables.sectionKey);state.currentSection=state.sections[Math.min(idx+1,7)].key;data={submitMockExamSection:snapshot()};}
   else if(name==='FinishMockExam'){if(qa.finishFail){await route.fulfill({json:{errors:[{message:'提交暂时失败'}]}});return;}state.status='EVALUATING';state.currentSection='正在评估两篇作文…';data={finishMockExam:snapshot()};}
   else if(query.includes('me {'))data={me:{id:'qa',username:'qa',email:'qa@example.invalid',totalXP:0,emailVerified:true}};
   await route.fulfill({json:{data}});
 });
 await p.route('**/telemetry/**',route=>route.fulfill({status:204}));
 await p.route('**/qa-audio-*.wav',route=>route.fulfill({status:404,body:''}));

 const silentAudio=await p.evaluate(()=>{
   const bytes=new Uint8Array(16044);const view=new DataView(bytes.buffer);
   const write=(offset,text)=>{for(let i=0;i<text.length;i++)bytes[offset+i]=text.charCodeAt(i);};
   write(0,'RIFF');view.setUint32(4,16036,true);write(8,'WAVEfmt ');view.setUint32(16,16,true);view.setUint16(20,1,true);view.setUint16(22,1,true);view.setUint32(24,8000,true);view.setUint32(28,16000,true);view.setUint16(32,2,true);view.setUint16(34,16,true);write(36,'data');view.setUint32(40,16000,true);
   return 'data:audio/wav;base64,'+btoa(String.fromCharCode(...bytes));
 });
 state.sections.filter(s=>s.skill==='LISTENING').forEach(s=>{s.audioUrl=silentAudio+'#clip1';s.audioUrls=[silentAudio+'#clip1',silentAudio+'#clip2'];});

 return { context, p, state, qa, checks, assert };
}

test("generation, saved answers, reload and ordered audio (15 checkpoints)", async ({ page }) => {
 const { p, state, qa, assert } = await createMockExam(page);

   await p.goto('http://127.0.0.1:5187/mock-exam');
   await p.getByText('本次生成消耗 100 积分').waitFor();
   await p.locator('main').ariaSnapshot();
   await p.getByRole('radio',{name:'6.5',exact:true}).check();
   await p.getByRole('button',{name:'生成我的模拟试卷'}).click();
   await p.getByRole('heading',{name:'正在准备你的模拟试卷'}).waitFor();
   await assert(qa.calls.find(c=>c.name==='StartMockExam').variables.targetBand===6.5,'target Band and cost displayed before generation');
   await assert(await p.locator('audio').count()===0,'GENERATING has no exposed content or audio');
   state.status='READY';state.currentSection='S1';
   await p.getByRole('heading',{name:'试卷已就绪，准备开始'}).waitFor();
   await p.locator('main').ariaSnapshot();
   await p.getByRole('button',{name:'开始考试 · 启动计时'}).click();
   await p.getByRole('heading',{name:'Listening Section 1',exact:true}).waitFor();
   await p.locator('main').ariaSnapshot();
   await assert(qa.calls.filter(c=>c.name==='BeginMockExam').length===1,'READY begins with hidden media and content');
   await assert(await p.getByRole('radio',{name:'NOT GIVEN',exact:true}).isChecked(),'full NOT GIVEN value hydrates');
   await assert(await p.getByRole('textbox').inputValue()==='saved response','server completion answer hydrates');
   await assert(await p.locator('audio').evaluate(a=>a.paused),'audio does not autoplay on opening');
   await p.locator('audio').evaluate(a=>{window.qaPlayCount=0;a.play=()=>{window.qaPlayCount++;return Promise.resolve();};a.dispatchEvent(new Event('ended'));});
   await p.getByText('片段 2 / 2',{exact:true}).waitFor();
   await p.locator('audio').evaluate(a=>a.dispatchEvent(new Event('canplay')));
   await assert(await p.evaluate(()=>window.qaPlayCount)===1,'ended advances source and continues playback');
   await assert((await p.locator('audio').getAttribute('src')).includes('#clip2'),'ordered second audio source');
   await p.getByRole('button',{name:'上一片段',exact:true}).click();
   await p.getByText('片段 1 / 2',{exact:true}).waitFor();
   qa.saveDelay=1200;
   await p.getByRole('radio',{name:'B. Station',exact:true}).check();
   await p.waitForFunction(()=>document.querySelector('.mock-save-state')?.textContent.includes('正在保存'));
   await p.getByRole('textbox').fill('newer response');
   await p.getByText('答案已保存',{exact:true}).waitFor();
   await assert(await p.getByRole('textbox').inputValue()==='newer response','slow save/poll does not replace latest text');
   await assert(state.sections[0].answers[0]==='B'&&state.sections[0].answers[1]==='NOT GIVEN'&&state.sections[0].answers[2]==='newer response','serialized save sends latest answers with correct option values');
   await p.getByRole('button',{name:'返回模拟考试',exact:true}).click();
   await p.getByRole('button',{name:/雅思 Academic.*目标 6.5/}).click();
   await p.getByRole('heading',{name:'Listening Section 1',exact:true}).waitFor();
   await assert(await p.getByRole('textbox').inputValue()==='newer response','reopening history retains saved answers');
   await p.getByRole('textbox').fill('reload draft');
   await p.reload();
   await p.getByRole('heading',{name:'Listening Section 1',exact:true}).waitFor();
   await assert(await p.getByRole('textbox').inputValue()==='reload draft','reload restores latest local pending draft');
   await p.getByText('答案已保存',{exact:true}).waitFor();
   await p.getByRole('button',{name:'保存并进入下一部分',exact:true}).click();
   await p.getByRole('heading',{name:'Listening Section 2',exact:true}).waitFor();
   await assert(await p.getByText('片段 1 / 2',{exact:true}).count()===1,'section change resets audio clip');
   await assert(await p.locator('audio').evaluate(a=>a.paused),'section change never autoplays');
   await assert(qa.errors.length===0,'no browser runtime errors');

});

test("cost gating, expiry, evaluation retry and report (18 checkpoints)", async ({ page }) => {
 const { p, state, qa, assert } = await createMockExam(page);

   qa.costFail=true;
   await p.goto('http://127.0.0.1:5187/mock-exam');
   await p.getByRole('button',{name:'重新查询费用'}).waitFor();
   await p.locator('main').ariaSnapshot();
   await assert(await p.getByRole('button',{name:'生成我的模拟试卷'}).isDisabled(),'unknown cost disables generation');
   qa.costFail=false;qa.cost=0;
   await p.getByRole('button',{name:'重新查询费用'}).click();
   await p.getByText('本次生成不扣积分').waitFor();
   await assert(await p.getByRole('button',{name:'生成我的模拟试卷'}).isEnabled(),'zero credit cost explicitly displayed');
   const desktop=await p.locator('main').boundingBox();
   await assert(Math.abs(desktop.x+desktop.width/2-720)<2,'desktop page remains centered');
   state.status='IN_PROGRESS';state.currentSection='S1';state.startedAt=new Date(Date.now()-8990000).toISOString();
   await p.getByRole('button',{name:/雅思 Academic.*目标 7.0/}).click();
   await p.getByRole('heading',{name:'Listening Section 1',exact:true}).waitFor();
   await p.locator('main').ariaSnapshot();
   qa.saveFail=true;
   await p.getByRole('textbox').fill('unsent at expiry');
   await p.getByText('保存未完成',{exact:true}).waitFor();
   await p.getByRole('button',{name:'返回模拟考试',exact:true}).click();
   await p.getByText(/操作未完成，请重试/).waitFor();
   await assert(await p.getByRole('textbox').inputValue()==='unsent at expiry','nonexpired save failure prevents leaving and retains input');
   state.startedAt=new Date(Date.now()-9001000).toISOString();
   await p.getByRole('heading',{name:'正在评估你的作答'}).waitFor();
   await p.getByText(/本地未同步内容未计入本次成绩/).waitFor();
   await assert(qa.calls.filter(c=>c.name==='FinishMockExam').length===1,'expiry submits despite rejected save');
   const cache=await p.evaluate(()=>sessionStorage.getItem('ielts-mock-draft:qa-ielts-async'));
   await assert(cache.includes('unsent at expiry'),'expired unsent draft remains in session cache');
   await p.waitForTimeout(2400);
   await assert(qa.calls.filter(c=>c.name==='FinishMockExam').length===1,'polling does not repeat timeout submission');
   await p.getByRole('button',{name:'返回模拟考试',exact:true}).click();
   await p.getByRole('heading',{name:'我的模拟考试'}).waitFor();
   await assert(await p.evaluate(()=>sessionStorage.getItem('ielts-mock-draft:qa-ielts-async')!==null),'return after expiry preserves pending draft');
   state.status='EVALUATION_FAILED';state.currentSection='评估服务暂时不可用，请重试。';
   await p.getByRole('button',{name:/雅思 Academic.*目标 7.0/}).click();
   await p.getByRole('heading',{name:'评估暂时未能完成'}).waitFor();
   await p.locator('main').ariaSnapshot();
   await p.getByRole('button',{name:'重试评估',exact:true}).click();
   await p.getByRole('heading',{name:'正在评估你的作答'}).waitFor();
   await assert(qa.calls.filter(c=>c.name==='FinishMockExam').length===2 && qa.calls.filter(c=>c.name==='StartMockExam').length===0,'evaluation retry calls finish without generating or charging again');
   state.status='COMPLETED';state.currentSection='RESULT';state.result={listeningScore:7,readingScore:7.5,writingScore:65,writingTask1Score:60,writingTask2Score:68,writingBand:6.5,writingTask1Band:6,writingTask2Band:7,estimatedBand:7,listeningCorrect:30,listeningTotal:40,readingCorrect:33,readingTotal:40,strengths:['论述结构清晰'],weaknesses:['语法准确度需要提高'],recommendations:['复习从句的使用'],feedback:'继续完成针对性训练。'};
   state.result.writingEvaluations = [0,1].map(index=>({bandEstimate:7,overallScore:700/9,taskResponseScore:700/9,grammarScore:700/9,coherenceScore:700/9,vocabularyScore:700/9,summary:'依据四个维度给出的训练反馈',evidence:[`[${index===0?'taskAchievement':'taskResponse'}] 原文：Libraries provide access；解释：论点清晰`],revisedExcerpt:'Libraries can provide equitable access to learning.'}));
   await p.getByRole('heading',{name:'你的模拟考试报告'}).waitFor();
   await p.locator('main').ariaSnapshot();
   await assert(await p.locator('.mock-score-grid article').nth(2).locator('strong').textContent()==='6.5','report uses writing Band instead of 100 scale');
   await assert(await p.getByText('听力、阅读、写作 · 三技能训练均分',{exact:true}).count()===1,'mean explicitly labeled as three-skill training');
   await assert(await p.getByText('复习从句的使用',{exact:true}).count()===1,'backend recommendations displayed');
   await assert(await p.getByText(/Task [12] · 四维评分与原文依据/).count()===2,'both writing task reports displayed');
   await p.getByText('Task 1 · 四维评分与原文依据',{exact:true}).click();
   const criterionPanel=p.locator('details').filter({hasText:'Task 1 · 四维评分与原文依据'});
   await assert((await criterionPanel.locator('dd').allTextContents()).join(',')==='7.0,7.0,7.0,7.0','four criterion scores use Band scale');
   await assert(await criterionPanel.getByText('【任务完成度】 原文：Libraries provide access；解释：论点清晰',{exact:true}).isVisible(),'criterion label translated and evidence retained');
   await assert(await criterionPanel.getByText('Libraries can provide equitable access to learning.',{exact:true}).isVisible(),'revised writing excerpt displayed');
   await p.setViewportSize({width:390,height:844});
   const mobile=await p.evaluate(()=>({width:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth}));
   await assert(mobile.scroll<=mobile.width,'mobile report has no horizontal overflow');
   await assert(qa.errors.length===0,'no runtime errors in failure and expiry flows');

});
