# Realtime Gateway

本文描述独立的 Realtime WebSocket 协议入口。开发分支为 `feat/qwen-realtime`。它与 `/v1/responses` WebSocket、Chat、TTS、STT 使用不同的协议和会话处理。

## 验证状态与上线门槛

- 以 `.agents/summary/realtime-delivery.md` 中保存的实际测试结果为准。Mock 验证只能证明本地契约与资源管理，不能证明官方模型可用、音质或实际账单正确。
- `gpt-realtime-2.1`、`gpt-realtime-2.1-mini`、Qwen3.8 及两个 Qwen3.5 模型分别需要真实官方账户验收。
- 本环境的 OpenAI 环境凭证属于第三方中转地址，不作为 OpenAI 官方凭证使用；没有百炼凭证及 Workspace。官方真实验收未执行，属于上线阻断项。
- 上线前还需要验证实际反向代理、Redis 故障模式、取消/断线、多轮、实际音频质量与时长、费用和供应商控制台账单。

## 管理界面配置

1. 保留现有 `openai` 或 `bailian` 渠道，不新建服务商类型。在渠道密钥管理中加入对应官方 API Key。
2. 在端点管理中显式添加 `openai/realtime-ws` 或 `qwen/realtime-ws`。可为端点单独设置 1000–60000 毫秒握手超时；留空继承服务配置。删除该显式端点即可停止新会话；不修改渠道原有 HTTP Base URL。
3. OpenAI 官方使用 `wss://api.openai.com` 和 `/v1/realtime`。首批支持 `gpt-realtime-2.1`、`gpt-realtime-2.1-mini`，不自动选择已弃用型号。
4. 百炼选择地域与 Workspace，生成 `wss://{WorkspaceId}.{region}.maas.aliyuncs.com`，路径为 `/api-ws/v1/realtime`。地域只接受 `cn-beijing` 和 `ap-southeast-1`。Workspace、地域、密钥必须属于同一官方资源配置。
5. 加入上游支持模型并配置模型映射、模型关联和 API Key Profile 授权。Qwen 首批型号为 `qwen3.8-omni-flash-realtime`、`qwen3.5-omni-plus-realtime`、`qwen3.5-omni-flash-realtime`。如需单独测试，使用模型协议覆盖将该模型固定到实时协议。
6. 配置分项价格，不能用普通文本模型的 `prompt_tokens`/`completion_tokens` 代替实时价格。使用管理员维护的官方当前费率与币种，不能混合人民币与美元。
7. 渠道连接测试执行真实 WS 握手、会话配置确认及轻量 conversation item 确认，不触发生成。此探测成功不代表真实语音或账单验收通过。

## 客户端契约

统一入口为 `GET /v1/realtime?model={网关模型或别名}`，使用标准 WebSocket Upgrade 和 `Authorization: Bearer {AxonHub API Key}`。只支持服务端客户端；浏览器 Origin、URL 凭证及客户端指定的上游 Host 均被拒绝。入口使用严格鉴权，不回退到 No-Auth。

原生 Qwen 调试入口为 `/v1/realtime/qwen?model={模型}`，同样经过权限、配额、价格与连接限制，仅能选择百炼实时端点；它也只暴露已实现且通过校验的安全能力子集。

OpenAI 上游使用原生 GA 事件，保留合法未知扩展字段和事件顺序，不发送 `OpenAI-Beta: realtime=v1`，不经 Qwen 转换，不对音频重采样。OpenAI 在 `session.created` 提供完整有效音频配置时可直接使用已确认的默认值；修改参数时应发送 `session.update` 并等待 `session.updated`，再按新参数发送媒体。禁止在会话中修改模型、覆盖凭证或身份安全字段。Safety Identifier 由已鉴权的网关密钥及用户/项目生成不可逆摘要，不接受客户端覆盖。

Qwen 上游使用独立适配器。GA 文本事件为 `response.output_text.delta/done`，GA 音频为 `response.output_audio.delta/done`，音频转录为 `response.output_audio_transcript.delta/done`。工具调用的 call_id、参数增量、最终参数和 function_call_output 必须保持关联。不能兼容的参数明确拒绝。

远程 MCP、视频、多声道、浏览器短期令牌和 Preview 协议兼容不是本期默认能力。不得因为上游接受字段而宣称网关已支持这些功能。原生 OpenAI 支持的图像计费必须有完整价格与用量分项。

下面是 Node.js 服务端最小调用示例（安装 `ws`）：

