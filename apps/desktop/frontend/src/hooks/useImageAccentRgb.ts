import { useEffect, useRef, useState } from "react";

import {
  DEFAULT_IMAGE_ACCENT_RGB,
  detectImageAccentRgb,
} from "../utils/detectImageBrightness";

export function useImageAccentRgb(
  imageUrl: string | null | undefined,
  defaultAccentRgb = DEFAULT_IMAGE_ACCENT_RGB,
) {
  const [accentRgb, setAccentRgb] = useState(defaultAccentRgb);
  const colorCacheRef = useRef(new Map<string, string>());

  useEffect(() => {
    if (!imageUrl) {
      setAccentRgb(defaultAccentRgb);
      return;
    }

    const cachedColor = colorCacheRef.current.get(imageUrl);
    if (cachedColor) {
      setAccentRgb(cachedColor);
      return;
    }

    let isCancelled = false;

    void detectImageAccentRgb(imageUrl)
      .then((rgb) => {
        colorCacheRef.current.set(imageUrl, rgb);
        if (!isCancelled) {
          setAccentRgb(rgb);
        }
      })
      .catch(() => {
        if (!isCancelled) {
          setAccentRgb(defaultAccentRgb);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [defaultAccentRgb, imageUrl]);

  return accentRgb;
}
