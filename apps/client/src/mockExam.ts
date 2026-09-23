import type { MockExam } from "./types";

export const MOCK_EXAM_LABELS = { IELTS: "雅思 Academic", CET4: "大学英语四级", CET6: "大学英语六级" } as const;
export type MockExamType = keyof typeof MOCK_EXAM_LABELS;

export function mockExamLabel(exam: string) {
  return MOCK_EXAM_LABELS[exam as MockExamType] ?? exam;
}

export function mockExamETA(exam: Pick<MockExam, "estimatedReadySeconds" | "generationEstimateSamples">) {
  const seconds = exam.estimatedReadySeconds ?? 0;
  const count = exam.generationEstimateSamples ?? 0;
  if (seconds < 0) return "已超过历史平均耗时，仍在处理中，请稍后查看。";
  if (seconds === 0 || count === 0) return "暂无同类考试的历史耗时样本，暂不能准确估算。";
  return `预计还需约 ${Math.max(1, Math.ceil(seconds / 60))} 分钟（参考 ${count} 次同类生成，非完成承诺）。`;
}

export function canDeleteMockExam(status: MockExam["status"]) {
  return ["READY", "COMPLETED", "FAILED", "EVALUATION_FAILED"].includes(status);
}
