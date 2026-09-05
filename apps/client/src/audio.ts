import { Howl } from "howler";
import { getApiBaseUrl } from "./api";

let activeClip: Howl | null = null;
const prefetchedClips = new Set<string>();
const MAX_PREFETCHED_CLIPS = 24;

export function preloadClip(url: string): void {
  const resolvedUrl = resolveAudioUrl(url);
  if (!resolvedUrl || resolvedUrl.startsWith("data:") || resolvedUrl.startsWith("blob:") || prefetchedClips.has(resolvedUrl)) {
    return;
  }
  if (prefetchedClips.size >= MAX_PREFETCHED_CLIPS) {
    const oldest = prefetchedClips.values().next().value as string | undefined;
    if (oldest) prefetchedClips.delete(oldest);
  }
  prefetchedClips.add(resolvedUrl);
  void fetch(resolvedUrl, { cache: "force-cache" }).then((response) => {
    if (!response.ok) prefetchedClips.delete(resolvedUrl);
  }).catch(() => {
    prefetchedClips.delete(resolvedUrl);
  });
}

export function playClip(url: string, rate = 1): Promise<void> {
  return new Promise((resolve, reject) => {
    if (activeClip) {
      activeClip.stop();
      activeClip.unload();
      activeClip = null;
    }
    const resolvedUrl = resolveAudioUrl(url);
    preloadClip(url);
    const audio = new Howl({
      src: [resolvedUrl],
      html5: true,
      rate,
      onend: () => {
        audio.unload();
        if (activeClip === audio) {
          activeClip = null;
        }
        resolve();
      },
      onloaderror: (_id, error) => {
        audio.unload();
        if (activeClip === audio) {
          activeClip = null;
        }
        reject(error);
      },
      onplayerror: (_id, error) => {
        audio.unload();
        if (activeClip === audio) {
          activeClip = null;
        }
        reject(error);
      }
    });
    activeClip = audio;
    audio.play();
  });
}

export function resolveAudioUrl(url: string): string {
  const clean = (url ?? "").trim();
  if (!clean || clean.startsWith("data:") || clean.startsWith("blob:")) {
    return clean;
  }
  if (clean.startsWith("/media/")) {
    return `${getApiBaseUrl()}${clean}`;
  }
  return clean;
}

export function speakText(text: string, rate = 1, lang = "en-US"): Promise<void> {
  return new Promise((resolve, reject) => {
    const synth = window.speechSynthesis;
    if (!synth || !text.trim()) {
      reject(new Error("Speech synthesis unavailable"));
      return;
    }
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.rate = rate;
    utterance.lang = lang;
    utterance.onend = () => resolve();
    utterance.onerror = () => reject(new Error("Speech synthesis failed"));
    synth.cancel();
    synth.speak(utterance);
  });
}

/** 停止当前文件音频或浏览器语音，切换演示内容时调用。 */
export function stopAudioPlayback(): void {
  if (activeClip) {
    activeClip.stop();
    activeClip.unload();
    activeClip = null;
  }
  window.speechSynthesis?.cancel();
}
