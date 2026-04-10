import { Howl } from "howler";

export function playClip(url: string, rate = 1): Promise<void> {
  return new Promise((resolve, reject) => {
    const audio = new Howl({
      src: [url],
      html5: true,
      rate,
      onend: () => {
        audio.unload();
        resolve();
      },
      onloaderror: (_id, error) => {
        audio.unload();
        reject(error);
      },
      onplayerror: (_id, error) => {
        audio.unload();
        reject(error);
      }
    });
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
