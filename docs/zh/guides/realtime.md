# Realtime API 使用指南

AxonHub 提供面向 OpenAI Realtime GA 客户端的 WebSocket API，以及独立的 Qwen 原生 Realtime 入口。本指南介绍如何配置服务商渠道、选择客户端入口，并了解连接限制、用量和计费。

## 配置 Realtime 渠道

1. 保留现有 `openai` 或 `bailian` 渠道，不新建服务商类型。在渠道密钥管理中加入对应官方 API Key。
2. 在渠道主表单中为 OpenAI 选择 `openai/realtime-ws`，为百炼选择 `qwen/realtime-ws`。独立保存在渠道的 `settings.realtime`，包含 `enabled`、`protocol`、`baseURL`、`path` 和可选的 `handshakeTimeoutMs`、`clientEventsPerSecond`。握手超时可设为 1000–60000 毫秒，留空继承服务配置。客户端事件速率可设为每秒 256–4096 条，留空继承服务端默认值 1024。选择 Realtime 后，在现有的 **Base URL** 输入框中填写 WSS Host。AxonHub 将其保存在 `settings.realtime.baseURL`，并保留渠道原有的 HTTP Base URL。
3. OpenAI 官方使用 `wss://api.openai.com` 和 `/v1/realtime`。支持的型号包括 `gpt-realtime-2.1` 和 `gpt-realtime-2.1-mini`。
4. 百炼应从控制台复制所选业务空间的可信 API Host，使用其 WSS 地址，路径为 `/api-ws/v1/realtime`。Host 仅接受 `maas.aliyuncs.com` 下的 `cn-beijing` 和 `ap-southeast-1` 地域；保留控制台提供的完整主机标签，包括实际存在的 `llm-` 前缀。Workspace、地域、密钥必须属于同一官方资源配置。
5. 在渠道层配置支持模型白名单、模型映射、模型关联和 API Key Profile 授权。Qwen 首批型号为 `qwen3.8-omni-flash-realtime`、`qwen3.5-omni-plus-realtime`、`qwen3.5-omni-flash-realtime`。Realtime 协议选择属于渠道主表单，不通过端点管理设置。
6. 配置分项价格，不能用普通文本模型的 `prompt_tokens`/`completion_tokens` 代替实时价格。使用管理员维护的官方当前费率与币种，不能混合人民币与美元。
7. 渠道连接测试执行真实 WS 握手、会话配置确认及轻量 conversation item 确认，不触发生成。此探测成功不代表真实语音或账单验收通过。

端点管理不提供 Realtime 协议。通过 `saveChannelEndpoints` 或其他端点写入输入传入 `openai/realtime-ws`、`qwen/realtime-ws`，服务端会返回 HTTP `422` 和错误码 `ENDPOINT_REALTIME_NOT_ALLOWED`，不会忽略或保存。将 `settings.realtime.enabled` 设为 `false`，或在渠道主表单切换到非 Realtime 协议，即可停止新实时会话，无需删除 HTTP 端点。

渠道选择 Realtime 后只承接实时请求。原有 HTTP Base URL、自定义端点和模型配置继续保存在渠道中，端点管理在此模式下显示停用提示。切回普通协议后，默认端点和自定义端点恢复路由。此前混合使用的渠道如果两种流量都需要，请分别配置普通渠道和 Realtime 渠道。

Realtime 渠道配置适用于现有 OpenAI 和百炼渠道类型。xAI Subscription 使用服务商管理的端点，不接受自定义 Realtime 端点。

握手时，AxonHub 先应用 API Key Profile 渠道过滤、模型路由策略、渠道配额路由策略和已配置的负载均衡策略，再将整个会话固定到一个渠道。WebSocket 建立后不会在不同上游之间切换。

上游握手返回 HTTP 状态码时，会交给所选渠道的 API Key 自动禁用规则处理。使用 `priority` 密钥策略时，匹配规则而被禁用的凭证会让后续建连选择下一个可用密钥。Realtime 的 `circuit-breaker` 策略会记录上游握手失败、失败响应和已完成响应，供后续渠道选路使用。

现有启动数据迁移流程会在渠道缓存加载之前，把历史 `endpoints` 中的 Realtime 配置搬到 `settings.realtime`。每个渠道的配置搬迁与旧条目清理原子完成，保留普通端点及其他设置。已有渠道级 Realtime 设置优先，包括显式禁用状态；重复执行不再写入渠道。该迁移只改变 JSON 配置结构，不新增数据库表或列。

