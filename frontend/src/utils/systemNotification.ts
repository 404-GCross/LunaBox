import type { NotificationOptions } from "../../bindings/github.com/wailsapp/wails/v3/pkg/services/notifications/models";

import {
  CheckNotificationAuthorization,
  RequestNotificationAuthorization,
  SendNotification,
} from "../../bindings/github.com/wailsapp/wails/v3/pkg/services/notifications/notificationservice";

let systemNotificationReady: Promise<boolean> | undefined;

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export async function checkSystemNotificationAvailability() {
  try {
    return await CheckNotificationAuthorization();
  }
  catch (error) {
    console.warn(
      `Failed to check system notification availability: ${getErrorMessage(error)}`,
    );
    return false;
  }
}

function ensureSystemNotificationReady() {
  systemNotificationReady ??= (async () => {
    try {
      if (await checkSystemNotificationAvailability()) {
        return true;
      }
      return await RequestNotificationAuthorization();
    }
    catch (error) {
      console.warn(
        `Failed to initialize system notifications: ${getErrorMessage(error)}`,
      );
      return false;
    }
  })();

  return systemNotificationReady;
}

export async function sendSystemNotification(options: NotificationOptions) {
  if (!(await ensureSystemNotificationReady())) {
    return false;
  }

  try {
    await SendNotification(options);
    return true;
  }
  catch (error) {
    console.warn(
      `Failed to send system notification: ${getErrorMessage(error)}`,
    );
    return false;
  }
}
