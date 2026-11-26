(async () => {
  const status = document.getElementById("status");
  const openModels = document.getElementById("open-models");
  try {
    const { locale } = await axonhub.ready();
    const zh = locale.startsWith("zh");
    document.documentElement.lang = locale;
    status.textContent = zh ? "正在读取检查结果…" : "Loading the last check…";
    document.getElementById("scope-label").textContent = zh ? "检查范围" : "Checked scope";
    document.getElementById("available-label").textContent = zh ? "可导入模型" : "Models to import";
    document.getElementById("changed-label").textContent = zh ? "跟随参考价有变化的模型" : "Models with followed price changes";
    openModels.textContent = zh ? "打开全局模型库" : "Open model library";
    openModels.disabled = false;
    openModels.addEventListener("click", async () => {
      try {
        await axonhub.navigate("/models");
      } catch (error) {
        status.textContent = error.message;
      }
    });

    const result = await axonhub.graphql('query { pluginKV(key: "summary") { value } }');
    if (result.errors?.length) throw new Error(result.errors[0].message);
    if (!result.data?.pluginKV?.value) {
      status.textContent = zh ? "尚无成功检查。Owner 可在插件详情中立即运行检查。" : "No successful check yet. Owners can run the task from plugin details.";
      return;
    }

    const { scope, fetchedAt, available, changed } = JSON.parse(result.data.pluginKV.value);
    const labels = [...scope.providers, zh ? "已跟随的参考价" : "Followed prices"];
    if (scope.channels) labels.unshift(zh ? "渠道已配置的模型" : "Configured channel models");
    if (scope.ignorePrefix) labels.push(zh ? "按末段模型名匹配" : "Match by last ID segment");
    status.textContent = (zh ? "上次检查 · " : "Last check · ") + new Date(fetchedAt).toLocaleString(locale);
    document.getElementById("scope").textContent = labels.join(zh ? "、" : ", ");
    document.getElementById("available").textContent = String(available);
    document.getElementById("changed").textContent = String(changed);
    document.getElementById("summary").hidden = false;
  } catch (error) {
    status.textContent = error.message;
  }
})();
