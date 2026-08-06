import { useEffect, useState } from "react";
import { Coins, RotateCcw } from "lucide-react";
import { aiCreditCosts } from "../api";
import { isCommercialEdition, isMiniProgramEdition } from "../edition";
import type { AICreditCost } from "../types";

export function AICreditCostNotice({ action }: { action: string }) {
  const [cost, setCost] = useState<AICreditCost>();

  useEffect(() => {
    if (!isCommercialEdition || isMiniProgramEdition) return;
    let active = true;
    void aiCreditCosts()
      .then((items) => active && setCost(items.find((item) => item.action === action)))
      .catch(() => active && setCost(undefined));
    return () => {
      active = false;
    };
  }, [action]);

  if (!isCommercialEdition || isMiniProgramEdition) return null;
  if (!cost) return <p className="ai-credit-cost loading"><Coins size={15} /> 正在确认本次 AI 点数消耗…</p>;

  return <p className="ai-credit-cost"><Coins size={15} /><strong>本次将消耗 {cost.credits} AI 点</strong><span>· {cost.description}</span><small><RotateCcw size={12} /> 任务失败自动退款</small></p>;
}
