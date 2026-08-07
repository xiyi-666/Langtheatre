import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { trackClick } from "../api";
import { motion } from "framer-motion";
import { ArrowLeft, AtSign, Compass, KeyRound, LifeBuoy, Mail, Route, ShieldCheck, UserRound, UserSearch } from "lucide-react";
import { login, loginCandidates, me, register, requestEmailVerification, requestPasswordReset, requestUsernameRecovery, resetPassword, verifyEmail } from "../api";
import type { AuthResult, LoginCandidate } from "../types";
import { useAppStore } from "../store";

type Screen = "login" | "register" | "forgot-password" | "forgot-username" | "account-select" | "reset" | "verify" | "verification-pending";
type SelectionAction = "login" | "reset" | "verify";

const passwordRule = /^(?=.*[A-Z])(?=.*[a-z])(?=.*\d).{8,15}$/;

export function LoginPage() {
  const [searchParams] = useSearchParams();
  const verifyToken = searchParams.get("verify")?.trim() ?? "";
  const resetToken = searchParams.get("reset")?.trim() ?? "";
  const [screen, setScreen] = useState<Screen>(() => (verifyToken ? "verify" : resetToken ? "reset" : "login"));
  const [identifier, setIdentifier] = useState("");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [candidates, setCandidates] = useState<LoginCandidate[]>([]);
  const [selectionAction, setSelectionAction] = useState<SelectionAction>("login");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const setUser = useAppStore((state) => state.setUser);
  const setLoading = useAppStore((state) => state.setLoading);
  const loading = useAppStore((state) => state.loading);
  const navigate = useNavigate();

  const passwordIsStrong = useMemo(() => passwordRule.test(password), [password]);

  function begin(screenName: Screen) {
    setScreen(screenName);
    setError("");
    setMessage("");
    setCandidates([]);
  }

  const completeAuth = useCallback(async (result: AuthResult) => {
    if (!result.accessToken) {
      throw new Error(result.message || "登录未完成");
    }
    localStorage.setItem("accessToken", result.accessToken);
		if (result.refreshToken) {
			localStorage.setItem("refreshToken", result.refreshToken);
		} else {
			localStorage.removeItem("refreshToken");
		}
    const profile = await me();
    setUser(profile);
    navigate("/courses");
  }, [navigate, setUser]);

  useEffect(() => {
    if (!verifyToken || screen !== "verify") return;
    let active = true;
    setLoading(true);
    void verifyEmail(verifyToken)
      .then(async (result) => {
        if (!active) return;
        await completeAuth(result);
      })
      .catch((caught: unknown) => {
        if (active) setError((caught as Error).message || "验证链接无效或已过期");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [completeAuth, screen, setLoading, verifyToken]);

  async function handleLogin(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const accounts = await loginCandidates(identifier);
      if (accounts.length > 1) {
        setCandidates(accounts);
        setSelectionAction("login");
        setScreen("account-select");
        return;
      }
      await completeAuth(await login(identifier, password, accounts[0]?.id));
    } catch (caught) {
      setError((caught as Error).message || "登录失败，请检查用户名、邮箱和密码。");
    } finally {
      setLoading(false);
    }
  }

  async function handleRegister(event: FormEvent) {
    event.preventDefault();
    if (!passwordIsStrong) {
      setError("密码须为 8–15 位，并同时包含大写字母、小写字母和数字。");
      return;
    }
    if (password !== confirmPassword) {
      setError("两次输入的密码不一致。");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const result = await register(username, email, password);
      if (result.accessToken) {
        await completeAuth(result);
        return;
      }
      setIdentifier(email);
      setMessage(result.message || "账号已创建，请前往邮箱完成验证。");
      setScreen("verification-pending");
    } catch (caught) {
      setError((caught as Error).message || "注册失败，请稍后重试。");
    } finally {
      setLoading(false);
    }
  }

  async function handlePasswordResetRequest(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const result = await requestPasswordReset(identifier);
      if (result.requiresSelection) {
        setCandidates(result.candidates ?? []);
        setSelectionAction("reset");
        setScreen("account-select");
        return;
      }
      setMessage(result.message || "如果账号存在，重置链接已发送到邮箱。");
    } catch (caught) {
      setError((caught as Error).message || "无法发送重置邮件。");
    } finally {
      setLoading(false);
    }
  }

  async function handleUsernameRecovery(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      await requestUsernameRecovery(email);
      setMessage("如果该邮箱已注册，用户名找回邮件已发送。");
    } catch (caught) {
      setError((caught as Error).message || "无法发送找回邮件。");
    } finally {
      setLoading(false);
    }
  }

  async function handleResetPassword(event: FormEvent) {
    event.preventDefault();
    if (!passwordIsStrong || password !== confirmPassword) {
      setError(password !== confirmPassword ? "两次输入的密码不一致。" : "密码须为 8–15 位，并同时包含大写字母、小写字母和数字。");
      return;
    }
    setLoading(true);
    setError("");
    try {
      await resetPassword(resetToken, password);
      setPassword("");
      setConfirmPassword("");
      setMessage("密码已更新，请使用新密码登录。");
      setScreen("login");
    } catch (caught) {
      setError((caught as Error).message || "重置链接无效或已过期。");
    } finally {
      setLoading(false);
    }
  }

  async function handleSelection(account: LoginCandidate) {
    setLoading(true);
    setError("");
    try {
      if (selectionAction === "login") {
        await completeAuth(await login(identifier, password, account.id));
      } else if (selectionAction === "reset") {
        const result = await requestPasswordReset(identifier, account.id);
        setMessage(result.message || "重置链接已发送到邮箱。");
        setScreen("forgot-password");
      } else {
        const result = await requestEmailVerification(identifier, account.id);
        setMessage(result.message || "验证邮件已发送。");
        setScreen("verification-pending");
      }
    } catch (caught) {
      setError((caught as Error).message || "操作失败，请稍后重试。");
    } finally {
      setLoading(false);
    }
  }

  async function resendVerification() {
    setLoading(true);
    setError("");
    try {
      const result = await requestEmailVerification(identifier);
      if (result.requiresSelection) {
        setCandidates(result.candidates ?? []);
        setSelectionAction("verify");
        setScreen("account-select");
        return;
      }
      setMessage(result.message || "验证邮件已发送。");
    } catch (caught) {
      setError((caught as Error).message || "无法发送验证邮件。");
    } finally {
      setLoading(false);
    }
  }

  const title = screen === "register" ? "创建 LinguaQuest 账号" : screen === "forgot-password" ? "找回密码" : screen === "forgot-username" ? "找回用户名" : screen === "reset" ? "设置新密码" : "LinguaQuest";

  return (
    <main className="page-center">
      <motion.section className="card auth-shell" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}>
        <section className="auth-main">
          <div className="auth-brand">
            <img src="/linguaquest-mark.svg" width="62" height="62" alt="LinguaQuest" />
            <div><h1>{title}</h1><p>英粤双路线训练，从每一次真实表达开始。</p></div>
          </div>

          {screen === "verify" ? <div className="auth-notice"><ShieldCheck size={18} /> {loading ? "正在验证邮箱…" : error || "邮箱验证成功"}</div> : null}

          {screen === "login" ? <form onSubmit={handleLogin}>
            <label><span><AtSign size={14} /> 用户名或邮箱</span><input required value={identifier} onChange={(event) => setIdentifier(event.target.value)} placeholder="请输入用户名或邮箱" /></label>
            <label><span><KeyRound size={14} /> 密码</span><input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="请输入密码" /></label>
            <button disabled={loading} type="submit">{loading ? "登录中…" : "登录"}</button>
            <section className="auth-assistance" aria-labelledby="account-help-title">
              <div className="auth-assistance-head">
                <span className="auth-assistance-mark"><LifeBuoy size={16} /></span>
                <div>
                  <strong id="account-help-title">需要账号协助？</strong>
                  <p>通过已验证邮箱安全恢复账号信息。</p>
                </div>
              </div>
              <div className="auth-recovery-grid">
                <button type="button" className="auth-recovery-action password" onClick={() => { trackClick("LOGIN_PASSWORD_RECOVERY"); begin("forgot-password"); }}>
                  <span className="auth-recovery-action-icon"><KeyRound size={18} /></span>
                  <span><strong>重置密码</strong><small>使用用户名或邮箱继续</small></span>
                </button>
                <button type="button" className="auth-recovery-action username" onClick={() => { trackClick("LOGIN_USERNAME_RECOVERY"); begin("forgot-username"); }}>
                  <span className="auth-recovery-action-icon"><UserSearch size={18} /></span>
                  <span><strong>找回用户名</strong><small>发送账号清单到邮箱</small></span>
                </button>
              </div>
            </section>
            <button type="button" className="btn-ghost" onClick={() => { trackClick("LOGIN_REGISTER_ENTRY"); begin("register"); }}>没有账号，去注册</button>
          </form> : null}

          {screen === "register" ? <form onSubmit={handleRegister}>
            <label><span><UserRound size={14} /> 用户名</span><input required value={username} onChange={(event) => setUsername(event.target.value)} placeholder="3–24 位：字母、数字、_ 或 -" /></label>
            <label><span><Mail size={14} /> 邮箱</span><input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="用于验证与找回账号" /></label>
            <label><span><KeyRound size={14} /> 密码</span><input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="8–15 位，含大小写字母和数字" /></label>
            <label><span><KeyRound size={14} /> 确认密码</span><input required type="password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} placeholder="再次输入密码" /></label>
            <small className={password && !passwordIsStrong ? "error" : "auth-help"}>密码须为 8–15 位，并同时包含大写字母、小写字母和数字。同一邮箱最多注册 3 个账号。</small>
            <button disabled={loading} type="submit">{loading ? "创建中…" : "注册并发送验证邮件"}</button>
            <button type="button" className="btn-ghost" onClick={() => begin("login")}>已有账号，去登录</button>
          </form> : null}

          {screen === "forgot-password" ? <form onSubmit={handlePasswordResetRequest}>
            <div className="auth-flow-intro">
              <span className="auth-flow-icon password"><KeyRound size={20} /></span>
              <div><p className="auth-flow-eyebrow">账号协助 · 密码恢复</p><h2>通过邮箱重置密码</h2><p>输入用户名或邮箱；若邮箱关联多个账号，下一步会让你选择要继续操作的账号。</p></div>
            </div>
            <ol className="auth-steps" aria-label="密码找回步骤"><li><span>1</span>确认账号</li><li><span>2</span>查收重置邮件</li><li><span>3</span>设置新密码</li></ol>
            <label><span><AtSign size={14} /> 用户名或邮箱</span><input required value={identifier} onChange={(event) => setIdentifier(event.target.value)} placeholder="请输入用户名或邮箱" /></label>
            <button disabled={loading} type="submit">{loading ? "发送中…" : "发送重置邮件"}</button>
            <button type="button" className="btn-ghost auth-return" onClick={() => begin("login")}><ArrowLeft size={16} /> 返回登录</button>
          </form> : null}

          {screen === "forgot-username" ? <form onSubmit={handleUsernameRecovery}>
            <div className="auth-flow-intro">
              <span className="auth-flow-icon username"><UserSearch size={20} /></span>
              <div><p className="auth-flow-eyebrow">账号协助 · 用户名恢复</p><h2>把账号名称发回给你</h2><p>输入注册邮箱，我们会把该邮箱关联的用户名发送给你，不会泄露密码。</p></div>
            </div>
            <ol className="auth-steps" aria-label="用户名找回步骤"><li><span>1</span>确认邮箱</li><li><span>2</span>查收账号邮件</li><li><span>3</span>使用用户名登录</li></ol>
            <label><span><Mail size={14} /> 注册邮箱</span><input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="请输入注册邮箱" /></label>
            <button disabled={loading} type="submit">{loading ? "发送中…" : "找回用户名"}</button>
            <button type="button" className="btn-ghost auth-return" onClick={() => begin("login")}><ArrowLeft size={16} /> 返回登录</button>
          </form> : null}

          {screen === "reset" ? <form onSubmit={handleResetPassword}>
            <div className="auth-flow-intro compact">
              <span className="auth-flow-icon password"><ShieldCheck size={20} /></span>
              <div><p className="auth-flow-eyebrow">密码恢复 · 最后一步</p><h2>设置新的安全密码</h2><p>使用 8–15 位、同时含大写字母、小写字母和数字的密码。</p></div>
            </div>
            <label><span><KeyRound size={14} /> 新密码</span><input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="8–15 位，含大小写字母和数字" /></label>
            <label><span><KeyRound size={14} /> 确认新密码</span><input required type="password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} placeholder="再次输入新密码" /></label>
            <button disabled={loading} type="submit">{loading ? "更新中…" : "更新密码"}</button>
          </form> : null}

          {screen === "account-select" ? <div className="account-select"><div className="auth-flow-intro compact"><span className="auth-flow-icon selection"><UserRound size={20} /></span><div><p className="auth-flow-eyebrow">账号协助 · 请选择账号</p><h2>这个邮箱关联多个账号</h2><p>请选择本次要登录或找回密码的账号，我们将仅向该账号发送邮件。</p></div></div>{candidates.map((candidate) => <button type="button" className="account-option" key={candidate.id} disabled={loading} onClick={() => void handleSelection(candidate)}><UserRound size={16} /><span><strong>{candidate.username}</strong><small>{candidate.email}</small></span></button>)}<button type="button" className="btn-ghost auth-return" onClick={() => begin(selectionAction === "login" ? "login" : "forgot-password")}><ArrowLeft size={16} /> 返回</button></div> : null}

          {screen === "verification-pending" ? <div className="verification-pending"><ShieldCheck size={24} /><h2>请验证邮箱</h2><p>{message || "验证链接已发送，请在 30 分钟内完成验证。"}</p><button disabled={loading} type="button" onClick={() => void resendVerification()}>重新发送验证邮件</button><button type="button" className="btn-ghost" onClick={() => begin("login")}>返回登录</button></div> : null}

          {message && screen !== "verification-pending" ? <p className="success">{message}</p> : null}
          {error && screen !== "verify" ? <p className="error">{error}</p> : null}
        </section>

		<aside className="floating-panel auth-side">
          <h3><Route size={16} /> 学习路径</h3>
          <p>粤语：生活交流 -&gt; 职场表达 -&gt; 时事话题</p>
          <p>英语：日常场景 -&gt; 职场交流 -&gt; 雅思口语</p>
          <div className="mini-progress" aria-hidden><span style={{ width: "64%" }} /></div>
			<p><Compass size={14} /> 邮箱验证后可安全恢复学习进度。</p>
			<Link className="auth-release-notes-link" to="/updates">查看 V1.0.1 版本说明</Link>
		</aside>
      </motion.section>
    </main>
  );
}
