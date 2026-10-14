import type { Crop } from "react-image-crop";
import { useEffect, useRef, useState } from "react";
import ReactCrop, { centerCrop, makeAspectCrop } from "react-image-crop";
import { ModalPortal } from "../ui/ModalPortal";
import "react-image-crop/dist/ReactCrop.css";

interface ImageCropperModalProps {
  imagePath: string;
  onConfirm: (crop: {
    x: number;
    y: number;
    width: number;
    height: number;
  }) => void;
  onCancel: () => void;
  windowWidth: number;
  windowHeight: number;
}

// 辅助函数：居中裁剪框
function centerAspectCrop(
  mediaWidth: number,
  mediaHeight: number,
  aspect: number,
) {
  return centerCrop(
    makeAspectCrop(
      {
        unit: "%",
        width: 90,
      },
      aspect,
      mediaWidth,
      mediaHeight,
    ),
    mediaWidth,
    mediaHeight,
  );
}

export function ImageCropperModal({
  imagePath,
  onConfirm,
  onCancel,
  windowWidth,
  windowHeight,
}: ImageCropperModalProps) {
  const [crop, setCrop] = useState<Crop>();
  const [completedCrop, setCompletedCrop] = useState<Crop>();
  const imgRef = useRef<HTMLImageElement>(null);
  const cropAreaRef = useRef<HTMLDivElement>(null);
  const [cropAreaSize, setCropAreaSize] = useState({ width: 0, height: 0 });
  // 默认宽高比为窗口宽高比
  const defaultAspect = windowWidth / windowHeight;
  const [aspect, setAspect] = useState<number | undefined>(defaultAspect);

  useEffect(() => {
    const cropArea = cropAreaRef.current;
    if (!cropArea)
      return;

    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setCropAreaSize(current =>
        current.width === width && current.height === height
          ? current
          : { width, height },
      );
    });

    observer.observe(cropArea);
    return () => observer.disconnect();
  }, []);

  function onImageLoad(e: React.SyntheticEvent<HTMLImageElement>) {
    const { width, height } = e.currentTarget;
    setCrop(centerAspectCrop(width, height, aspect || width / height));
  }

  const handleConfirm = () => {
    if (!completedCrop || !imgRef.current) {
      return;
    }

    const image = imgRef.current;
    const imageRect = image.getBoundingClientRect();
    if (imageRect.width <= 0 || imageRect.height <= 0) {
      return;
    }

    const scaleX = image.naturalWidth / imageRect.width;
    const scaleY = image.naturalHeight / imageRect.height;

    const x = Math.min(
      image.naturalWidth,
      Math.max(0, Math.round(completedCrop.x * scaleX)),
    );
    const y = Math.min(
      image.naturalHeight,
      Math.max(0, Math.round(completedCrop.y * scaleY)),
    );
    const right = Math.min(
      image.naturalWidth,
      Math.max(x, Math.round((completedCrop.x + completedCrop.width) * scaleX)),
    );
    const bottom = Math.min(
      image.naturalHeight,
      Math.max(
        y,
        Math.round((completedCrop.y + completedCrop.height) * scaleY),
      ),
    );

    // 按端点换算并限制在原图范围内，避免贴边裁剪因浮点误差越界。
    const cropData = {
      x,
      y,
      width: right - x,
      height: bottom - y,
    };

    onConfirm(cropData);
  };

  const handleAspectChange = (newAspect: number | null) => {
    setAspect(newAspect || undefined);
    if (imgRef.current) {
      const { width, height } = imgRef.current;
      setCrop(centerAspectCrop(width, height, newAspect || width / height));
    }
  };

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="max-h-[90vh] w-[90vw] max-w-4xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-brand-800 flex flex-col">
          {/* Header */}
          <div className="flex items-center justify-between p-4 border-b border-brand-200 dark:border-brand-700">
            <h2 className="text-lg font-semibold text-brand-900 dark:text-brand-100">
              裁剪背景图片
            </h2>
            <button
              type="button"
              onClick={onCancel}
              aria-label="关闭裁剪窗口"
              className="text-brand-400 hover:text-brand-600 dark:hover:text-brand-200 transition-colors"
            >
              <span className="i-mdi-close text-xl" />
            </button>
          </div>

          {/* 宽高比选择 */}
          <div className="flex items-center gap-2 p-4 border-b border-brand-200 dark:border-brand-700">
            <span className="text-sm text-brand-600 dark:text-brand-300">
              宽高比：
            </span>
            <button
              type="button"
              onClick={() => handleAspectChange(defaultAspect)}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                aspect === defaultAspect
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
              }`}
            >
              {`当前窗口 (${windowWidth}x${windowHeight})`}
            </button>
            <button
              type="button"
              onClick={() => handleAspectChange(null)}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                aspect === undefined
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
              }`}
            >
              自由
            </button>
            <button
              type="button"
              onClick={() => handleAspectChange(16 / 9)}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                aspect === 16 / 9
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
              }`}
            >
              16:9
            </button>
            <button
              type="button"
              onClick={() => handleAspectChange(4 / 3)}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                aspect === 4 / 3
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
              }`}
            >
              4:3
            </button>
            <button
              type="button"
              onClick={() => handleAspectChange(1)}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                aspect === 1
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
              }`}
            >
              1:1
            </button>
          </div>

          {/* Image Crop Area */}
          <div
            ref={cropAreaRef}
            className="min-h-0 flex-1 overflow-hidden p-4 flex items-center justify-center bg-brand-50 dark:bg-brand-900"
          >
            <ReactCrop
              crop={crop}
              onChange={(_, percentCrop) => setCrop(percentCrop)}
              onComplete={c => setCompletedCrop(c)}
              aspect={aspect}
              className="max-w-full max-h-full"
            >
              <img
                ref={imgRef}
                alt="Crop preview"
                src={imagePath}
                onLoad={onImageLoad}
                className="object-contain"
                style={{
                  maxWidth: cropAreaSize.width || "100%",
                  maxHeight: cropAreaSize.height || "60vh",
                }}
                draggable="false"
                onDragStart={e => e.preventDefault()}
              />
            </ReactCrop>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 p-4 border-t border-brand-200 dark:border-brand-700">
            <button
              type="button"
              onClick={onCancel}
              className="px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors text-sm font-medium"
            >
              取消
            </button>
            <button
              type="button"
              onClick={handleConfirm}
              className="px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors text-sm font-medium"
            >
              确认裁剪
            </button>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
