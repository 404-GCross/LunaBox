import type { ImgHTMLAttributes } from "react";
import { useMemo, useState } from "react";
import { useAppStore } from "../../store";
import { proxiedImageSrc, shouldProxyImageSrc } from "../../utils/imageProxy";

type ProxyImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src: string | null | undefined;
  isNSFW?: boolean;
  revealNSFWOnHover?: boolean;
};

export function ProxyImage({
  src,
  referrerPolicy = "no-referrer",
  draggable = false,
  onDragStart,
  onError,
  className,
  isNSFW = false,
  revealNSFWOnHover = false,
  ...props
}: ProxyImageProps) {
  const shouldBlurNSFW = useAppStore(
    state => state.config?.blur_nsfw_game_covers !== false,
  );
  const rawSrc = src?.trim() ?? "";
  const proxySrc = useMemo(() => proxiedImageSrc(rawSrc), [rawSrc]);
  const [failedProxySrc, setFailedProxySrc] = useState("");
  const shouldUseOriginalSrc
    = failedProxySrc === proxySrc && shouldProxyImageSrc(rawSrc);
  const resolvedSrc = shouldUseOriginalSrc ? rawSrc : proxySrc;

  return (
    <img
      {...props}
      src={resolvedSrc}
      className={`${className ?? ""} ${
        isNSFW && shouldBlurNSFW
          ? `nsfw-cover-blur will-change-[filter] transition-[filter] duration-300 ${
            revealNSFWOnHover ? "hover:nsfw-cover-reveal" : ""
          }`
          : ""
      }`.trim()}
      referrerPolicy={referrerPolicy}
      draggable={draggable}
      onError={(event) => {
        if (!shouldUseOriginalSrc && shouldProxyImageSrc(rawSrc)) {
          setFailedProxySrc(proxySrc);
          return;
        }
        onError?.(event);
      }}
      onDragStart={onDragStart ?? (event => event.preventDefault())}
    />
  );
}
