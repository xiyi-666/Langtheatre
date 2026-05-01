import { Howl } from "howler";

let activeClip: Howl | null = null;

export function playClip(url: string, rate = 1): Promise<void> {
  return new Promise((resolve, reject) => {
    if (activeClip) {
      activeClip.stop();
      activeClip.unload();
      activeClip = null;
    }
    const audio = new Howl({
      src: [url],
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
