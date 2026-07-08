from __future__ import annotations
import json, re
from pathlib import Path
ROOT=Path(__file__).resolve().parent
coverage=json.loads((ROOT/'code-field-coverage.json').read_text())
openai=json.loads((ROOT/'openai-fields.json').read_text())
anth=json.loads((ROOT/'anthropic-fields.json').read_text())
events=json.loads((ROOT/'openai-response-stream-event-types.json').read_text())

# Chinese concise meanings for top-level fields and event families.
ZH={
 'metadata':'元数据键值对，供请求/响应携带业务侧附加信息。',
 'top_logprobs':'返回每个输出 token 的候选 logprob 数量。',
 'temperature':'采样温度，控制随机性。',
 'top_p':'核采样概率阈值。',
 'user':'终端用户标识，旧字段，部分协议用来风控/缓存分桶。',
 'safety_identifier':'安全风控用稳定用户标识，替代 user 的部分用途。',
 'prompt_cache_key':'提示缓存分桶 key，用于提升缓存命中。',
 'service_tier':'服务层级/优先级容量选择。',
 'prompt_cache_retention':'提示缓存保留策略，例如内存或 24h。',
 'previous_response_id':'Responses 续接上一条 response 的 ID。',
 'model':'模型 ID。',
 'reasoning':'推理配置，例如 effort、summary、encrypted_content 等。',
 'background':'Responses 后台运行开关。',
 'max_tool_calls':'Responses 最大工具调用次数。',
 'text':'Responses 文本输出配置，例如格式/JSON schema。',
 'tools':'可供模型调用的工具定义。',
 'tool_choice':'工具选择策略。',
 'prompt':'Responses 存储提示模板引用/变量。',
 'truncation':'Responses 截断策略。',
 'input':'Responses 输入，可能是字符串或 typed input item 列表。',
 'include':'要求响应额外包含哪些输出数据，例如 logprobs、检索结果、图片 URL、encrypted reasoning。',
 'parallel_tool_calls':'是否允许并行工具调用。',
 'store':'是否存储输出供平台后续使用。',
 'instructions':'Responses 顶层指令。',
 'stream':'是否流式返回。',
 'stream_options':'流式返回配置。',
 'conversation':'Responses 服务端 conversation 挂载/续接。',
 'context_management':'Responses 上下文管理配置，例如 compaction 阈值。',
 'max_output_tokens':'Responses 最大输出 token 数。',
 'id':'对象唯一 ID。',
 'object':'对象类型标识。',
 'status':'Responses 状态。',
 'created_at':'创建时间戳。',
 'completed_at':'完成时间戳。',
 'error':'错误对象。',
 'incomplete_details':'未完成原因详情。',
 'output':'Responses 输出 item 列表。',
 'output_text':'Responses 便捷聚合文本输出。',
 'usage':'用量统计。',
 'messages':'Chat/Claude 消息数组。',
 'modalities':'输出模态列表，例如 text/audio/image。',
 'verbosity':'输出详细程度。',
 'reasoning_effort':'推理努力程度。',
 'max_completion_tokens':'Chat 最大 completion token 数，包含推理 token。',
 'frequency_penalty':'频率惩罚，降低重复 token。',
 'presence_penalty':'存在惩罚，鼓励新主题。',
 'web_search_options':'Chat 内置 web search 配置。',
 'response_format':'Chat 输出格式配置，例如 text/json_schema/json_object。',
 'audio':'Chat 顶层音频输出配置。',
 'stop':'停止序列配置。',
 'logit_bias':'对指定 token 的 logit 偏置。',
 'logprobs':'是否返回 logprob。',
 'max_tokens':'旧版最大输出 token 字段。',
 'n':'Chat 一次生成几个候选。',
 'prediction':'Chat 预测内容/静态内容提示，用于加速已知输出场景。',
 'seed':'采样随机种子。',
 'function_call':'旧版函数调用选择字段，已被 tool_choice 替代。',
 'functions':'旧版函数定义字段，已被 tools 替代。',
 'choices':'Chat 候选输出列表。',
 'created':'Chat 创建时间戳。',
 'system_fingerprint':'模型后端系统指纹。',
 'max_tokens_anthropic':'Claude 最大生成 token 数。',
 'container':'Claude container 标识/返回信息，用于代码执行等容器能力。',
 'inference_geo':'Claude 推理地理区域。',
 'output_config':'Claude 输出配置，例如 structured output schema、effort、task_budget。',
 'stop_sequences':'Claude 自定义停止序列。',
 'system':'Claude 顶层 system prompt。',
 'thinking':'Claude extended thinking 配置或 thinking 内容块。',
 'top_k':'Claude/部分 provider top-k 采样。',
 'role':'消息角色。',
 'content':'消息/响应内容块数组。',
 'stop_details':'Claude 结构化停止/拒绝详情。',
 'stop_reason':'Claude 停止原因。',
 'stop_sequence':'触发停止的具体序列。',
 'type':'对象或事件类型。',
 'mcp_servers':'Anthropic MCP connector 远程 MCP 服务器定义。',
 'tools[].type=mcp_toolset':'Anthropic MCP toolset 工具变体，引用 mcp_servers 中的服务器。',
 'mcp_servers[].name':'MCP server 名称。',
 'mcp_servers[].url':'MCP server URL。',
 'mcp_servers[].authorization_token':'MCP server 鉴权 token。',
 'mcp_servers[].tool_configuration':'MCP server 工具过滤/配置。',
}

