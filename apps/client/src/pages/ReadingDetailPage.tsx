import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { readingMaterial } from "../api";
import type { ReadingMaterial } from "../types";

export function ReadingDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [item, setItem] = useState<ReadingMaterial | null>(null);
  const [error, setError] = useState("");
  const [answers, setAnswers] = useState<Record<number, number>>({});
  const [submitted, setSubmitted] = useState(false);
  const [audioIndex, setAudioIndex] = useState(0);
  const [mergedAudioUrl, setMergedAudioUrl] = useState("");
  const [audioMerging, setAudioMerging] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  const audioQueue = useMemo(() => {
    if (!item) return [] as string[];
    if (item.audioUrls?.length) {
      return item.audioUrls.filter((u) => Boolean(u?.trim()));
    }
    if (item.audioUrl?.trim()) {
      return [item.audioUrl];
    }
    return [] as string[];
  }, [item]);

  const scoreInfo = useMemo(() => {
    if (!item?.questions?.length) return { score: 0, total: 0 };
    const total = item.questions.length;
    let correct = 0;
    item.questions.forEach((q, idx) => {
      const selected = answers[idx];
      if (selected == null) return;
      const userAnswer = q.options?.[selected];
      if (userAnswer && q.answerKey && userAnswer === q.answerKey) {
        correct += 1;
      }
    });
    return { score: correct, total };
  }, [answers, item]);

  useEffect(() => {
    void (async () => {
      try {
        const data = await readingMaterial(id);
        setItem(data);
        setAudioIndex(0);
        setMergedAudioUrl("");
      } catch (e) {
        setError((e as Error).message);
      }
    })();
  }, [id]);

  useEffect(() => {
    let revoked = "";
    let cancelled = false;

    async function mergeChunks() {
      if (audioQueue.length <= 1) return;
      setAudioMerging(true);
      try {
        const url = await mergeAudioChunksToWav(audioQueue);
        if (cancelled) {
          URL.revokeObjectURL(url);
          return;
        }
        revoked = url;
        setMergedAudioUrl(url);
      } catch {
        // Ignore merge failures (e.g. CORS on remote audio), fallback to sequential playback.
        if (!cancelled) {
          setMergedAudioUrl("");
        }
      } finally {
        if (!cancelled) {
          setAudioMerging(false);
        }
      }
    }

    void mergeChunks();

    return () => {
      cancelled = true;
      if (revoked) {
        URL.revokeObjectURL(revoked);
      }
    };
  }, [audioQueue]);

  if (error) {
    return <main className="page"><section className="card"><p className="error">{error}</p></section></main>;
  }
  if (!item) {
    return <main className="page"><section className="card"><p>加载中...</p></section></main>;
  }

  return (
    <main className="page">
      <section className="card">
        <h2>{item.title}</h2>
        <p>{item.topic}</p>
        <p style={{ whiteSpace: "pre-wrap" }}>{item.passage}</p>

        {item.vocabulary?.length ? (
          <article className="stage-banner">
            <strong>重点词汇</strong>
            <p>{item.vocabulary.join(" / ")}</p>
          </article>
        ) : null}

        <article className="stage-banner">
          <strong>阅读题（5题）</strong>
          <ol>
            {(item.questions ?? []).map((q, idx) => (
              <li key={`${q.question}-${idx}`}>
                <p>{q.question}</p>
                {q.options?.length ? (
                  <div className="dialogue-list">
                    {q.options.map((opt, i) => {
                      const selected = answers[idx] === i;
                      const isCorrect = submitted && q.answerKey === opt;
                      const isWrongSelected = submitted && selected && q.answerKey !== opt;
                      const classNames = [
                        "option-item",
                        selected ? "selected" : "",
                        isCorrect ? "correct" : "",
                        isWrongSelected ? "wrong" : ""
                      ]
                        .filter(Boolean)
                        .join(" ");
                      return (
                        <button
                          key={`${opt}-${i}`}
                          type="button"
                          className={classNames}
                          onClick={() => {
                            if (submitted) return;
                            setAnswers((prev) => ({ ...prev, [idx]: i }));
                          }}
                        >
                          {String.fromCharCode(65 + i)}. {opt}
                        </button>
                      );
                    })}
                  </div>
                ) : null}
              </li>
            ))}
          </ol>
          <div className="row">
            <button type="button" onClick={() => setSubmitted(true)}>提交答案</button>
            <button
              type="button"
              className="btn-ghost"
              onClick={() => {
                setAnswers({});
                setSubmitted(false);
              }}
            >
              重做
            </button>
          </div>
          {submitted ? <p>得分：{scoreInfo.score} / {scoreInfo.total}</p> : null}
        </article>

        <article className="stage-banner">
          <strong>全文音频</strong>
          {item.audioStatus === "READY" && audioQueue.length > 0 ? (
            <div className="audio-inline">
              {audioQueue.length > 1 ? (
                <small>
                  {mergedAudioUrl
                    ? "已合并为单条音频播放"
                    : audioMerging
                      ? "正在合并分段音频..."
                      : `合并失败，已回退连续播放（${audioIndex + 1}/${audioQueue.length}）`}
                </small>
              ) : null}
              <audio
                ref={audioRef}
                controls
                preload="none"
                src={mergedAudioUrl || audioQueue[audioIndex]}
                onEnded={() => {
                  if (mergedAudioUrl) return;
                  if (audioIndex >= audioQueue.length - 1) return;
                  const next = audioIndex + 1;
                  setAudioIndex(next);
                  // Let state update first, then continue autoplay for seamless segmented playback.
                  setTimeout(() => {
                    audioRef.current?.play().catch(() => undefined);
                  }, 0);
                }}
              >
                <track kind="captions" />
              </audio>
            </div>
          ) : (
            <p>{item.audioStatus === "FAILED" ? "音频生成失败，请重新生成材料。" : "音频后台生成中，完成后可播放。"}</p>
          )}
        </article>

        <div className="row">
          <button className="btn-ghost" onClick={() => navigate("/reading")}>返回阅读中心</button>
        </div>
      </section>
    </main>
  );
}

