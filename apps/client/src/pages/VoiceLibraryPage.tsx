import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { ArrowLeft, AudioLines, CheckCircle2, Clock3, Plus, RefreshCw, Trash2, Volume2, Waves } from "lucide-react";
import { approveVoiceProfile, deleteVoiceProfile, getVoiceProfiles } from "../api";
import { resolveAudioUrl } from "../audio";
import type { VoiceProfile } from "../types";

type VoiceFilter = "ALL" | "CANTONESE" | "ENGLISH";

const voiceFilterLabels: Record<VoiceFilter, string> = {
  ALL: "全部",
  CANTONESE: "粤语",
  ENGLISH: "英语"
};

function voiceStatus(profile: VoiceProfile) {
  if (profile.status === "READY") return "可用于剧场";
  if (profile.status === "PREVIEW") return "待试听确认";
  if (profile.status === "FAILED") return "生成失败";
  return "后台生成中";
}

function previewQualityMessage(duration?: number) {
  if (duration === undefined) return "请先完整试听这段样本，再确认保存";
  if (duration < 8) return "样本时长偏短，建议删除后使用更完整的语料重做";
  if (duration < 15) return `样本约 ${Math.round(duration)} 秒，确认前请重点检查停顿和语气`;
  return `样本约 ${Math.round(duration)} 秒，建议完整试听后确认`;
}

