import { useEffect, useMemo, useState } from "react";
import { BadgeCheck, Coins, Crown, LoaderCircle, ShieldCheck, Sparkles } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { billingProducts, billingStatus, createPaymentOrder, paymentOrder } from "../api";
import { useAppStore } from "../store";
import type { BillingProduct, BillingStatus } from "../types";

function money(cents: number) {
  return `¥${(cents / 100).toFixed(cents % 100 === 0 ? 0 : 1)}`;
}

export function MembershipPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const user = useAppStore((state) => state.user);
  const [products, setProducts] = useState<BillingProduct[]>([]);
  const [status, setStatus] = useState<BillingStatus | null>(null);
  // 当前本地易支付默认配置为微信支付；用户仍可在页面中切换至支付宝。
  const [channel, setChannel] = useState<"alipay" | "wxpay">("wxpay");
  const [loadingCode, setLoadingCode] = useState("");
  const [message, setMessage] = useState("");
  const [checkoutError, setCheckoutError] = useState("");
  const [pendingReturnSeconds, setPendingReturnSeconds] = useState<number | null>(null);
  const orderID = searchParams.get("order")?.trim() ?? "";

  const activeProduct = useMemo(() => products.find((product) => product.code === status?.productCode), [products, status?.productCode]);

  useEffect(() => {
    let active = true;
    void billingProducts()
      .then((nextProducts) => active && setProducts(nextProducts))
      .catch((error: Error) => active && setMessage(error.message || "商品目录暂时不可用"));
    void billingStatus()
      .then((nextStatus) => active && setStatus(nextStatus))
      .catch((error: Error) => active && setMessage(error.message || "请先登录后再继续操作"));
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!orderID) {
      setPendingReturnSeconds(null);
      return;
    }
    let active = true;
    let timer = 0;
    const refresh = async () => {
      try {
        const order = await paymentOrder(orderID);
        if (!active) return;
        if (order.status === "PAID") {
          const nextStatus = await billingStatus();
          if (!active) return;
          setStatus(nextStatus);
          setMessage("支付已确认，会员权益和 AI 点数已到账。");
          setPendingReturnSeconds(null);
          window.clearInterval(timer);
        } else {
          setMessage("支付尚未完成，正在等待易支付确认。");
          setPendingReturnSeconds((seconds) => seconds ?? 5);
        }
      } catch (error) {
        if (active) setMessage((error as Error).message || "订单查询失败");
      }
    };
    void refresh();
    timer = window.setInterval(() => void refresh(), 2500);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [orderID]);

  useEffect(() => {
    if (pendingReturnSeconds === null) return;
    if (pendingReturnSeconds <= 0) {
      navigate("/membership", { replace: true });
      return;
    }
    const timer = window.setTimeout(() => setPendingReturnSeconds((seconds) => seconds === null ? null : seconds - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [navigate, pendingReturnSeconds]);

  function returnToMembership() {
    setPendingReturnSeconds(null);
    navigate("/membership", { replace: true });
  }

  async function beginCheckout(product: BillingProduct) {
    setLoadingCode(product.code);
    setMessage("");
    setCheckoutError("");
    try {
      const order = await createPaymentOrder(product.code, channel);
      if (!order.checkoutURL) throw new Error("支付网关未返回收银台地址");
      window.location.assign(order.checkoutURL);
    } catch (error) {
      const errorMessage = (error as Error).message || "无法创建支付订单";
      setMessage(errorMessage);
      setCheckoutError(errorMessage);
      setLoadingCode("");
    }
  }

  return <main className="page membership-page">
    <section className="card membership-shell">
      <header className="membership-hero">
        <div>
          <p className="eyebrow"><Crown size={14} /> LinguaQuest Membership</p>
          <h1>把学习节奏交给自己</h1>
          <p>会员去广告并获得更多 AI 点数；点数仅用于生成、评分和语音服务，所有学习记录与经验始终保留。</p>
        </div>
        <article className={`rank-badge rank-${user?.rankCode?.toLowerCase() ?? "novice"}`}>
          <Sparkles size={18} /><span>{user?.rankLabel ?? "初学探索者"}</span><strong>Lv.{user?.level ?? 1}</strong>
        </article>
      </header>

      <section className="membership-status" aria-label="当前权益">
        <div><small>当前身份</small><strong>{status?.productName ?? "加载中…"}</strong></div>
        <div><small>可用 AI 点数</small><strong><Coins size={17} /> {status?.creditBalance ?? "--"}</strong></div>
        <div><small>广告</small><strong>{status?.adsFree ? "已关闭" : "免费版展示"}</strong></div>
        <button type="button" className="btn-ghost" onClick={() => navigate("/profile")}>返回设置</button>
      </section>

      {checkoutError ? <section className="membership-checkout-error" role="alert" aria-live="assertive">
        <div><strong>订单创建失败</strong><p>{checkoutError}</p></div>
        <div className="membership-checkout-error-actions">
          <button type="button" className="btn-ghost" onClick={() => navigate("/profile")}>返回设置</button>
          <button type="button" onClick={() => navigate("/")}>返回首页</button>
        </div>
      </section> : null}

      {pendingReturnSeconds !== null ? <section className="membership-pending-return" role="status" aria-live="polite">
        <div><strong>支付尚未完成</strong><p>支付成功后权益会自动到账；未付款将于 {pendingReturnSeconds} 秒后返回会员中心。</p></div>
        <button type="button" className="btn-ghost" onClick={returnToMembership}>立即返回</button>
      </section> : null}

      <section className="payment-channel" aria-label="支付方式">
        <span>支付方式</span>
        <button type="button" className={channel === "alipay" ? "control-chip active" : "control-chip"} onClick={() => setChannel("alipay")}>支付宝</button>
        <button type="button" className={channel === "wxpay" ? "control-chip active" : "control-chip"} onClick={() => setChannel("wxpay")}>微信支付</button>
      </section>

      <section className="membership-products" aria-label="会员商品">
        {products.map((product) => <article key={product.code} className={`membership-product ${product.code === activeProduct?.code ? "active" : ""} ${product.kind === "LIFETIME" ? "lifetime" : ""}`}>
          <div className="membership-product-head"><span>{product.kind === "LIFETIME" ? <Crown size={18} /> : <BadgeCheck size={18} />}</span><div><h2>{product.name}</h2><p>{product.description}</p></div></div>
          <strong className="membership-price">{money(product.amountCents)}<small>{product.kind === "LIFETIME" ? "一次买断" : "/ 30 天"}</small></strong>
          <ul><li><Coins size={15} /> {product.creditAllowance.toLocaleString()} AI 点数</li><li><ShieldCheck size={15} /> 全程去广告</li></ul>
          <button type="button" disabled={Boolean(loadingCode)} onClick={() => void beginCheckout(product)}>{loadingCode === product.code ? <><LoaderCircle size={16} className="spin" /> 创建订单中</> : product.code === activeProduct?.code ? "续费 / 补充点数" : "立即开通"}</button>
        </article>)}
      </section>

      <p className="membership-note">免费用户每日有 20 点 AI 点数；买断会员永久去广告，并每 30 天重置 1,200 点。{message}</p>
    </section>
  </main>;
}
