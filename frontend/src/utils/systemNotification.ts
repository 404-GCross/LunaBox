import type { NotificationOptions } from "../../wailsjs/runtime/runtime";

import {
  InitializeNotifications,
  IsNotificationAvailable,
  LogWarning,
  SendNotification,
} from "../../wailsjs/runtime/runtime";

let systemNotificationReady: Promise<boolean> | undefined;

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export async function checkSystemNotificationAvailability() {
  try {
    return await IsNotificationAvailable();
  }
  catch (error) {
    LogWarning(
      `Failed to check system notification availability: ${getErrorMessage(error)}`,
    );
    return false;
  }
}

function ensureSystemNotificationReady() {
  systemNotificationReady ??= (async () => {
    try {
      if (!(await checkSystemNotificationAvailability())) {
        return false;
      }

      await InitializeNotifications();
      return true;
    }
    catch (error) {
      LogWarning(
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
    LogWarning(`Failed to send system notification: ${getErrorMessage(error)}`);
    return false;
  }
}
