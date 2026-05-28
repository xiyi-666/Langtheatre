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
