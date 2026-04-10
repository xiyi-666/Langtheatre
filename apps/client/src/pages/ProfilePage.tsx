import { FormEvent, useEffect, useState } from "react";
import { motion } from "framer-motion";
import { BadgeCheck, IdCard, Mail, UserRound } from "lucide-react";
import { me, updateProfile } from "../api";
import { useAppStore } from "../store";

export function ProfilePage() {
  const user = useAppStore((s) => s.user);
  const setUser = useAppStore((s) => s.setUser);
  const [nickname, setNickname] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [bio, setBio] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      setError("");
      try {
        const profile = await me();
        setUser(profile);
        setNickname(profile.nickname ?? "");
        setAvatarUrl(profile.avatarUrl ?? "");
        setBio(profile.bio ?? "");
      } catch (e) {
        setError((e as Error).message);
      }
    })();
  }, [setUser]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setMessage("");
    setError("");
    try {
      const updated = await updateProfile({ nickname, avatarUrl, bio });
      setUser(updated);
      setMessage("资料已更新");
    } catch (e) {
      setError((e as Error).message);
    }
  }

  return (
    <main className="page-center">
      <motion.section className="card auth-shell" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}>
        <form className="auth-main" onSubmit={handleSubmit}>
          <h2>个人中心</h2>
          <p><Mail size={14} /> 邮箱：{user?.email ?? "--"}</p>
          <label>
            <span><UserRound size={14} /> 昵称</span>
            <input value={nickname} onChange={(e) => setNickname(e.target.value)} />
          </label>
          <label>
            <span><IdCard size={14} /> 头像 URL</span>
            <input value={avatarUrl} onChange={(e) => setAvatarUrl(e.target.value)} />
          </label>
          <label>
            <span><BadgeCheck size={14} /> 简介</span>
            <input value={bio} onChange={(e) => setBio(e.target.value)} />
          </label>
          <button type="submit">保存资料</button>
          {message ? <p>{message}</p> : null}
          {error ? <p className="error">{error}</p> : null}
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