async function mergeAudioChunksToWav(urls: string[]): Promise<string> {
  const buffers = await Promise.all(
    urls.map(async (url) => {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`audio fetch failed: ${response.status}`);
      }
      return response.arrayBuffer();
    })
  );

  const audioContext = new AudioContext();
  try {
    const decoded = await Promise.all(buffers.map((buf) => audioContext.decodeAudioData(buf.slice(0))));
    const sampleRate = decoded[0].sampleRate;
    const channelCount = decoded[0].numberOfChannels;
    const totalLength = decoded.reduce((sum, b) => sum + b.length, 0);
    const merged = audioContext.createBuffer(channelCount, totalLength, sampleRate);

    let offset = 0;
    for (const buffer of decoded) {
      for (let channel = 0; channel < channelCount; channel++) {
        merged.getChannelData(channel).set(buffer.getChannelData(channel), offset);
      }
      offset += buffer.length;
    }
    const wavBlob = encodeWav(merged);
    return URL.createObjectURL(wavBlob);
  } finally {
    await audioContext.close();
  }
}

function encodeWav(buffer: AudioBuffer): Blob {
  const channels = buffer.numberOfChannels;
  const sampleRate = buffer.sampleRate;
  const length = buffer.length;
  const bytesPerSample = 2;
  const blockAlign = channels * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = length * blockAlign;
  const wav = new ArrayBuffer(44 + dataSize);
  const view = new DataView(wav);

  writeAscii(view, 0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeAscii(view, 8, "WAVE");
  writeAscii(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, channels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, 16, true);
  writeAscii(view, 36, "data");
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (let i = 0; i < length; i++) {
    for (let channel = 0; channel < channels; channel++) {
      const sample = Math.max(-1, Math.min(1, buffer.getChannelData(channel)[i]));
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
      offset += bytesPerSample;
    }
  }
  return new Blob([wav], { type: "audio/wav" });
}

function writeAscii(view: DataView, offset: number, text: string) {
  for (let i = 0; i < text.length; i++) {
    view.setUint8(offset + i, text.charCodeAt(i));
  }
}
