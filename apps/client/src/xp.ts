const BASE_STAGE_THRESHOLDS = [0, 120, 280, 480, 720, 980, 1280, 1680];

export type StageProgress = {
  totalXP: number;
  stageIndex: number;
  currentStart: number;
  nextTarget: number;
  currentPercent: number;
  totalProgressPercent: number;
  nextLevelRemaining: number;
};

function getStageThresholds(totalStages: number): number[] {
  const safeTotalStages = Math.max(1, totalStages);
  const requiredLength = safeTotalStages + 1;
  const thresholds = [...BASE_STAGE_THRESHOLDS];

  while (thresholds.length < requiredLength) {
    const last = thresholds[thresholds.length - 1] ?? 0;
    thresholds.push(last + 400);
  }

  return thresholds.slice(0, requiredLength);
}

export function getStageRequirement(stageIndex: number, totalStages: number): number {
  const thresholds = getStageThresholds(totalStages);
  const safeIndex = Math.max(0, Math.min(stageIndex, thresholds.length - 2));
  return thresholds[safeIndex] ?? 0;
}

export function calculateStageProgress(totalXP: number, totalStages: number): StageProgress {
  const thresholds = getStageThresholds(totalStages);
  const safeTotalXP = Math.max(0, totalXP);
  const lastStageIndex = thresholds.length - 2;
  let stageIndex = lastStageIndex;

  for (let index = 0; index <= lastStageIndex; index += 1) {
    if (safeTotalXP < thresholds[index + 1]) {
      stageIndex = index;
      break;
    }
  }

  const currentStart = thresholds[stageIndex] ?? 0;
  const nextTarget = thresholds[stageIndex + 1] ?? currentStart;
  const denominator = Math.max(1, nextTarget - currentStart);
  const finalMilestone = thresholds[thresholds.length - 1] ?? 1;

  return {
    totalXP: safeTotalXP,
    stageIndex,
    currentStart,
    nextTarget,
    currentPercent: Math.min(100, Math.max(0, Math.round(((safeTotalXP - currentStart) / denominator) * 100))),
    totalProgressPercent: Math.min(100, Math.max(0, Math.round((safeTotalXP / Math.max(1, finalMilestone)) * 100))),
    nextLevelRemaining: Math.max(0, nextTarget - safeTotalXP)
  };
}

export const MAX_LEARNING_LEVEL = 999;

export type LearningProgress = {
  level: number;
  xpIntoLevel: number;
  xpToNextLevel: number;
  levelProgress: number;
  rankCode: string;
  rankLabel: string;
};

function xpForNextLevel(level: number): number {
  if (level < 100) return 6 + Math.floor((level - 1) / 25);
  if (level < 300) return 10 + Math.floor((level - 100) / 20);
  if (level < 600) return 20 + Math.floor((level - 300) / 18);
  return 38 + Math.floor((level - 600) / 16);
}

export function getLearningRank(level: number): Pick<LearningProgress, "rankCode" | "rankLabel"> {
  if (level >= 999) return { rankCode: "LEGEND", rankLabel: "Lingua 传说" };
  if (level >= 900) return { rankCode: "SOVEREIGN", rankLabel: "至尊领航者" };
  if (level >= 800) return { rankCode: "CELESTIAL", rankLabel: "星耀宗师" };
  if (level >= 650) return { rankCode: "MASTER", rankLabel: "语言大师" };
  if (level >= 500) return { rankCode: "EXPERT", rankLabel: "表达专家" };
  if (level >= 350) return { rankCode: "SCHOLAR", rankLabel: "语境学者" };
  if (level >= 200) return { rankCode: "ADEPT", rankLabel: "进阶研习者" };
  if (level >= 100) return { rankCode: "VOYAGER", rankLabel: "表达旅人" };
  if (level >= 50) return { rankCode: "EXPLORER", rankLabel: "语言行者" };
  return { rankCode: "NOVICE", rankLabel: "初学探索者" };
}

export function calculateLearningProgress(totalXP: number): LearningProgress {
  let remaining = Math.max(0, totalXP);
  for (let level = 1; level < MAX_LEARNING_LEVEL; level += 1) {
    const required = xpForNextLevel(level);
    if (remaining < required) {
      return { level, xpIntoLevel: remaining, xpToNextLevel: required, levelProgress: Math.floor((remaining * 100) / required), ...getLearningRank(level) };
    }
    remaining -= required;
  }
  return { level: MAX_LEARNING_LEVEL, xpIntoLevel: 0, xpToNextLevel: 0, levelProgress: 100, ...getLearningRank(MAX_LEARNING_LEVEL) };
}
