import { describe, expect, it } from "vitest";
import { canDeleteMockExam, mockExamETA, mockExamLabel } from "./mockExam";

describe("模拟考试状态展示", () => {
  it("不会把冷启动或超时显示为虚假的剩余一分钟", () => {
    expect(mockExamETA({ estimatedReadySeconds: 0, generationEstimateSamples: 0 })).toContain("暂无");
    expect(mockExamETA({ estimatedReadySeconds: -1, generationEstimateSamples: 3 })).toContain("超过");
    expect(mockExamETA({ estimatedReadySeconds: 121, generationEstimateSamples: 2 })).toContain("约 3 分钟");
  });
  it("禁止删除生成、作答和评分中的记录", () => {
    expect(canDeleteMockExam("GENERATING")).toBe(false);
    expect(canDeleteMockExam("IN_PROGRESS")).toBe(false);
    expect(canDeleteMockExam("EVALUATING")).toBe(false);
    expect(canDeleteMockExam("READY")).toBe(true);
    expect(canDeleteMockExam("COMPLETED")).toBe(true);
    expect(canDeleteMockExam("FAILED")).toBe(true);
    expect(canDeleteMockExam("EVALUATION_FAILED")).toBe(true);
  });
  it("独立标注大学英语四级和六级", () => {
    expect(mockExamLabel("CET4")).toBe("大学英语四级");
    expect(mockExamLabel("CET6")).toBe("大学英语六级");
  });
});
