import { useEffect, useState } from "react";
import { adPlacements } from "../api";
import { isCommercialEdition } from "../edition";
import type { AdPlacement } from "../types";

type RenderableAdPlacement = AdPlacement & Required<Pick<AdPlacement, "scriptURL" | "slotId">>;

// eslint-disable-next-line react-refresh/only-export-components
export function isRenderableAdPlacement(slot: AdPlacement | null): slot is RenderableAdPlacement {
  return Boolean(
    slot
      && slot.provider.trim().toUpperCase() !== "NONE"
      && slot.scriptURL?.trim()
      && slot.slotId?.trim()
  );
}

export function AdSlot({ placement }: { placement: AdPlacement["placement"] }) {
  const [slot, setSlot] = useState<AdPlacement | null>(null);

  useEffect(() => {
    if (!isCommercialEdition) {
      setSlot(null);
      return;
    }
    let active = true;
    void adPlacements()
      .then((items) => {
        if (active) setSlot(items.find((item) => item.placement === placement) ?? null);
      })
      .catch(() => {
        if (active) setSlot(null);
      });
    return () => {
      active = false;
    };
  }, [placement]);

  useEffect(() => {
    if (!isRenderableAdPlacement(slot)) return;
    const selector = `script[data-linguaquest-ad="${placement}"]`;
    if (document.querySelector(selector)) return;
    const script = document.createElement("script");
    script.src = slot.scriptURL;
    script.async = true;
    script.dataset.linguaquestAd = placement;
    document.head.appendChild(script);
  }, [placement, slot]);

  if (!isCommercialEdition || !isRenderableAdPlacement(slot)) return null;
  return <aside className="ad-slot" data-ad-provider={slot.provider} data-ad-slot={slot.slotId} aria-label="赞助内容" />;
}