export function VoiceLibraryPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [profiles, setProfiles] = useState<VoiceProfile[]>([]);
  const [filter, setFilter] = useState<VoiceFilter>("ALL");
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState(() => (location.state as { message?: string } | null)?.message ?? "");
  const [previewDurations, setPreviewDurations] = useState<Record<string, number>>({});
  const [previewErrors, setPreviewErrors] = useState<Record<string, string>>({});
  const [previewPlayed, setPreviewPlayed] = useState<Record<string, boolean>>({});
  const [approvingId, setApprovingId] = useState("");

  const loadProfiles = useCallback(async () => {
    try {
      setProfiles(await getVoiceProfiles());
    } catch (error) {
      console.error("load voice profiles failed", error);
      setMessage("音色库加载失败，请稍后重试。");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  const hasGenerating = useMemo(() => profiles.some((profile) => profile.status === "GENERATING"), [profiles]);
  const visibleProfiles = useMemo(
    () => filter === "ALL" ? profiles : profiles.filter((profile) => profile.language === filter),
    [filter, profiles]
  );
  const readyCount = useMemo(() => profiles.filter((profile) => profile.status === "READY").length, [profiles]);
  const previewCount = useMemo(() => profiles.filter((profile) => profile.status === "PREVIEW").length, [profiles]);
  const generatingCount = useMemo(() => profiles.filter((profile) => profile.status === "GENERATING").length, [profiles]);

  useEffect(() => {
    if (!hasGenerating) return;
    const timer = window.setInterval(() => void loadProfiles(), 1500);
    return () => window.clearInterval(timer);
  }, [hasGenerating, loadProfiles]);

  async function handleDelete(profile: VoiceProfile) {
    if (!window.confirm(`确定删除音色“${profile.name}”吗？已生成的剧场音频不会受影响。`)) return;
    try {
      await deleteVoiceProfile(profile.id);
      setProfiles((current) => current.filter((item) => item.id !== profile.id));
    } catch (error) {
      console.error("delete voice profile failed", error);
      setMessage("删除失败，请稍后重试。");
    }
  }

  async function handleApprove(profile: VoiceProfile) {
    if (!previewDurations[profile.id] || previewDurations[profile.id] < 8 || !previewPlayed[profile.id] || previewErrors[profile.id]) {
      setMessage("请先点击播放试听一次；如果音频无法播放或样本过短，请删除后重做。 ");
      return;
    }
    setApprovingId(profile.id);
    setMessage("");
    try {
      const approved = await approveVoiceProfile(profile.id);
      setProfiles((current) => current.map((item) => item.id === approved.id ? approved : item));
      setMessage(`“${approved.name}”已确认保存，现在可以在剧场生成时选择。`);
    } catch (error) {
      console.error("approve voice profile failed", error);
      setMessage((error as Error).message || "确认保存失败，请稍后重试。 ");
    } finally {
      setApprovingId("");
    }
  }

  return (
    <main className="page voice-library-page">
      <nav className="voice-module-breadcrumb" aria-label="音色库导航">
        <Link to="/profile"><ArrowLeft size={15} /> 账号与服务</Link>
        <span aria-hidden>／</span>
        <strong>角色音色库</strong>
      </nav>

      <section className="card voice-library-hero voice-studio-hero">
        <div className="voice-studio-intro">
          <span className="eyebrow"><Waves size={15} /> Voice studio</span>
          <h2>角色音色库</h2>
          <p>把可复用的角色声线集中在这里创建、试听和管理；生成剧场时即可为人物选择对应音色。</p>
          <dl className="voice-studio-metrics">
            <div><dt><CheckCircle2 size={14} /> 可用</dt><dd>{readyCount}</dd></div>
            <div><dt><Volume2 size={14} /> 待确认</dt><dd>{previewCount}</dd></div>
            <div><dt><AudioLines size={14} /> 全部</dt><dd>{profiles.length}</dd></div>
            <div><dt><Clock3 size={14} /> 队列中</dt><dd>{generatingCount}</dd></div>
          </dl>
        </div>
        <div className="voice-library-actions voice-studio-actions">
          <button className="btn-ghost" type="button" onClick={() => void loadProfiles()} disabled={loading}>
            <RefreshCw size={16} /> 刷新
          </button>
          <button type="button" onClick={() => navigate("/voices/create")}>
            <Plus size={16} /> 创建音色
          </button>
        </div>
      </section>

      <section className="voice-library-toolbar voice-library-console" aria-label="音色筛选">
        <div className="voice-library-stats">
          <span className="voice-console-label">浏览音色</span>
          <strong>{filter === "ALL" ? "全部角色" : `${voiceFilterLabels[filter]}角色`}</strong>
          {hasGenerating ? <small>有音色正在后台生成，会自动刷新</small> : null}
        </div>
        <div className="segmented-control" role="group" aria-label="按语言筛选">
          {(Object.keys(voiceFilterLabels) as VoiceFilter[]).map((item) => (
            <button key={item} type="button" className={filter === item ? "active" : ""} onClick={() => setFilter(item)}>
              {voiceFilterLabels[item]}
            </button>
          ))}
        </div>
      </section>

      {message ? <p className="voice-library-message" role="status">{message}</p> : null}
      {loading ? <section className="card voice-library-empty">正在加载音色库…</section> : null}
      {!loading && visibleProfiles.length === 0 ? (
        <section className="card voice-library-empty">
          <Volume2 size={28} />
          <h3>还没有{filter === "ALL" ? "" : voiceFilterLabels[filter]}音色</h3>
          <p>先设计一条角色声线，再把它分配到剧场人物。</p>
          <button type="button" onClick={() => navigate("/voices/create")}><Plus size={16} /> 创建音色</button>
        </section>
      ) : null}

      <section className="voice-library-grid" aria-live="polite">
        {visibleProfiles.map((profile) => (
          <article key={profile.id} className={`voice-library-card ${profile.status.toLowerCase()}`}>
            <div className="voice-library-card-head">
              <div>
                <span className="voice-card-language">{profile.language === "CANTONESE" ? "粤语" : "英语"}</span>
                <strong>{profile.name}</strong>
              </div>
              <div className="voice-card-actions">
                <span className={`voice-status-chip ${profile.status.toLowerCase()}`}>{voiceStatus(profile)}</span>
                <button type="button" className="icon-button" onClick={() => void handleDelete(profile)} aria-label={`删除${profile.name}音色`}>
                  <Trash2 size={15} />
                </button>
              </div>
            </div>
            <p>{profile.prompt}</p>
            {profile.previewAudioUrl ? (
              <audio
                controls
                controlsList="nodownload noplaybackrate"
                preload="metadata"
                src={resolveAudioUrl(profile.previewAudioUrl)}
                onLoadedMetadata={(event) => {
                  const duration = event.currentTarget.duration;
                  if (!Number.isFinite(duration) || duration <= 0) {
                    setPreviewErrors((current) => ({ ...current, [profile.id]: "试听音频时长读取失败" }));
                    return;
                  }
                  setPreviewDurations((current) => ({ ...current, [profile.id]: duration }));
                  setPreviewErrors((current) => {
                    const next = { ...current };
                    delete next[profile.id];
                    return next;
                  });
                }}
                onPlay={() => setPreviewPlayed((current) => ({ ...current, [profile.id]: true }))}
                onError={() => setPreviewErrors((current) => ({ ...current, [profile.id]: "试听音频无法解码" }))}
              >你的浏览器不支持音频预览。</audio>
            ) : (
              <div className="voice-preview-pending"><Volume2 size={16} /> {profile.status === "FAILED" ? "暂无试听音频" : "试听音频生成后会出现在这里"}</div>
            )}
            {profile.status === "PREVIEW" ? (
              <div className="voice-preview-review">
                <div className={previewErrors[profile.id] ? "voice-preview-quality error" : "voice-preview-quality"}>
                  <Volume2 size={15} />
                  <span>{previewErrors[profile.id] || (previewPlayed[profile.id] ? previewQualityMessage(previewDurations[profile.id]) : "请点击播放试听一次，再确认保存")}</span>
                </div>
                <div className="voice-preview-actions">
                  <button type="button" onClick={() => void handleApprove(profile)} disabled={approvingId !== "" || !previewDurations[profile.id] || previewDurations[profile.id] < 8 || !previewPlayed[profile.id] || Boolean(previewErrors[profile.id])}>
                    <CheckCircle2 size={15} /> {approvingId === profile.id ? "保存中…" : "确认保存"}
                  </button>
                  <button type="button" className="btn-ghost" onClick={() => void handleDelete(profile)} disabled={approvingId !== ""}>
                    <Trash2 size={15} /> 删除重做
                  </button>
                </div>
              </div>
            ) : null}
            <small className="voice-card-message">{profile.generationMessage || "等待处理"}</small>
          </article>
        ))}
      </section>
    </main>
  );
}