COMMON={'model','metadata','temperature','top_p','user','safety_identifier','prompt_cache_key','service_tier','stream','stream_options','store','parallel_tool_calls','tools','tool_choice','reasoning','reasoning_effort','max_completion_tokens','max_tokens','frequency_penalty','presence_penalty','top_logprobs','logprobs','seed','stop','response_format','modalities','verbosity','messages'}
DEPRECATED={'function_call','functions','max_tokens'}
DESIGN_DROP={'n'}
RESPONSES_NATIVE={'input','instructions','include','previous_response_id','background','max_tool_calls','text','prompt','truncation','conversation','context_management','max_output_tokens','prompt_cache_retention'}
CHAT_NATIVE={'web_search_options','audio','prediction','prompt_cache_retention'}
ANTH_NATIVE={'max_tokens','messages','model','container','inference_geo','metadata','output_config','service_tier','stop_sequences','stream','system','temperature','thinking','tool_choice','tools','top_k','top_p'}
RESPONSE_ONLY={'id','object','status','created_at','completed_at','error','incomplete_details','output','output_text','usage','choices','created','system_fingerprint','role','content','stop_details','stop_reason','stop_sequence','type'}

def zhen(field, official=''):
    if field=='max_tokens' and 'anthropic' in official:
        return ZH['max_tokens_anthropic']
    return ZH.get(field, '') or brief_from_official(official)

def brief_from_official(s):
    s=re.sub(r'`[^`]+`','',s or '')
    s=re.sub(r'\s+',' ',s).strip()
    return s[:120] + ('…' if len(s)>120 else '')

def route(protocol, direction, field, upstream_top, current_top, any_tag=False):
    if direction=='stream':
        return 'stream 聚合/事件转换层：inbound_stream、outbound_stream、aggregator；不能放进 request struct。'
    if direction=='response':
        if upstream_top:
            return '协议 native response struct + TransformResponse/TransformStream。'
        return '补 native response 字段或在聚合层派生；若只是便捷字段可由 aggregator 生成。'
    if field in DESIGN_DROP:
        return '设计性不支持/可丢弃，但必须文档化；如要兼容需新增字段和测试。'
    if field in {'function_call','functions'}:
        return '旧版兼容字段：优先转换到 tools/tool_choice；不能表达时 raw same-protocol 或明确丢弃。'
    if protocol=='openai_chat':
        if field in COMMON and upstream_top:
            return '直接 common llm.Request ↔ Chat native struct。'
        if field in CHAT_NATIVE:
            return '补 Chat native request 字段；同协议保真；跨协议需 lossy diagnostic 或显式桥接。'
        if upstream_top:
            return 'Chat native struct；必要时映射 common 字段。'
    if protocol=='openai_responses':
        if field in RESPONSES_NATIVE:
            if upstream_top:
                return 'Responses native request struct + TransformerMetadata/ProviderExtensions 按需恢复。'
            return '补 Responses native/opaque request 字段；同协议保真；跨协议不静默映射。'
        if field in COMMON and upstream_top:
            return 'common llm.Request ↔ Responses native struct。'
        if upstream_top:
            return 'Responses native struct。'
    if protocol=='anthropic':
        if field in ANTH_NATIVE:
            if upstream_top:
                return 'Anthropic native MessageRequest；与 common 字段可桥接则桥接。'
            return '补 Anthropic native/opaque request 字段；同协议保真；跨协议诊断。'
        if field.startswith('mcp_servers') or field.startswith('tools[].type=mcp_toolset'):
            return 'Anthropic MCP connector companion：ProviderExtensions 或 Anthropic native raw 字段；不要自动映射成 OpenAI mcp。'
        if upstream_top:
            return 'Anthropic native struct。'
    if any_tag:
        return '已有嵌套/辅助 struct；需确认是否缺顶层入口或仅嵌套使用。'
    return '未建模：先判断是否官方字段；若是同协议 native，否则 lossy/drop。'

