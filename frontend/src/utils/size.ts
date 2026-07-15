/**
 * 格式化文件大小为易读的二进制单位
 * @param bytes
 */
export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0)
    return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.min(
    sizes.length - 1,
    Math.floor(Math.log(bytes) / Math.log(k)),
  );
  return `${(bytes / k ** i).toFixed(1)} ${sizes[i]}`;
}