```js
import WebSocket from 'ws';
const ws = new WebSocket('wss://axonhub.example/v1/realtime?model=voice', {
  headers: { Authorization: `Bearer ${process.env.AXONHUB_API_KEY}` },
});
ws.on('message', (bytes) => {
  const e = JSON.parse(bytes.toString());
  if (e.type === 'session.created') {
    ws.send(JSON.stringify({ type: 'session.update', session: {
      type: 'realtime', output_modalities: ['text'],
      audio: { input: { turn_detection: null } },
    }}));
  }
  if (e.type === 'session.updated') {
    ws.send(JSON.stringify({ type: 'conversation.item.create', item: {
      type: 'message', role: 'user',
      content: [{ type: 'input_text', text: '你好，请简短介绍自己。' }],
    }}));
    ws.send(JSON.stringify({ type: 'response.create' }));
  }
  if (e.type === 'response.output_text.delta') process.stdout.write(e.delta);
  if (e.type === 'response.done') ws.close();
});
```

Qwen 原生入口使用相同 Header，`session.update` 使用 `session.modalities: ['text']`、`session.turn_detection: null`，省略 GA 的 `session.type`。原生文本事件仍是 Qwen 的 `response.text.delta/done`。

## 用量、价格与配额

在发送音频或 `response.create` 前预约仅含元数据的请求记录，随后将 `response.created` 关联到该记录；`response.done` 的确认 usage 恰好记账一次。会话可包含多条独立记录，每会话最多 512 条审计预约，已完成、已清空和在途记录均计入该边界，去重集合也有界。异常关闭保留已经确认的用量，没有最终 usage 的响应记为失败且 `usage_confirmed: false`，不生成伪造零用量账单。

实时价格独立于历史价格项，覆盖输入文本、输入音频、输出文本、输出音频，以及 OpenAI 的输入图像、缓存文本/音频/图像。缓存是各输入模态的子集，必须从对应非缓存数量中扣除。计算使用 decimal；精确 CostItem 小计及价格来源用于审计。沿用历史 totalCost 字段的数值存储兼容边界，不改变旧账单。

Qwen3.8 的语音及对应文本按各自官方输出费率计费；Qwen3.5 语音输出对应文本通过型号计费规则保留实际 Token，并将对应文本计费数量记为 0；纯文本及工具参数仍按文本费率计算，不能套用 Qwen3.8 的费率。会话建立时冻结价格来源；缺必需价格拒绝建立连接。

在建立连接前检查配额，并在会话中周期检查及阻止后续输入；VAD 自主生成期间也检查。它是使用量核查，不是供应商端硬费用预付锁：一条已经生成的响应可能在最终 usage 确认前超过余额。灰度前必须按业务预算确认这一边界。

## 连接限制与部署

配置例子：

```yaml
server:
  realtime:
    handshake_timeout: 15s
    idle_timeout: 2m
    max_duration: 15m
    limits:
      global: 32
      project: 8
      api_key: 1
      channel: 8
      connections_per_minute: 20
```

配置使用现有 YAML 与环境变量加载机制，例如 `AXONHUB_SERVER_REALTIME_MAX_DURATION=15m`（具体前缀以现有部署配置为准）。没有另一个必填 Feature Flag；显式端点就是启用条件。

共享 Redis 从现有 `cache.redis` 配置读取，在第一次实时会话时延迟连接。四个维度的并发与每密钥建连频率在单槽 Lua 中原子获取，租约超过会话硬时限，崩溃自动到期；Redis 不可用时拒绝新会话。没有 Redis 时计数仅适用于单进程；多实例部署必须使用相同 Redis 和相同限制，不可称本地计数为集群配额。Redis 的数据持久化、主从切换丢写风险必须纳入部署验收。

单消息上限 256 KiB，音频块解码上限 64 KiB，每方向发送队列最多 8 条，队列满时关闭而非丢弃媒体。慢客户端、握手、配置确认、空闲和总时长均有边界。停止服务时取消现有会话并关闭两端，不无限等待长连接。

Nginx 示例（放入现有 TLS server；保留其他路由）：

```nginx
map $http_upgrade $realtime_connection {
    default upgrade;
    '' close;
}
# 在已有 server {} 内：
location /v1/realtime {
    proxy_pass http://axonhub:8090;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $realtime_connection;
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Host $host;
    proxy_read_timeout 20m;
    proxy_send_timeout 20m;
    proxy_buffering off;
}
```

容器不需要新增端口。负载均衡必须支持 Upgrade，现有 HTTP 健康检查只能检测进程存活，不能作为 Realtime 配置成功证据。

复用现有 OpenTelemetry Provider 导出 `axonhub_realtime_active_sessions`、建连/拒绝/上游失败事件计数、会话时长、首次输出延迟及分模态 Token。请求与用量页面展示确认的费用；日志只含渠道、网关密钥记录 ID、时长和关闭状态，不包含长期凭证、原始媒体、Base64 或对话文本。

## 实际验收与回滚

从仓库根目录执行可复现 smoke（凭证只从环境读取，不放命令行或 URL）：

