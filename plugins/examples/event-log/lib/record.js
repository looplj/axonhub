export function recordKey(seq) {
  const digits = String(seq);
  const pad =
    digits.length >= 8 ? digits : "00000000".slice(digits.length) + digits;
  return "event." + pad;
}

export function summarize(event, seq) {
  const subject = event && event.subject ? event.subject : {};
  const channel = subject.channel ? subject.channel : {};
  const credential = subject.credential ? subject.credential : {};
  const data = event && event.data ? event.data : {};
  const model = subject.model ? String(subject.model) : "";
  return {
    seq: seq,
    type: event && event.type ? String(event.type) : "",
    severity: event && event.severity ? String(event.severity) : "",
    occurredAt: event && event.occurredAt ? String(event.occurredAt) : "",
    channelId: channel.id == null ? null : channel.id,
    channelName: channel.name ? String(channel.name) : "",
    model: model,
    credential: credential.masked ? String(credential.masked) : "",
    detail: detailOf(event && event.type, data),
  };
}

function detailOf(type, data) {
  if (type === "channel.credential_disabled") {
    const parts = [];
    if (data.reason) {
      parts.push(String(data.reason));
    }
    if (data.expiresAt) {
      parts.push("until " + data.expiresAt);
    }
    return parts.join(" · ");
  }
  if (type === "channel.credential_enabled") {
    return data.trigger ? String(data.trigger) : "";
  }
  if (type === "request.completed") {
    const parts = [];
    if (data.latencyMs != null) {
      parts.push(String(data.latencyMs) + " ms");
    }
    if (data.promptTokens != null || data.completionTokens != null) {
      const prompt = data.promptTokens == null ? 0 : data.promptTokens;
      const completion =
        data.completionTokens == null ? 0 : data.completionTokens;
      parts.push(prompt + " → " + completion + " tokens");
    }
    if (data.stream === true) {
      parts.push("stream");
    }
    return parts.join(" · ");
  }
  return "";
}
