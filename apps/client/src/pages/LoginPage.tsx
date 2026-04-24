import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Compass, KeyRound, Mail, Route } from "lucide-react";
import { login, me, register } from "../api";
import { useAppStore } from "../store";

export function LoginPage() {
  const [email, setEmail] = useState("demo@linguaquest.app");
  const [password, setPassword] = useState("demo1234");
  const [isRegister, setIsRegister] = useState(false);
  const setUser = useAppStore((s) => s.setUser);
  const setLoading = useAppStore((s) => s.setLoading);
  const loading = useAppStore((s) => s.loading);
  const navigate = useNavigate();

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    try {
      const token = isRegister ? await register(email, password) : await login(email, password);
      localStorage.setItem("accessToken", token);
      const profile = await me();
      setUser(profile);
      navigate("/courses");
    } catch (e) {
      console.error("auth submit failed", e);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="page-center">
      <motion.section className="card auth-shell" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}>
        <form onSubmit={handleSubmit} className="auth-main">
          <h1>LinguaQuest</h1>
          <p>{isRegister ? "创建账号，进入英粤双路线训练" : "登录后继续你的剧场学习进度"}</p>

          <label>
            <span><Mail size={14} /> 邮箱</span>
            <input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="邮箱" />
          </label>

          <label>
            <span><KeyRound size={14} /> 密码</span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="密码"
            />
          </label>

          <button disabled={loading} type="submit">
            {loading ? "处理中..." : isRegister ? "注册并进入" : "登录"}
          </button>
          <button
            type="button"
            className="btn-ghost"
            onClick={() => setIsRegister((value) => !value)}
          >
            {isRegister ? "已有账号，去登录" : "没有账号，去注册"}
          </button>
        </form>

        <aside className="floating-panel auth-side">
          <h3><Route size={16} /> 学习路径</h3>
          <p>粤语：生活交流 -&gt; 职场表达 -&gt; 时事话题</p>
          <p>英语：日常场景 -&gt; 职场交流 -&gt; 雅思口语</p>
          <div className="mini-progress" aria-hidden>
            <span style={{ width: "64%" }} />
          </div>
          <p><Compass size={14} /> 登录后可直接回到最近一次练习节点。</p>
        </aside>
      </motion.section>
    </main>
  );
}
