(async () => {
  const status = document.getElementById("status");
  const review = document.getElementById("review");
  try {
    const { locale } = await axonhub.ready();
    const zh = locale.startsWith("zh");
    document.documentElement.lang = locale;
    status.textContent = zh ? "正在读取检查结果…" : "Loading the last check…";
    document.getElementById("available-label").textContent = zh ? "可导入模型" : "Models to import";
    document.getElementById("changed-label").textContent = zh ? "参考价有变化的模型" : "Models with reference price changes";
    review.textContent = zh ? "打开全局模型库" : "Open model library";
    review.disabled = false;
    review.addEventListener("click", async () => {
      try {
        await axonhub.navigate("/models");
      } catch (error) {
        status.textContent = error.message;
      }
    });

    const result = await axonhub.graphql('query { pluginKV(key: "summary") { value } }');
    if (result.errors?.length) throw new Error(result.errors[0].message);
    const value = result.data?.pluginKV?.value;
    if (!value) {
      status.textContent = zh
        ? "还没有成功的检查。可在插件管理的任务列表中立即运行。"
        : "No successful check yet. Run the task from plugin management.";
      return;
    }

    const { provider, fetchedAt, available, changed } = JSON.parse(value);
    const date = new Date(fetchedAt).toLocaleString(locale);
    status.textContent = zh ? `${provider} · 上次成功检查 ${date}` : `${provider} · Last successful check ${date}`;
    document.getElementById("available").textContent = String(available);
    document.getElementById("changed").textContent = String(changed);
    document.getElementById("summary").hidden = false;
  } catch (error) {
    status.textContent = error.message;
  }
})();
