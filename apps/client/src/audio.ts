import { Howl } from "howler";
import { getApiBaseUrl } from "./api";

let activeClip: Howl | null = null;

export function playClip(url: string, rate = 1): Promise<void> {
  return new Promise((resolve, reject) => {
    if (activeClip) {
      activeClip.stop();
      activeClip.unload();
      activeClip = null;
    }
    const resolvedUrl = resolveAudioUrl(url);
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

function resolveAudioUrl(url: string): string {
  const clean = (url ?? "").trim();
  if (!clean || clean.startsWith("data:") || clean.startsWith("blob:")) {
    return clean;
  }
  if (clean.startsWith("/media/")) {
    return `${getApiBaseUrl()}${clean}`;
  }
  return clean;
}

export function speakText(text: string, rate = 1): Promise<void> {
  return new Promise((resolve, reject) => {
    const synth = window.speechSynthesis;
    if (!synth || !text.trim()) {
      reject(new Error("Speech synthesis unavailable"));
      return;
    }
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.rate = rate;
    utterance.onend = () => resolve();
    utterance.onerror = () => reject(new Error("Speech synthesis failed"));
    synth.cancel();
    synth.speak(utterance);
  });
}
