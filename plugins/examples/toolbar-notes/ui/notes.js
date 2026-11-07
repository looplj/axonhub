(async () => {
  const note = document.getElementById('note');
  const save = document.getElementById('save');
  const status = document.getElementById('status');
  try {
    const init = await axonhub.ready();
    const zh = init.locale.startsWith('zh');
    document.documentElement.lang = init.locale;
    document.getElementById('label').textContent = zh ? '插件共享笔记' : 'Shared plugin notes';
    save.textContent = zh ? '保存' : 'Save';
    const result = await axonhub.graphql('query { pluginKV(key: "scratchpad") { value } }');
    if (result.errors?.length) throw new Error(result.errors[0].message);
    note.value = result.data?.pluginKV?.value ?? '';
    note.disabled = !init.canWriteKV;
    save.disabled = !init.canWriteKV;
    status.textContent = init.canWriteKV
      ? (zh ? '所有可访问该插件的用户共享这份笔记。' : 'These notes are shared with everyone who can access this plugin.')
      : (zh ? '只读：保存需要插件管理权限。' : 'Read only: saving requires plugin management permission.');
    save.onclick = async () => {
      save.disabled = true;
      try {
        const result = await axonhub.graphql('mutation Save($value: String!) { setPluginKV(key: "scratchpad", value: $value) { value } }', { value: note.value });
        if (result.errors?.length) throw new Error(result.errors[0].message);
        status.textContent = zh ? '已保存' : 'Saved';
      } catch (error) { status.textContent = error.message; }
      finally { save.disabled = false; }
    };
  } catch (error) { status.textContent = error.message; }
})();
