import type { ComponentProps } from "react";
import { useAppStore } from "../../store";
import { ProxyImage } from "./ProxyImage";

type GameCoverImageProps = Omit<
  ComponentProps<typeof ProxyImage>,
  "className" | "isNSFW" | "revealNSFWOnHover"
> & {
  className?: string;
  imageClassName?: string;
  isNSFW?: boolean;
  revealNSFWOnHover?: boolean;
};

export function GameCoverImage({
  className,
  imageClassName,
  isNSFW = false,
  revealNSFWOnHover = false,
  ...props
}: GameCoverImageProps) {
  const shouldProtectNSFWCover = useAppStore(
    state => isNSFW && state.config?.blur_nsfw_game_covers !== false,
  );
  const shouldShowWatermark = shouldProtectNSFWCover && revealNSFWOnHover;

  return (
    <span
      className={`relative block overflow-hidden ${className ?? ""}`.trim()}
    >
      <ProxyImage
        {...props}
        isNSFW={isNSFW}
        revealNSFWOnHover={revealNSFWOnHover}
        className={`${imageClassName ?? "h-full w-full object-cover"} ${
          shouldShowWatermark ? "peer" : ""
        }`.trim()}
      />
      {shouldShowWatermark && (
        <span className="pointer-events-none absolute inset-0 flex items-center justify-center transition-opacity duration-300 peer-hover:opacity-0">
          <span
            aria-hidden="true"
            className="i-mdi-eye-outline size-24 text-white opacity-[0.22] drop-shadow-[0_2px_10px_rgba(0,0,0,0.45)]"
          />
        </span>
      )}
    </span>
  );
}
