import { useEffect, useState } from "react";
import { CheckCircle2, LoaderCircle } from "lucide-react";

interface DemoGenerationProgressProps {
  active: boolean;
  complete?: boolean;
  title: string;
  note: string;
  steps: readonly string[];
  minimumMs?: number;
}

export function DemoGenerationProgress({ active, complete = false, title, note, steps, minimumMs = 1200 }: DemoGenerationProgressProps) {
  const [progress, setProgress] = useState(0);
  const [stepIndex, setStepIndex] = useState(0);

  useEffect(() => {
    if (!active) {
      setProgress(0);
      setStepIndex(0);
      return;
    }
    if (complete) {
      setProgress(100);
      setStepIndex(Math.max(0, steps.length - 1));
      return;
    }

    const startedAt = Date.now();
    setProgress(8);
    setStepIndex(0);
    const timer = window.setInterval(() => {
      const ratio = Math.min(1, (Date.now() - startedAt) / minimumMs);
      const eased = 1 - Math.pow(1 - ratio, 1.2);
      setProgress(Math.min(92, Math.max(8, Math.round(8 + eased * 84))));
      setStepIndex(Math.min(Math.max(0, steps.length - 1), Math.floor(ratio * steps.length)));
    }, 120);
    return () => window.clearInterval(timer);
  }, [active, complete, minimumMs, steps.length]);

  if (!active) return null;

  return (
    <section className="stage-banner demo-generation-card" role="status" aria-live="polite" aria-label={title}>
      <div className="demo-generation-heading">
        <div>
          <strong>{complete ? "内容已准备完成" : title}</strong>
          <p>{note}</p>
        </div>
        {complete ? <CheckCircle2 className="demo-generation-complete" size={22} aria-hidden /> : <LoaderCircle className="demo-generation-loader" size={22} aria-hidden />}
      </div>
      <div className="progress-bar" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress} aria-label="演示内容准备进度">
        <div className="progress-value" style={{ width: `${progress}%` }} />
      </div>
      <div className="demo-generation-progress-label"><span>正在准备演示内容</span><strong>{progress}%</strong></div>
      <div className="demo-generation-steps">
        {steps.map((step, index) => (
          <span key={step} className={index < stepIndex || complete ? "is-done" : index === stepIndex ? "is-active" : undefined}>
            {index < stepIndex || complete ? "✓" : "•"} {step}
          </span>
        ))}
      </div>
    </section>
  );
}
