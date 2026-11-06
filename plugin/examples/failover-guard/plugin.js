import { failoverKey } from "./lib/key.js";

const defaultThreshold = 10;
const defaultWindowSeconds = 300;
const maxWindowSeconds = 86400;

export function onEvent(event, ctx) {
  const channel = event.subject && event.subject.channel;
  if (!channel) {
    return;
  }

  const threshold = positiveInt(ctx.config.threshold, defaultThreshold);
  const windowSeconds = windowOf(ctx.config.windowSeconds);
  const count = ctx.counter.add(
    failoverKey(channel.id, event.subject.model),
    windowSeconds,
  );
  if (count !== threshold) {
    return;
  }

  ctx.axonhub.channels.disable(channel.id, { reason: "failover threshold" });

  const target = ctx.config.target;
  if (typeof target !== "string" || target.trim() === "") {
    ctx.log.warn(
      "webhook target is empty; channel disabled without a notification",
      {
        channelId: channel.id,
        count: count,
      },
    );
    return;
  }

  const model = event.subject.model || "unknown";
  const next = event.data && event.data.toChannelName;
  let message =
    channel.name +
    " / " +
    model +
    ": " +
    count +
    " failovers in " +
    windowSeconds +
    "s";
  if (next) {
    message += "; switched to " + next;
  }
  try {
    ctx.webhook.send(target, { message: message });
  } catch (err) {
    ctx.log.error("webhook send failed", {
      channelId: channel.id,
      code: err && err.code ? String(err.code) : "unknown",
    });
  }
}

function positiveInt(value, fallback) {
  if (
    typeof value !== "number" ||
    !isFinite(value) ||
    Math.floor(value) !== value ||
    value < 1
  ) {
    return fallback;
  }
  return value;
}

function windowOf(value) {
  const windowSeconds = positiveInt(value, defaultWindowSeconds);
  if (windowSeconds > maxWindowSeconds) {
    return defaultWindowSeconds;
  }
  return windowSeconds;
}
