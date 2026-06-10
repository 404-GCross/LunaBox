import { FastAverageColor } from "fast-average-color";

export const DEFAULT_IMAGE_ACCENT_RGB = "71, 85, 105";

const DEFAULT_IMAGE_ACCENT_VALUE: [number, number, number, number] = [
  71,
  85,
  105,
  255,
];

function clampColorChannel(value: number) {
  return Math.max(32, Math.min(224, Math.round(value)));
}

function formatImageAccentRgb(value: number[]) {
  const [rawRed, rawGreen, rawBlue] = value;
  const neutral = [71, 85, 105];
  const red = clampColorChannel(rawRed * 0.72 + neutral[0] * 0.28);
  const green = clampColorChannel(rawGreen * 0.72 + neutral[1] * 0.28);
  const blue = clampColorChannel(rawBlue * 0.72 + neutral[2] * 0.28);

  return `${red}, ${green}, ${blue}`;
}

function loadImage(imageUrl: string) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const image = new Image();

    image.crossOrigin = "anonymous";
    image.referrerPolicy = "no-referrer";
    image.onload = () => resolve(image);
    image.onerror = reject;
    image.src = imageUrl;
  });
}

/**
 * 检测图片的亮度，返回是否为亮色背景
 * @param imageUrl - 图片URL或路径
 * @returns Promise<boolean> - true 表示亮色背景，false 表示暗色背景
 */
export async function detectImageBrightness(
  imageUrl: string,
): Promise<boolean> {
  const fac = new FastAverageColor();

  try {
    const image = await loadImage(imageUrl);
    const color = fac.getColor(image);

    // 计算相对亮度 (根据 WCAG 标准)
    // https://www.w3.org/TR/WCAG20-TECHS/G17.html
    const luminance
      = (0.299 * color.value[0]
        + 0.587 * color.value[1]
        + 0.114 * color.value[2])
      / 255;

    // 亮度 > 0.5 认为是亮色背景
    return luminance > 0.5;
  }
  catch (error) {
    console.error("Failed to analyze image brightness:", error);
    throw error;
  }
  finally {
    fac.destroy();
  }
}

export async function detectImageAccentRgb(imageUrl: string): Promise<string> {
  const fac = new FastAverageColor();

  try {
    const image = await loadImage(imageUrl);
    const color = fac.getColor(image, {
      algorithm: "sqrt",
      defaultColor: DEFAULT_IMAGE_ACCENT_VALUE,
      mode: "speed",
      silent: true,
    });

    return formatImageAccentRgb(color.value);
  }
  finally {
    fac.destroy();
  }
}
