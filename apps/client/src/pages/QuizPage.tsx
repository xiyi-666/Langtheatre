import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { motion } from "framer-motion";
import { AlarmClock, CheckCircle2 } from "lucide-react";
import { getTheater, submitAnswers } from "../api";
import { useAppStore } from "../store";

export function QuizPage() {
  const { id = "" } = useParams();
  const [questions, setQuestions] = useState<{ question: string; options: string[] }[]>([]);
  const [answers, setAnswers] = useState<string[]>([]);
  const [language, setLanguage] = useState<"CANTONESE" | "ENGLISH">("CANTONESE");
  const [activeIndex, setActiveIndex] = useState(0);
  const [seconds, setSeconds] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [pageTitle, setPageTitle] = useState("听力理解测试");
  const [initialLoad, setInitialLoad] = useState(true);
  const setResult = useAppStore((s) => s.setResult);
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoadError("");
      try {
        const theater = await getTheater(id);
        if (cancelled) return;
        const qs = (theater.quizQuestions ?? []).map((q) => ({
          question: q.question,
          options: Array.isArray(q.options) ? q.options.filter((item) => item.trim() !== "") : []
        }));
        setQuestions(qs);
        setAnswers(qs.map(() => ""));
        setLanguage(theater.language);
        setPageTitle(theater.language === "ENGLISH" ? "Listening comprehension" : "听力理解测试");
        if (qs.length === 0) {
          setLoadError(
            theater.language === "ENGLISH"
              ? "No quiz for this theater. Please generate a new mini-theater."
              : "此剧场暂无测验题，请重新生成小剧场。"
          );
        }
      } catch (e) {
        if (!cancelled) setLoadError((e as Error).message);
      } finally {
        if (!cancelled) setInitialLoad(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    if (initialLoad) return;
    const timer = window.setInterval(() => {
      setSeconds((value) => value + 1);
    }, 1000);
    return () => {
      window.clearInterval(timer);
    };
  }, [initialLoad]);

  async function handleSubmit() {
    setError("");
    setLoading(true);
    try {
      const result = await submitAnswers(id, answers);
      setResult(result);
      navigate("/result");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  const minuteText = String(Math.floor(seconds / 60)).padStart(2, "0");
  const secondText = String(seconds % 60).padStart(2, "0");
  const currentQuestion = questions[activeIndex]?.question ?? "";
  const optionList = questions[activeIndex]?.options ?? [];
  const completion = questions.length > 0 ? Math.round(((activeIndex + 1) / questions.length) * 100) : 0;
  const labels =
    language === "ENGLISH"
      ? {
          current: "Question",
          fill: "Optional fill answer",
          fillPlaceholder: "Type your complete understanding",
          previous: "Previous",
          next: "Next",
          timer: "Elapsed",
          submit: "Submit answers",
          submitting: "Submitting..."
        }
      : {
          current: "问题",
          fill: "填空补充",
          fillPlaceholder: "可继续补充你的理解答案",
          previous: "上一题",
          next: "下一题",
          timer: "用时",
          submit: "提交答案",
          submitting: "提交中..."
        };

  return (
    <main className="page-center">
      <section className="card stage-shell">
        <h2>{pageTitle}</h2>
        {loadError ? <p className="error">{loadError}</p> : null}
        {initialLoad && !loadError ? <p>加载题目中…</p> : null}
        {!initialLoad && !loadError && questions.length > 0 ? (
          <div className="quiz-layout">
            <nav className="quiz-nav">
              {questions.map((_, index) => (
                <button
                  key={`q-${index}`}
                  className={index === activeIndex ? "quiz-dot active" : "quiz-dot"}
                  onClick={() => setActiveIndex(index)}
                >
                  问题 {index + 1}
                </button>
              ))}
            </nav>

            <motion.section initial={{ opacity: 0, x: 10 }} animate={{ opacity: 1, x: 0 }}>
              <p>{labels.current} {activeIndex + 1}/{questions.length}</p>
              <div className="mini-progress" aria-hidden style={{ marginBottom: 10 }}>
                <span style={{ width: `${completion}%` }} />
              </div>
              <h3>{currentQuestion}</h3>

              {optionList.length > 0 ? (
                optionList.map((option) => {
                  const selected = answers[activeIndex] === option;
                  return (
                    <div
                      key={option}
                      className={selected ? "option-item selected" : "option-item"}
                      onClick={() => {
                        const next = [...answers];
                        next[activeIndex] = option;
                        setAnswers(next);
                      }}
                    >
                      {selected ? <CheckCircle2 size={14} /> : null} {option}
                    </div>
                  );
                })
              ) : (
                <label>
                  {labels.fill}
                  <input
                    placeholder={labels.fillPlaceholder}
                    value={answers[activeIndex] ?? ""}
                    onChange={(event) => {
                      const next = [...answers];
                      next[activeIndex] = event.target.value;
                      setAnswers(next);
                    }}
                  />
                </label>
              )}

              <div className="row" style={{ marginTop: 10 }}>
                <button
                  className="btn-ghost"
                  onClick={() => setActiveIndex((value) => Math.max(0, value - 1))}
                >
                  {labels.previous}
                </button>
                <button
                  className="btn-ghost"
                  onClick={() => setActiveIndex((value) => Math.min(questions.length - 1, value + 1))}
                >
                  {labels.next}
                </button>
                <span>
                  <AlarmClock size={14} /> {labels.timer} {minuteText}:{secondText}
                </span>
              </div>
            </motion.section>
          </div>
        ) : null}
        {error ? <p className="error">{error}</p> : null}
        <button
          onClick={handleSubmit}
          disabled={loading || !!loadError || initialLoad || questions.length === 0}
        >
          {loading ? labels.submitting : labels.submit}
        </button>
      </section>
    </main>
  );
}