```sh
go run ./scripts/realtime-smoke -model gpt-realtime-2.1-mini -audio-pcm /secure/input-24k-mono.pcm
go run ./scripts/realtime-smoke -model gpt-realtime-2.1 -audio-pcm /secure/input-24k-mono.pcm
go run ./scripts/realtime-smoke -model qwen3.8-omni-flash-realtime -audio-pcm /secure/input-24k-mono.pcm
go run ./scripts/realtime-smoke -qwen-native -model qwen3.8-omni-flash-realtime -audio-pcm /secure/input-24k-mono.pcm
```

需设置 `AXONHUB_API_KEY`。脚本覆盖配置、文本、音频输入/输出、多轮和工具结果继续生成，打印确认 usage；它不播放或保存音频，音质和时长需单独验收，VAD、取消和异常网络还需按测试清单验证。

先备份数据库和渠道配置，在单个受限项目启用端点，设低并发/短时长/小配额，核对供应商账单再扩大。回滚先移除实时端点或禁用相应渠道，等待有界会话结束或停止服务取消连接，再回滚应用版本。此实现不引入数据库表迁移，但新价格码与端点应在回滚旧应用前删除；备份并保留新版本确认的账单审计数据，旧应用可能不认识新价格枚举。

## 官方资料

- [OpenAI WebSocket](https://developers.openai.com/api/docs/guides/voice-websockets)
- [OpenAI GA 客户端事件](https://developers.openai.com/api/reference/resources/realtime/client-events)
- [OpenAI GA 服务端事件](https://developers.openai.com/api/reference/resources/realtime/server-events)
- [GPT-Realtime-2.1](https://developers.openai.com/api/docs/models/gpt-realtime-2.1)
- [GPT-Realtime-2.1 mini](https://developers.openai.com/api/docs/models/gpt-realtime-2.1-mini)
- [百炼 WebSocket](https://help.aliyun.com/zh/model-studio/omni-realtime-websocket-access)
- [百炼客户端事件](https://help.aliyun.com/zh/model-studio/client-events)
- [百炼服务端事件](https://help.aliyun.com/zh/model-studio/server-events)
- [Qwen3.8](https://help.aliyun.com/zh/model-studio/qwen3-8-omni-flash-realtime)

Nginx 本地验证可执行 `go run ./scripts/realtime-proxy-check`（需要 Docker 与 `nginx:alpine`）。该脚本使用本地 Mock 回显，验证真实 Nginx 的 HTTP 101、鉴权 Header、查询参数和双向帧，不连接官方 Provider。

## 会计安全策略与明确限制

未知使用量不按比例猜测，也不回退普通文本费率。未确认的输入/生成异常关闭后保留 `usage_confirmed: false`、模型、价格引用和完整价格快照；同 API Key 后续实时会话返回 HTTP 409，直到管理员核对供应商消费并补录确认的 UsageLog。当前没有自动对账或独立对账编辑界面，不能将未核对记录当成零费用。

持续 VAD 输入结束时，应停止采集、发送 `input_audio_buffer.clear` 并等待 `input_audio_buffer.cleared`，再关闭连接。只有得到上游确认的未生成输入清空才会撤销其预约；仅断开 socket 不会抹去未知消费。

OpenAI 的独立输入转录（`session.audio.input.transcription`）有单独计量和价格，当前明确拒绝开启，不能借 Realtime 主模型授权选择另一台付费转录模型。模型自身的音频理解及生成音频转录事件不受此限制。Qwen 声明支持的内置输入转录通过其独立适配器处理。

请求数在创建新审计预约时按既有已确认 UsageLog 的计数检查，运行中的 Token/费用配额单独核查，不因已预约的请求数达到上限而提前取消它自身。缓存明细缺失或使用了未配置价格的模态时停止会话并进入核对流程。全局费用仍属于实际用量核查，不提供供应商端硬金额预付锁。

计划扩展：独立转录计费、自动对账/人工对账管理页、浏览器有限权限短期令牌、经验证后开放 MCP/搜索/视频/多声道。上述能力不计入本期已验证清单。

进程崩溃或数据库收尾失败留下的 `processing` 预约也会检查其会话租约。没有活跃所有者的预约被视为待核对；元数据直接原子写入 Request，不受媒体请求体存储开关影响。Redis 模式可跨实例检查所有权，本地模式仅适用于单进程。

Qwen3.5 的一个响应若同时包含音频和 Function Calling 文本，而上游只有聚合文本 Token，无法证明哪些属于免费转录、哪些属于工具参数。该情况保留已观察到的 Token 并进入核对流程，不猜测拆分；可将工具调用阶段配置为纯文本输出，或优先使用 Qwen3.8/OpenAI。

同一渠道沿用一个凭证池。HTTP 和 Realtime 端点必须使用相匹配的官方密钥与地域；跨地域或跨凭证域应配置不同渠道，不能把中转服务密钥当成 OpenAI 官方密钥。端点测试及逐密钥测试均走实际 WebSocket 探测。
