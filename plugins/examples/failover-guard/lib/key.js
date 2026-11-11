// Counter keys use the host pattern ^[A-Za-z0-9_.:-]{1,128}$.
const unsafe = /[^A-Za-z0-9_.:-]/g;

export function failoverKey(channelId, model) {
  const safeModel = String(model == null ? "" : model)
    .replace(unsafe, "_")
    .slice(0, 96);
  return (
    "failover:" + channelId + ":" + (safeModel === "" ? "unknown" : safeModel)
  );
}
