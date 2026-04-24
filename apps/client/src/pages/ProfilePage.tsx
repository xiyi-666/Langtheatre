import { FormEvent, useEffect, useMemo, useState } from "react";
import { motion } from "framer-motion";
import { BadgeCheck, Eye, EyeOff, IdCard, Mail, UserRound } from "lucide-react";
import { me, updateProfile } from "../api";
import { useAppStore } from "../store";

export function ProfilePage() {
  const user = useAppStore((s) => s.user);
  const setUser = useAppStore((s) => s.setUser);
  const [nickname, setNickname] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [bio, setBio] = useState("");
  const [message, setMessage] = useState("");
  const [avatarLoadError, setAvatarLoadError] = useState(false);
  const [showEmail, setShowEmail] = useState(false);

  const safeAvatarUrl = useMemo(() => {
    const value = avatarUrl.trim();
    if (!value) return "";
    try {
      const parsed = new URL(value);
      return parsed.protocol === "http:" || parsed.protocol === "https:" ? value : "";
    } catch {
      return "";
    }
  }, [avatarUrl]);


  useEffect(() => {
    void (async () => {
      try {
        const profile = await me();
        setUser(profile);
        setNickname(profile.nickname ?? "");
        setAvatarUrl(profile.avatarUrl ?? "");
        setBio(profile.bio ?? "");
      } catch (e) {
        console.error("load profile failed", e);
      }
    })();
  }, [setUser]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setMessage("");
    try {
      const updated = await updateProfile({ nickname, avatarUrl, bio });
      setUser(updated);
      setMessage("资料已更新");
    } catch (e) {
      console.error("update profile failed", e);
    }
  }

  return (
    <main className="page-center">
      <motion.section className="card auth-shell" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}>
        <form className="auth-main" onSubmit={handleSubmit}>
          <h2>个人中心</h2>
          <article className="profile-hero">
            {safeAvatarUrl ? (
              <img
                className="profile-hero-avatar"
                src={safeAvatarUrl}
                alt="头像预览"
                onError={() => setAvatarLoadError(true)}
                onLoad={() => setAvatarLoadError(false)}
              />
            ) : (
              <div className="profile-hero-avatar profile-hero-avatar-fallback">
                {(nickname.trim() || user?.email?.slice(0, 1) || "U").slice(0, 1).toUpperCase()}
              </div>
            )}
            <div className="profile-hero-meta">
              <strong>{nickname.trim() || "未设置昵称"}</strong>
              <small>当前总 XP：{user?.totalXP ?? 0}</small>
              <p>{bio.trim() || "还没有填写简介，写一句你的学习目标吧。"}</p>
            </div>
          </article>

          <div className="profile-email">
            <span className="profile-email-label"><Mail size={14} /> 邮箱</span>
            <span className="profile-email-value">{showEmail ? user?.email ?? "--" : "已隐藏"}</span>
            <button
              type="button"
              className="profile-email-toggle"
              onClick={() => setShowEmail((prev) => !prev)}
              aria-label={showEmail ? "隐藏邮箱" : "显示邮箱"}
            >
              {showEmail ? <EyeOff size={14} /> : <Eye size={14} />}
              {showEmail ? "隐藏" : "显示"}
            </button>
          </div>
          <label>
            <span><UserRound size={14} /> 昵称</span>
            <input value={nickname} onChange={(e) => setNickname(e.target.value)} />
          </label>
          <label>
            <span><BadgeCheck size={14} /> 简介</span>
            <input value={bio} onChange={(e) => setBio(e.target.value)} />
          </label>
          <label>
            <span><IdCard size={14} /> 头像 URL</span>
            <input value={avatarUrl} onChange={(e) => setAvatarUrl(e.target.value)} />
          </label>
          <button type="submit">保存资料</button>
          {message ? <p>{message}</p> : null}
          {avatarUrl.trim() && !safeAvatarUrl ? <p className="error">头像链接无效，仅支持 http/https 图片链接。</p> : null}
          {avatarLoadError && safeAvatarUrl ? <p className="error">头像加载失败，请确认图片链接可公开访问。</p> : null}
        </form>

        <aside className="floating-panel auth-side">
          <h3>成长轨迹</h3>
          <p>你可以在这里维护学习身份信息，便于复练与分享时展示。</p>
          <div className="mini-progress" aria-hidden>
            <span style={{ width: `${Math.min(100, Math.max(8, (user?.totalXP ?? 0) / 10))}%` }} />
          </div>
          <p>当前总 XP：{user?.totalXP ?? 0}</p>
        </aside>
      </motion.section>
    </main>
  );
}