def drop_policy(protocol, direction, field):
    if direction=='stream':
        return '不能按 request 字段丢弃；缺事件处理会导致流式保真风险，应单独审计。'
    if field in DESIGN_DROP:
        return '可设计性丢弃：`n` 会改变返回多候选语义，Hub 当前统一模型偏单候选。'
    if field in {'function_call','functions'}:
        return '可在新版路径降级/丢弃，但应优先转换为 tools/tool_choice，并保留旧版兼容测试。'
    if protocol=='openai_chat' and field in {'web_search_options','prediction','audio','prompt_cache_retention'}:
        return '同协议不应丢；跨到不支持协议时诊断后丢弃或 provider-specific。'
    if protocol=='openai_responses' and field in {'conversation','context_management','prompt','include','previous_response_id','background','max_tool_calls'}:
        return '同协议不应丢；跨 Chat/Claude 无等价时 diagnostic + drop/bridge。'
    if protocol=='anthropic' and field in {'container','inference_geo','mcp_servers','tools[].type=mcp_toolset','output_config','thinking'}:
        return '同协议不应丢；跨协议无等价时 diagnostic + drop。'
    if direction=='response' and field in {'output_text','completed_at','container','stop_details'}:
        return '响应同协议应保留；若 common response 无等价，native response/metadata 保留。'
    return '公共字段不应丢；若目标协议不支持，必须记录 lossy。'

def protocol_key(matrix_key):
    if matrix_key.startswith('openai_chat'): return 'openai_chat'
    if matrix_key.startswith('openai_responses'): return 'openai_responses'
    return 'anthropic'

def direction_key(matrix_key):
    if 'request' in matrix_key: return 'request'
    if 'response' in matrix_key: return 'response'
    return 'stream'

lines=['# 字段中文含义与处理路径分类','','本文件是实现前的字段分类表：每个顶层 request/response 字段、stream/event 字段都要先决定走哪条路径，不能边改边猜。','','## 处理路径枚举','','| 路径 | 含义 | 适用场景 |','|---|---|---|',
'| Common / `llm.Request` | 进入统一公共请求模型 | 多协议都有稳定等价语义的字段 |',
'| Native struct | 进入对应协议 native request/response struct | 官方协议字段，同协议必须保真 |',
'| `TransformerMetadata` | 转换恢复提示/桥接元数据 | 字段不适合进 common，但转换后还要恢复 |',
'| `ProviderExtensions` | 协议私有 sidecar | 不应该序列化进 common JSON 的协议私有数据 |',
'| Raw fallback | 保存原始 JSON 片段 | known/unmodeled variant，同协议回放 |',
'| Provider-specific outbound | 具体 provider 出站层处理 | OpenAI-compatible 但并非所有 provider 都支持的字段 |',
'| Lossy diagnostic + drop | 记录损失后丢弃 | 目标协议无等价语义 |',
'| Deliberate unsupported | 设计性不支持 | 例如多候选 `n` 这类会改变 Hub 统一语义的字段 |',
'']

for key,val in coverage['matrices'].items():
    proto=protocol_key(key); direction=direction_key(key)
    lines += [f"## {val['title']} 字段分类", '', '| 字段 | 中文含义 | 作者 upstream | 当前分支 | 推荐处理路径 | 什么时候可丢/应诊断 |', '|---|---|---:|---:|---|---|']
    for r in val['rows']:
        field=r['field']
        meaning=zhen(field, r.get('meaning',''))
        rec=route(proto,direction,field,r['upstream_top'],r['current_top'],r['upstream_any'])
        drop=drop_policy(proto,direction,field)
        lines.append(f"| `{field}` | {meaning} | {'有' if r['upstream_top'] else '缺'} | {'有' if r['current_top'] else '缺'} | {rec} | {drop} |")
    lines.append('')

