export function onRequest(ctx, req) {
  const configured = ctx.config && ctx.config.maxTemperature;
  return { ...req, temperature: temperatureOf(configured) };
}

function temperatureOf(value) {
  if (typeof value !== "number" || !isFinite(value)) {
    return 0.2;
  }
  return value;
}
