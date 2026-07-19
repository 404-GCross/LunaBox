import type { i18n as I18nInstance } from "i18next";

import { useEffect, useRef } from "react";
import { toast } from "react-hot-toast";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import { invalidateAllGameLists } from "../cache/gameCache";
import { useAppStore } from "../store";

type DownloadProgressEvent = {
  id: string;
  request?: {
    title?: string;
    download_source?: string;
  };
  status: string;
  error?: string;
};

const IMAGE_DOWNLOAD_SOURCE = "cover-image-batch";

export function useDownloadNotifications(i18n: I18nInstance) {
  const downloadStatusRef = useRef<Record<string, string>>({});

  useEffect(() => {
    const unsubscribeProgress = EventsOn(
      "download:progress",
      (evt: DownloadProgressEvent) => {
        const previousStatus = downloadStatusRef.current[evt.id];
        downloadStatusRef.current[evt.id] = evt.status;

        const title
          = evt.request?.title?.trim()
            || i18n.t("downloads.unknownTitle", "未知标题");
        const isImageDownloadTask
          = evt.request?.download_source === IMAGE_DOWNLOAD_SOURCE;

        if (evt.status === "done" && previousStatus !== "done") {
          if (isImageDownloadTask) {
            toast.success(
              i18n.t("downloads.imageTask.toastDone", "批量图片下载任务已完成"),
              { id: `download-done-${evt.id}` },
            );
            return;
          }

          const message
            = evt.error === "manual_extract_required"
              ? `${i18n.t("downloads.toast.downloadCompleted", { title, defaultValue: "{{title}} 下载完成" })}\n${i18n.t("downloads.toast.manualExtractRequired", "自动解压失败，请手动解压后再导入或启动")}`
              : i18n.t("downloads.toast.downloadCompleted", {
                  title,
                  defaultValue: "{{title}} 下载完成",
                });

          toast.success(message, { id: `download-done-${evt.id}` });
          return;
        }

        if (evt.status === "error" && previousStatus !== "error") {
          const message = evt.error?.trim()
            ? `${i18n.t("downloads.toast.downloadFailed", { title, defaultValue: "{{title}} 下载失败" })}\n${evt.error.trim()}`
            : i18n.t("downloads.toast.downloadFailed", {
                title,
                defaultValue: "{{title}} 下载失败",
              });

          toast.error(message, { id: `download-error-${evt.id}` });
        }
      },
    );

    const unsubscribeGameImported = EventsOn("download:game-imported", () => {
      invalidateAllGameLists();
      void useAppStore.getState().fetchHomeData({
        showLoading: false,
        syncRuntime: false,
      });
    });

    return () => {
      unsubscribeProgress();
      unsubscribeGameImported();
    };
  }, [i18n]);
}