备份恢复使用同一转换逻辑处理合法的历史端点。启动时，无法转换的旧配置会保留原记录并记录警告，旧端点不会启用。恢复备份时，若其中包含无法转换的 Realtime 配置，恢复会拒绝写入并返回错误。

迁移遵循现有数据库自动迁移开关；若已关闭，需要先受控启用迁移启动一次，再开放迁移后的 Realtime 渠道。混合版本发布时先排空 Realtime 流量，完成全部实例升级后再开放，旧二进制不能读取新的渠道配置。

## 选择客户端入口

统一入口为 `GET /v1/realtime?model={网关模型或别名}`，使用标准 WebSocket Upgrade 和 `Authorization: Bearer {AxonHub API Key}`。只支持服务端客户端；浏览器 Origin、URL 凭证及客户端指定的上游 Host 均被拒绝。入口使用严格鉴权，不回退到 No-Auth。

原生 Qwen 入口为 `/qwen/v1/realtime?model={模型}`，同样经过权限、配额、价格与连接限制，仅能选择已启用 Realtime 配置的百炼渠道；它也只暴露已实现且通过校验的安全能力子集。

选择入口时按客户端使用的事件协议决定：标准 OpenAI Realtime GA 客户端使用 `/v1/realtime`；如果选中的模型路由到 Qwen，网关会将已支持的事件转换为 GA。只有客户端需要发送/接收 Qwen 原生事件名时，才使用 `/qwen/v1/realtime`；该入口仅连接已启用 Realtime 的百炼渠道，不经过 GA 事件转换。

下表中的 SDK Base URL 是 AxonHub 的公开地址，不是渠道配置中的上游 Realtime Host。按 SDK 自动追加的路径选择对应值，并确保 SDK 保留 Base URL 中的路径前缀。

| 客户端协议 | WebSocket 入口 | SDK Base URL 示例 | 适用场景 |
| --- | --- | --- | --- |
| OpenAI Realtime GA | `wss://gateway.example/v1/realtime?model={模型}` | SDK 追加 `/realtime` 时使用 `https://gateway.example/v1`；SDK 追加 `/v1/realtime` 时使用 `https://gateway.example` | 使用标准 OpenAI GA 事件；可路由到 OpenAI 上游，也可将 Qwen 模型转换为 GA 事件 |
| Qwen 原生 Realtime | `wss://gateway.example/qwen/v1/realtime?model={模型}` | SDK 追加 `/realtime` 时使用 `https://gateway.example/qwen/v1`；SDK 追加 `/v1/realtime` 时使用 `https://gateway.example/qwen` | 客户端使用 Qwen 原生事件名，只路由到百炼 Qwen 上游 |

两个入口都使用 AxonHub API Key 的 `Authorization: Bearer ...` Header。若客户端不能保留 Base URL 的路径前缀，需改用支持自定义完整 WebSocket URL 的客户端，并直接连接上表中的 WebSocket 入口。

OpenAI 上游使用原生 GA 事件，保留合法未知扩展字段和事件顺序，不发送 `OpenAI-Beta: realtime=v1`，不经 Qwen 转换，不对音频重采样。OpenAI 在 `session.created` 提供完整有效音频配置时可直接使用已确认的默认值；修改参数时应发送 `session.update` 并等待 `session.updated`，再按新参数发送媒体。禁止在会话中修改模型、覆盖凭证或身份安全字段。Safety Identifier 由已鉴权的网关密钥及用户/项目生成不可逆摘要，不接受客户端覆盖。

Qwen 实时模型支持 `server_vad`；Qwen3.8 Omni Flash、Qwen3.5 Omni Plus 和 Qwen3.5 Omni Flash 也支持 `semantic_vad`。语义 VAD 的阈值范围为 -1 到 1，静音时长为 200 到 6000 毫秒；其他模型使用其能力表声明的 VAD 类型。

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

## 了解用量、价格与配额

在发送音频或 `response.create` 前预约仅含元数据的请求记录，随后将 `response.created` 关联到该记录；`response.done` 的确认 usage 恰好记账一次。会话可包含多条独立记录，每会话最多 512 条审计预约，已完成、已清空和在途记录均计入该边界，去重集合也有界。异常关闭保留已经确认的用量，没有最终 usage 的响应记为失败且 `usage_confirmed: false`，不生成伪造零用量账单。

