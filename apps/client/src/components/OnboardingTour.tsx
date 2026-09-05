import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, BookOpenText, Check, Clapperboard, Compass, FilePenLine, X } from "lucide-react";
import { isMiniProgramEdition } from "../edition";
import { useAppStore } from "../store";
import { getOnboardingPendingKey, isOnboardingComplete, markOnboardingComplete } from "../onboarding";

type TourStep = {
  selector: string;
  title: string;
  description: string;
  icon: typeof Compass;
};

const steps: TourStep[] = [
  { selector: '[data-onboarding="courses-nav"]', title: "从学习路线开始", description: "在课程中心选择英语或粤语路线，按阶段练习，逐步积累学习进度。", icon: Compass },
  { selector: '[data-onboarding="generate-nav"]', title: "快速生成练习", description: "进入生成台，选择主题和难度，创建适合当前水平的阅读、对话练习。", icon: Clapperboard },
  { selector: '[data-onboarding="reading-nav"]', title: "阅读与写作练习", description: "阅读库支持材料精读，写作库可以限时完成英语文章并获得 AI 评分建议。", icon: BookOpenText },
  { selector: '[data-onboarding="library-nav"]', title: "在剧场库沉浸表达", description: "在剧场库练习多人对话和角色扮演，把新学内容用到真实场景里。", icon: FilePenLine }
];

type Rect = { top: number; left: number; width: number; height: number };

function getTargetRect(selector: string): Rect | undefined {
  const element = document.querySelector<HTMLElement>(selector);
  if (!element) return undefined;
  const rect = element.getBoundingClientRect();
  return { top: rect.top, left: rect.left, width: rect.width, height: rect.height };
}

export function OnboardingTour() {
  const user = useAppStore((state) => state.user);
  const [stepIndex, setStepIndex] = useState(0);
  const [rect, setRect] = useState<Rect>();
  const [visible, setVisible] = useState(false);
  const dialogRef = useRef<HTMLElement>(null);

  const finish = useCallback(() => {
    if (user?.id) markOnboardingComplete(user.id);
    setVisible(false);
  }, [user?.id]);

  const locateStep = useCallback((index: number): boolean => {
    const step = steps[index];
    if (!step) return false;
    const element = document.querySelector<HTMLElement>(step.selector);
    if (!element) return false;
    if (window.innerWidth <= 768) {
      element.scrollIntoView({ behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "center", inline: "center" });
    }
    setRect(getTargetRect(step.selector));
    return true;
  }, []);

  useEffect(() => {
    if (!isMiniProgramEdition || !user?.id || isOnboardingComplete(user.id)) {
      setVisible(false);
      return;
    }
    if (localStorage.getItem(getOnboardingPendingKey(user.id)) !== "1") {
      setVisible(false);
      return;
    }
    setStepIndex(0);
    setVisible(true);
  }, [user?.id]);

  useEffect(() => {
    if (!visible) return;
    let frame = 0;
    const refresh = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        if (!locateStep(stepIndex)) {
          const nextAvailable = steps.findIndex((_, index) => index >= stepIndex && locateStep(index));
          if (nextAvailable < 0) finish();
          else setStepIndex(nextAvailable);
        }
      });
    };
    refresh();
    window.addEventListener("resize", refresh);
    window.addEventListener("scroll", refresh, true);
    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("resize", refresh);
      window.removeEventListener("scroll", refresh, true);
    };
  }, [finish, locateStep, stepIndex, visible]);

  useEffect(() => {
    if (!visible) return;
    dialogRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        finish();
      } else if (event.key === "ArrowRight") {
        event.preventDefault();
        if (stepIndex >= steps.length - 1) finish();
        else setStepIndex((current) => current + 1);
      } else if (event.key === "ArrowLeft" && stepIndex > 0) {
        event.preventDefault();
        setStepIndex((current) => current - 1);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [finish, stepIndex, visible]);

  if (!visible || !rect) return null;
  const step = steps[stepIndex];
  const Icon = step.icon;
  const isLast = stepIndex === steps.length - 1;

  return (
    <div className="onboarding-overlay" aria-hidden={false}>
      <div className="onboarding-spotlight" style={{ top: rect.top - 8, left: rect.left - 8, width: rect.width + 16, height: rect.height + 16 }} />
      <section className="onboarding-card" role="dialog" aria-modal="true" aria-labelledby="onboarding-title" tabIndex={-1} ref={dialogRef}>
        <button type="button" className="onboarding-close" aria-label="关闭新手引导" onClick={finish}><X size={17} /></button>
        <div className="onboarding-progress" aria-label={`第 ${stepIndex + 1} 步，共 ${steps.length} 步`}>
          {steps.map((item, index) => <span key={item.title} className={index <= stepIndex ? "active" : ""} />)}
        </div>
        <span className="onboarding-icon"><Icon size={22} /></span>
        <p className="onboarding-kicker">新手引导 · {stepIndex + 1}/{steps.length}</p>
        <h2 id="onboarding-title">{step.title}</h2>
        <p>{step.description}</p>
        <div className="onboarding-actions">
          {stepIndex > 0 ? <button type="button" className="btn-ghost" onClick={() => setStepIndex((current) => current - 1)}><ArrowLeft size={16} /> 上一步</button> : <span />}
          <button type="button" className="btn-ghost onboarding-skip" onClick={finish}>跳过</button>
          <button type="button" onClick={() => isLast ? finish() : setStepIndex((current) => current + 1)}>{isLast ? <><Check size={16} /> 开始学习</> : <>下一步 <ArrowRight size={16} /></>}</button>
        </div>
      </section>
    </div>
  );
}
