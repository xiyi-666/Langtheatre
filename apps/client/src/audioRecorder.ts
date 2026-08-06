function encodeWav(audioBuffer: AudioBuffer): Blob {
  const channels = Math.min(1, audioBuffer.numberOfChannels);
  const samples = audioBuffer.length;
  const bytes = new ArrayBuffer(44 + samples * 2);
  const view = new DataView(bytes);
  const write = (offset: number, value: string) => [...value].forEach((char, index) => view.setUint8(offset + index, char.charCodeAt(0)));
  write(0, "RIFF"); view.setUint32(4, 36 + samples * 2, true); write(8, "WAVE"); write(12, "fmt "); view.setUint32(16, 16, true);
  view.setUint16(20, 1, true); view.setUint16(22, channels, true); view.setUint32(24, audioBuffer.sampleRate, true); view.setUint32(28, audioBuffer.sampleRate * channels * 2, true);
  view.setUint16(32, channels * 2, true); view.setUint16(34, 16, true); write(36, "data"); view.setUint32(40, samples * 2, true);
  const channel = audioBuffer.getChannelData(0);
  for (let index = 0; index < samples; index += 1) { view.setInt16(44 + index * 2, Math.max(-1, Math.min(1, channel[index])) * 0x7fff, true); }
  return new Blob([bytes], { type: "audio/wav" });
}

export async function recordingToWavDataURL(recording: Blob): Promise<string> {
  const AudioContextClass = window.AudioContext || (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AudioContextClass) throw new Error("当前浏览器不支持音频转换，请上传 WAV/MP3 或使用新版 Chrome。");
  const context = new AudioContextClass();
  try {
    const decoded = await context.decodeAudioData(await recording.arrayBuffer());
    const wav = encodeWav(decoded);
    if (wav.size > 10 * 1024 * 1024) throw new Error("录音转换后超过 10MB，请缩短到 90 秒以内。");
    return await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onerror = () => reject(reader.error ?? new Error("录音读取失败")); reader.onload = () => resolve(String(reader.result)); reader.readAsDataURL(wav); });
  } finally { await context.close(); }
}