实时价格独立于历史价格项，覆盖输入文本、输入音频、输出文本、输出音频，以及 OpenAI 的输入图像、缓存文本/音频/图像。缓存是各输入模态的子集，必须从对应非缓存数量中扣除。计算使用 decimal；精确 CostItem 小计及价格来源用于审计。沿用历史 totalCost 字段的数值存储兼容边界，不改变旧账单。

Realtime 的语音模态允许部分费率：基础价格或生效的价格计划覆盖中，配置了哪些正数语音费率，就只按这些费率计算相应 CostItem；未配置的语音模态仍保留在 UsageLog 总用量中，但不计入费用。OpenAI 原生 Realtime 若配置了任意 Realtime 费率，其生效价格仍须完整包含输入图像、缓存输入文本、缓存输入音频和缓存输入图像四项；语音/音频费率可部分配置。完全没有 Realtime 费率时仍允许调用，记录用量而将费用留空。Realtime Translation 使用独立的音频样本按分钟费率。

缺少语音费率不会阻止路由或上游 WebSocket 建立。若 OpenAI 的图像/缓存费率缺项，仍会在路由阶段拒绝该价格配置。部分计费意味着账单只覆盖已配置语音费率的模态；在模型价格页核对 UsageLog 的总用量和 CostItem 分项，避免把未配置费率的模态误认为供应商没有产生用量。

Qwen3.8 的语音及对应文本按各自已配置的官方输出费率计费；Qwen3.5 语音输出对应文本通过型号计费规则保留实际 Token，并将对应文本计费数量记为 0；纯文本及工具参数仍按文本费率计算，不能套用 Qwen3.8 的费率。会话建立时冻结价格来源；没有 Realtime 费率时费用留空，有部分费率时只对已定价的模态收费。

在建立连接前检查配额，并在会话中周期检查及阻止后续输入；VAD 自主生成期间也检查。它是使用量核查，不是供应商端硬费用预付锁：一条已经生成的响应可能在最终 usage 确认前超过余额。灰度前必须按业务预算确认这一边界。

## 配置连接限制和代理

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

配置使用现有 YAML 与环境变量加载机制，例如 `AXONHUB_SERVER_REALTIME_MAX_DURATION=15m`（具体前缀以现有部署配置为准）。没有另一个必填 Feature Flag；渠道中显式设置 `settings.realtime.enabled` 即为启用条件。

共享 Redis 从现有 `cache.redis` 配置读取，在第一次实时会话时延迟连接。四个维度的并发与每密钥建连频率在单槽 Lua 中原子获取，租约超过会话硬时限，崩溃自动到期；Redis 不可用时拒绝新会话。没有 Redis 时计数仅适用于单进程；多实例部署必须使用相同 Redis 和相同限制，不可称本地计数为集群配额。Redis 的数据持久化、主从切换丢写风险必须纳入部署验收。

单消息上限 256 KiB，音频块解码上限 64 KiB，每方向发送队列最多 8 条。客户端事件速率限制与 2 MiB/s 的消息字节保护分别生效，事件超限时用 `1008 client_rate_limited` 关闭。队列满时关闭而非丢弃媒体。服务端分别记录两个 WebSocket 对端的活动，包括 ping/pong；上游 pong 不会延长无响应客户端的会话，单端等待期限会给心跳最多两个间隔的 pong 时间。两端都空闲时，按配置的空闲时限用 `1000 idle_timeout` 关闭。握手、配置确认和总时长也有边界。停止服务时取消现有会话并关闭两端，不无限等待长连接。

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

## 测试并回滚

使用渠道的**测试**操作检查上游握手和会话配置。该探测不会生成语音，也不能验证音质。正式接入前，用客户端 SDK 分别测试各型号，确认音频可以播放，并将记录用量与供应商账单核对。

先备份数据库和渠道配置，在单个受限项目启用渠道的 Realtime 配置，设低并发/短时长/小配额，核对供应商账单再扩大。回滚先关闭 `settings.realtime.enabled` 或禁用相应渠道，等待有界会话结束或停止服务取消连接，再回滚应用版本。迁移不新增表或列，但 JSON 配置结构已变化：只读取旧实时端点的应用不会使用 `settings.realtime`。若旧版本仍需提供 Realtime，切换版本后恢复与其匹配的渠道配置备份，不要通过当前端点 API 写回实时协议。运行旧代码前检查价格枚举兼容性，保留已经确认的账单审计记录。

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

请通过反向代理的公开地址测试 WebSocket。HTTP 健康检查只能确认 AxonHub 进程运行，不能验证代理是否正确转发 WebSocket Upgrade。

## 处理未确认的用量