# Stream events with Chinese families
def event_meaning(protocol, event):
    if protocol=='openai_responses':
        ev=event
        if ev.startswith('response.audio'): return 'Responses 音频输出/转写流式事件。'
        if 'code_interpreter' in ev: return 'Responses code interpreter 工具调用状态/代码增量事件。'
        if 'mcp_' in ev or 'mcp.' in ev: return 'Responses MCP 工具调用/列工具状态事件。'
        if 'web_search' in ev: return 'Responses web search 工具调用状态事件。'
        if 'file_search' in ev: return 'Responses file search 工具调用状态事件。'
        if 'image_generation' in ev: return 'Responses image generation 工具调用状态/部分图像事件。'
        if 'reasoning' in ev: return 'Responses reasoning/summary 文本增量事件。'
        if 'output_text' in ev or 'text' in ev: return 'Responses 文本输出增量/完成事件。'
        if 'function_call' in ev: return 'Responses function call 参数增量/完成事件。'
        if 'custom_tool' in ev: return 'Responses custom tool 输入增量/完成事件。'
        if ev in {'response.created','response.in_progress','response.completed','response.failed','response.incomplete','response.queued'}: return 'Responses 生命周期状态事件。'
        return 'Responses stream 事件。'
    if protocol=='openai_chat': return 'Chat Completions 流式 chunk/schema。'
    return 'Anthropic Messages SSE 事件或 delta 类型。'

# Build corrected stream rows from official event types
resp_event_rows=[]
# event map stored as schema/event_type
for e in events:
    resp_event_rows.append(('openai_responses', e['event_type']))
for n in openai['openai_chat_stream_schemas']:
    resp_event_rows.append(('openai_chat', n))
for n in anth['anthropic_stream_events']+anth['anthropic_stream_delta_types']:
    resp_event_rows.append(('anthropic', n))

def present(root, proto, term):
    base=Path(root)
    rels=[]
    if proto=='openai_responses': rels=['llm/transformer/openai/responses/model.go','llm/transformer/openai/responses/inbound_stream.go','llm/transformer/openai/responses/outbound_stream.go','llm/transformer/openai/responses/aggregator.go']
    elif proto=='openai_chat': rels=['llm/transformer/openai/model.go','llm/transformer/openai/inbound_stream.go','llm/transformer/openai/outbound_stream.go','llm/transformer/openai/aggregator.go']
    else: rels=['llm/transformer/anthropic/model.go','llm/transformer/anthropic/inbound_stream.go','llm/transformer/anthropic/outbound_stream.go','llm/transformer/anthropic/aggregator.go']
    text=''
    for rel in rels:
        p=base/rel
        if p.exists(): text += p.read_text(errors='replace')+'\n'
    return term in text

lines += ['## Stream/Event 字段分类', '', '| 协议 | 事件/schema | 中文含义 | 作者 upstream 显式覆盖 | 当前分支显式覆盖 | 推荐处理路径 |', '|---|---|---|---:|---:|---|']
for proto,event in resp_event_rows:
    up=present('/tmp/axonhub-upstream-20260706-175405', proto, event)
    cur=present('/Users/asuan/项目/AI/axonhub', proto, event)
    lines.append(f"| `{proto}` | `{event}` | {event_meaning(proto,event)} | {'有' if up else '缺/泛化'} | {'有' if cur else '缺/泛化'} | stream 聚合/转换层处理；不能用 request 字段替代。 |")

lines += ['', '## Anthropic MCP connector companion 字段分类', '', '| 字段 | 中文含义 | 推荐处理路径 | 什么时候可丢/应诊断 |', '|---|---|---|---|']
for x in anth['anthropic_mcp_connector_fields']:
    f=x['field']; meaning=zhen(f,x.get('description',''))
    lines.append(f"| `{f}` | {meaning} | Anthropic native/provider extension/raw same-protocol；不要自动映射到 OpenAI Responses `mcp`。 | 同协议不应丢；跨协议无等价时 lossy diagnostic + drop。 |")

lines += ['', '## 作者一般丢弃的字段类型', '', '| 类型 | 作者当前表现 | 应不应该丢 |', '|---|---|---|',
'| 目标协议确实不支持的字段 | 例如 Chat builder 过滤非 function tool | 可以丢，但必须确认不是协议漂移；应加 lossy diagnostic |',
'| 旧版 deprecated 字段 | `function_call` / `functions` 未建模 | 可转换到新版 `tools/tool_choice`；不能转换时同协议 raw 或文档化丢弃 |',
'| 改变统一语义的字段 | `n` 多候选当前未支持 | 可以设计性不支持，但要明确不支持原因 |',
'| Provider 私有扩展 | 当前经常靠 metadata/raw 兜底 | 同协议不该丢；跨协议默认不透传 |',
'| Stream 细粒度事件 | OpenAI Responses 很多事件未显式覆盖 | 不应简单丢；要看 aggregator 是否泛化保真，否则补 stream 层 |',
'| 无等价服务端状态字段 | `conversation`、`previous_response_id`、Claude `container` 等 | 同协议保留，跨协议只能诊断/桥接，不应静默伪映射 |',
]

OUT=ROOT/'field-routing-classification.zh.md'
OUT.write_text('\n'.join(lines)+'\n')
print('wrote', OUT, 'lines', len(lines))