未知使用量不按比例猜测，也不回退普通文本费率。未确认的输入/生成异常关闭后保留 `usage_confirmed: false`、模型、价格引用和完整价格快照。缺少 UsageLog 表示消费尚未确认，不等于确认欠费，不会永久阻止该 Key 建立后续会话；新连接仍受并发预约和已确认消费配额约束。不能将未核对记录当成零费用。

实例所有者可以使用本人管理员 JWT，通过 `POST /admin/realtime/reconcile` 将终态且未定价的审计记录标记为已审核。Authorization Header 使用常规管理员 JWT；请求示例为 `{"apiKeyID":123,"requestIDs":[456],"note":"已核查供应商消费，暂无可确认计费量"}`。省略 `requestIDs` 时审核该 Key 最多 500 条匹配记录。响应包含 `reviewedCount` 和 `usageOrBalanceChanged: false`；此操作仅补充审核人、时间和备注，不创建 UsageLog、不虚构零消耗、不修改余额，可安全重试。API Key、插件及非所有者用户无权执行此操作。当前没有供应商自动对账或独立编辑页面。

持续 VAD 输入结束时，应停止采集、发送 `input_audio_buffer.clear` 并等待 `input_audio_buffer.cleared`，再关闭连接。只有得到上游确认的未生成输入清空才会撤销其预约；仅断开 socket 不会抹去未知消费。

OpenAI 的独立输入转录（`session.audio.input.transcription`）有单独计量和价格，当前未启用。网关转发会话更新前会移除 SDK 提供的转录设置。会话更新中的 `session.model` 必须与建连模型一致，网关接受后会移除该字段。模型自身的音频理解及生成音频转录事件不受此限制。Qwen 声明支持的内置输入转录通过其独立适配器处理。

请求数在创建新审计预约时按既有已确认 UsageLog 的计数检查，运行中的 Token/费用配额单独核查，不因已预约的请求数达到上限而提前取消它自身。无法验证的 usage 明细仍按既有校验进入核对流程；仅缺少某个模态的价格不阻止会话，该模态不计入费用。全局费用仍属于实际用量核查，不提供供应商端硬金额预付锁。

会话并发由租约限制；配置 Redis 时可跨实例共享，未配置时仅在单进程内生效。审计元数据保存在请求记录中，不受媒体/正文存储设置影响。

Qwen3.5 的一个响应若同时包含音频和 Function Calling 文本，而上游只有聚合文本 Token，无法证明哪些属于免费转录、哪些属于工具参数。该情况保留已观察到的 Token 并进入核对流程，不猜测拆分；可将工具调用阶段配置为纯文本输出，或优先使用 Qwen3.8/OpenAI。

同一渠道沿用一个凭证池。HTTP 配置和渠道级 Realtime 配置必须使用相匹配的官方密钥与地域；跨地域或跨凭证域应配置不同渠道，不能把中转服务密钥当成 OpenAI 官方密钥。渠道连接测试及逐密钥测试均走实际 WebSocket 探测。

## 协议行为与支持范围

OpenAI 原生 Adapter 对每个有效 JSON 消息返回相同字节，包括字段顺序与错误消息；客户端的 session.update、response.create、input_audio_buffer.clear 或 input_audio_buffer.commit 缺少事件 ID 或 ID 为空时，网关会补上 ID，以便把上游拒绝关联到该次客户端事件。网关也会对上游 error 及 status_details.error 脱敏。WebSocket 传输保留 JSON 消息边界，不承诺保留底层 RFC6455 分片方式或 TCP 包边界。

## 配置 OpenAI Realtime 翻译

在现有 OpenAI 渠道启用 Realtime，配置上游模型 `gpt-realtime-translate`，在模型价格中填写「实时翻译音频采样数（按分钟计价）」并选择按量计费。客户端使用网关 API Key 连接 `/v1/realtime/translations?model=gpt-realtime-translate`，用 `session.update` 设置 `session.audio.output.language`，等 `session.updated` 后通过 `session.input_audio_buffer.append` 流式发送 24 kHz 单声道 PCM16。结束时发送 `session.close`，读取剩余译文及音频直至收到 `session.closed`。

上游未在会话事件提供账单用量。网关按成功写到上游的输入 PCM 样本数，以冻结的每分钟费率计算费用，即使连接中途中断也保留已发送部分。审计标记计量来源为 `gateway_sent_pcm`，费用明细的数量是样本数。标准对话入口 `/v1/realtime` 不接收该翻译模型。
