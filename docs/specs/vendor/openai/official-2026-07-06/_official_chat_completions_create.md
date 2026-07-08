# Create chat completion | OpenAI API Reference
[Skip to content](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#_top)

[![Image 1: OpenAI Developers](https://developers.openai.com/OpenAI_Developers.svg)](https://developers.openai.com/)

[Home](https://developers.openai.com/)

[API](https://developers.openai.com/api)

[Docs Guides and concepts for the OpenAI API](https://developers.openai.com/api/docs)[API reference Endpoints, parameters, and responses](https://developers.openai.com/api/reference/overview)

[Codex](https://developers.openai.com/codex)

[Docs Guides, concepts, and product docs for Codex](https://developers.openai.com/codex)[Use cases Example workflows and tasks teams hand to Codex](https://developers.openai.com/codex/use-cases)

[ChatGPT](https://developers.openai.com/chatgpt)

[Apps SDK Build apps to extend ChatGPT](https://developers.openai.com/apps-sdk)[Workspace Agents Trigger published ChatGPT workspace agents](https://developers.openai.com/workspace-agents)[Commerce Build commerce flows in ChatGPT](https://developers.openai.com/commerce)[Ads Publish and measure ads in ChatGPT](https://developers.openai.com/ads)

[Resources](https://developers.openai.com/learn)

[Showcase Demo apps to get inspired](https://developers.openai.com/showcase)[Blog Learnings and experiences from developers](https://developers.openai.com/blog)[Cookbook Notebook examples for building with OpenAI models](https://developers.openai.com/cookbook)[Learn Docs, videos, and demo apps for building with OpenAI](https://developers.openai.com/learn)[Community Programs, meetups, and support for builders](https://developers.openai.com/community)

Start searching

[API Dashboard](https://platform.openai.com/login)

## Search the API docs

Search docs 

### Suggested

responses create reasoning_effort realtime prompt caching

Primary navigation

 API  API Reference  Codex  ChatGPT  Resources 

Search docs 

### Suggested

responses create reasoning_effort realtime prompt caching

### Get started

*   [Overview](https://developers.openai.com/api/docs)
*   [Quickstart](https://developers.openai.com/api/docs/quickstart)
*   [Models](https://developers.openai.com/api/docs/models)
*   [Pricing](https://developers.openai.com/api/docs/pricing)
*   
[SDKs and CLI](https://developers.openai.com/api/docs/libraries)
    *   [OpenAI SDK](https://developers.openai.com/api/docs/libraries)
    *   [Agents SDK](https://developers.openai.com/api/docs/guides/agents)
    *   [OpenAI CLI](https://developers.openai.com/api/docs/libraries/openai-cli)

*   [Latest: GPT-5.5](https://developers.openai.com/api/docs/guides/latest-model)
*   [Prompt guidance](https://developers.openai.com/api/docs/guides/prompt-guidance)

### Core concepts

*   [Text generation](https://developers.openai.com/api/docs/guides/text)
*   [Code generation](https://developers.openai.com/api/docs/guides/code-generation)
*   [Images and vision](https://developers.openai.com/api/docs/guides/images-vision)
*   [Audio and speech](https://developers.openai.com/api/docs/guides/audio)
*   [Structured output](https://developers.openai.com/api/docs/guides/structured-outputs)
*   [Function calling](https://developers.openai.com/api/docs/guides/function-calling)
*   [Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses)
*   [Using tools](https://developers.openai.com/api/docs/guides/tools)

### Agents SDK

*   [Overview](https://developers.openai.com/api/docs/guides/agents)
*   [Quickstart](https://developers.openai.com/api/docs/guides/agents/quickstart)
*   [Agent definitions](https://developers.openai.com/api/docs/guides/agents/define-agents)
*   [Models and providers](https://developers.openai.com/api/docs/guides/agents/models)
*   [Running agents](https://developers.openai.com/api/docs/guides/agents/running-agents)
*   [Sandbox agents](https://developers.openai.com/api/docs/guides/agents/sandboxes)
*   [Orchestration](https://developers.openai.com/api/docs/guides/agents/orchestration)
*   [Guardrails](https://developers.openai.com/api/docs/guides/agents/guardrails-approvals)
*   [Results and state](https://developers.openai.com/api/docs/guides/agents/results)
*   [Integrations and observability](https://developers.openai.com/api/docs/guides/agents/integrations-observability)
*   [Evaluate agent workflows](https://developers.openai.com/api/docs/guides/agent-evals)
*   [Voice agents](https://developers.openai.com/api/docs/guides/voice-agents)
*   
ChatKit
    *   [Overview](https://developers.openai.com/api/docs/guides/chatkit)
    *   [Customize](https://developers.openai.com/api/docs/guides/chatkit-themes)
    *   [Widgets](https://developers.openai.com/api/docs/guides/chatkit-widgets)
    *   [Actions](https://developers.openai.com/api/docs/guides/chatkit-actions)
    *   [Advanced integrations](https://developers.openai.com/api/docs/guides/custom-chatkit)

### Tools

*   [Web search](https://developers.openai.com/api/docs/guides/tools-web-search)
*   
[MCP and Connectors](https://developers.openai.com/api/docs/guides/tools-connectors-mcp)
    *   [Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)

*   [Skills](https://developers.openai.com/api/docs/guides/tools-skills)
*   [Shell](https://developers.openai.com/api/docs/guides/tools-shell)
*   [Computer use](https://developers.openai.com/api/docs/guides/tools-computer-use)
*   
File search and retrieval
    *   [File search](https://developers.openai.com/api/docs/guides/tools-file-search)
    *   [Retrieval](https://developers.openai.com/api/docs/guides/retrieval)

*   [Tool search](https://developers.openai.com/api/docs/guides/tools-tool-search)
*   
More tools
    *   [Apply Patch](https://developers.openai.com/api/docs/guides/tools-apply-patch)
    *   [Local shell](https://developers.openai.com/api/docs/guides/tools-local-shell)
    *   [Image generation](https://developers.openai.com/api/docs/guides/tools-image-generation)
    *   [Code interpreter](https://developers.openai.com/api/docs/guides/tools-code-interpreter)

### Run and scale

*   [Conversation state](https://developers.openai.com/api/docs/guides/conversation-state)
*   [Background mode](https://developers.openai.com/api/docs/guides/background)
*   [Streaming](https://developers.openai.com/api/docs/guides/streaming-responses)
*   [WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode)
*   [Webhooks](https://developers.openai.com/api/docs/guides/webhooks)
*   [File inputs](https://developers.openai.com/api/docs/guides/file-inputs)
*   
Context management
    *   [Compaction](https://developers.openai.com/api/docs/guides/compaction)
    *   [Counting tokens](https://developers.openai.com/api/docs/guides/token-counting)
    *   [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching)

*   
Prompting
    *   [Overview](https://developers.openai.com/api/docs/guides/prompting)
    *   [Prompt engineering](https://developers.openai.com/api/docs/guides/prompt-engineering)
    *   [Citation formatting](https://developers.openai.com/api/docs/guides/citation-formatting)
    *   [Migration guide](https://developers.openai.com/api/docs/guides/prompting/migrate-from-prompt-object)

*   
Reasoning
    *   [Reasoning models](https://developers.openai.com/api/docs/guides/reasoning)
    *   [Reasoning best practices](https://developers.openai.com/api/docs/guides/reasoning-best-practices)

### Evaluation

*   [Red teaming](https://developers.openai.com/api/docs/guides/red-teaming)

### Realtime and audio

*   [Overview](https://developers.openai.com/api/docs/guides/realtime)
*   [Voice agents](https://developers.openai.com/api/docs/guides/voice-agents)
*   [Live translation](https://developers.openai.com/api/docs/guides/realtime-translation)
*   
Transcription
    *   [Realtime transcription](https://developers.openai.com/api/docs/guides/realtime-transcription)
    *   [Speech to text](https://developers.openai.com/api/docs/guides/speech-to-text)

*   [Speech generation](https://developers.openai.com/api/docs/guides/text-to-speech)
*   [Realtime prompting guide](https://developers.openai.com/api/docs/guides/realtime-models-prompting)
*   
Connection methods
    *   [WebRTC](https://developers.openai.com/api/docs/guides/realtime-webrtc)
    *   [WebSocket](https://developers.openai.com/api/docs/guides/realtime-websocket)
    *   [SIP](https://developers.openai.com/api/docs/guides/realtime-sip)

*   
Realtime sessions
    *   [Managing conversations](https://developers.openai.com/api/docs/guides/realtime-conversations)
    *   [Voice activity detection](https://developers.openai.com/api/docs/guides/realtime-vad)
    *   [Realtime with tools](https://developers.openai.com/api/docs/guides/realtime-mcp)
    *   [Webhooks and server-side controls](https://developers.openai.com/api/docs/guides/realtime-server-controls)
    *   [Managing costs](https://developers.openai.com/api/docs/guides/realtime-costs)

### Specialized models

*   [Image generation](https://developers.openai.com/api/docs/guides/image-generation)
*   [Video generation](https://developers.openai.com/api/docs/guides/video-generation)
*   [Deep research](https://developers.openai.com/api/docs/guides/deep-research)
*   [Embeddings](https://developers.openai.com/api/docs/guides/embeddings)
*   [Moderation](https://developers.openai.com/api/docs/guides/moderation)

### Going live

*   [Production best practices](https://developers.openai.com/api/docs/guides/production-best-practices)
*   
[Workload identity federation](https://developers.openai.com/api/docs/guides/workload-identity-federation)
    *   [Overview](https://developers.openai.com/api/docs/guides/workload-identity-federation)
    *   [Kubernetes](https://developers.openai.com/api/docs/guides/workload-identity-federation/kubernetes)
    *   [AWS](https://developers.openai.com/api/docs/guides/workload-identity-federation/aws)
    *   [Microsoft Azure](https://developers.openai.com/api/docs/guides/workload-identity-federation/microsoft-azure)
    *   [Google Cloud](https://developers.openai.com/api/docs/guides/workload-identity-federation/google-cloud)
    *   [GitHub Actions](https://developers.openai.com/api/docs/guides/workload-identity-federation/github-actions)
    *   [SPIFFE](https://developers.openai.com/api/docs/guides/workload-identity-federation/spiffe)

*   [Deployment checklist](https://developers.openai.com/api/docs/guides/deployment-checklist)
*   [Amazon Bedrock](https://developers.openai.com/api/docs/guides/amazon-bedrock)
*   
Latency optimization
    *   [Overview](https://developers.openai.com/api/docs/guides/latency-optimization)
    *   [Predicted Outputs](https://developers.openai.com/api/docs/guides/predicted-outputs)
    *   [Priority processing](https://developers.openai.com/api/docs/guides/priority-processing)

*   
Cost optimization
    *   [Overview](https://developers.openai.com/api/docs/guides/cost-optimization)
    *   [Batch](https://developers.openai.com/api/docs/guides/batch)
    *   [Flex processing](https://developers.openai.com/api/docs/guides/flex-processing)

*   [Accuracy optimization](https://developers.openai.com/api/docs/guides/optimizing-llm-accuracy)
*   
Safety
    *   [Safety best practices](https://developers.openai.com/api/docs/guides/safety-best-practices)
    *   [Safety checks](https://developers.openai.com/api/docs/guides/safety-checks)
    *   [Cybersecurity checks](https://developers.openai.com/api/docs/guides/safety-checks/cybersecurity)
    *   [Under 18 API Guidance](https://developers.openai.com/api/docs/guides/safety-checks/under-18-api-guidance)

### Legacy APIs

*   
Agent Builder
    *   [Overview](https://developers.openai.com/api/docs/guides/agent-builder)
    *   [Migration guide](https://developers.openai.com/api/docs/guides/agent-builder/migrate-from-agent-builder)
    *   [Node reference](https://developers.openai.com/api/docs/guides/node-reference)
    *   [Safety in building agents](https://developers.openai.com/api/docs/guides/agent-builder-safety)

*   
Evals
    *   [Getting started](https://developers.openai.com/api/docs/guides/evaluation-getting-started)
    *   [Working with evals](https://developers.openai.com/api/docs/guides/evals)
    *   [Prompt optimizer](https://developers.openai.com/api/docs/guides/prompt-optimizer)
    *   [External models](https://developers.openai.com/api/docs/guides/external-models)
    *   [Best practices](https://developers.openai.com/api/docs/guides/evaluation-best-practices)
    *   [Graders](https://developers.openai.com/api/docs/guides/graders)

*   
Fine-tuning
    *   [Optimization cycle](https://developers.openai.com/api/docs/guides/model-optimization)
    *   [Supervised fine-tuning](https://developers.openai.com/api/docs/guides/supervised-fine-tuning)
    *   [Vision fine-tuning](https://developers.openai.com/api/docs/guides/vision-fine-tuning)
    *   [Direct preference optimization](https://developers.openai.com/api/docs/guides/direct-preference-optimization)
    *   [Reinforcement fine-tuning](https://developers.openai.com/api/docs/guides/reinforcement-fine-tuning)
    *   [RFT use cases](https://developers.openai.com/api/docs/guides/rft-use-cases)
    *   [Best practices](https://developers.openai.com/api/docs/guides/fine-tuning-best-practices)

*   
Assistants API
    *   [Migration guide](https://developers.openai.com/api/docs/assistants/migration)
    *   [Deep dive](https://developers.openai.com/api/docs/assistants/deep-dive)
    *   [Tools](https://developers.openai.com/api/docs/assistants/tools)

### Resources

*   [Terms and policies](https://openai.com/policies)
*   [Changelog](https://developers.openai.com/api/docs/changelog)
*   [Your data](https://developers.openai.com/api/docs/guides/your-data)
*   [Permissions](https://developers.openai.com/api/docs/guides/rbac)
*   [Rate limits](https://developers.openai.com/api/docs/guides/rate-limits)
*   [IP egress ranges](https://developers.openai.com/api/docs/guides/ip-addresses)
*   [Admin APIs](https://developers.openai.com/api/docs/guides/admin-apis)
*   [Deprecations](https://developers.openai.com/api/docs/deprecations)
*   [MCP for deep research](https://developers.openai.com/api/docs/mcp)
*   [Developer mode](https://developers.openai.com/api/docs/guides/developer-mode)
*   
ChatGPT Actions
    *   [Introduction](https://developers.openai.com/api/docs/actions/introduction)
    *   [Getting started](https://developers.openai.com/api/docs/actions/getting-started)
    *   [Actions library](https://developers.openai.com/api/docs/actions/actions-library)
    *   [Authentication](https://developers.openai.com/api/docs/actions/authentication)
    *   [Production](https://developers.openai.com/api/docs/actions/production)
    *   [Data retrieval](https://developers.openai.com/api/docs/actions/data-retrieval)
    *   [Sending files](https://developers.openai.com/api/docs/actions/sending-files)

 Docs  Use cases 

### Getting Started

*   [Overview](https://developers.openai.com/codex)
*   [Quickstart](https://developers.openai.com/codex/quickstart)
*   [Explore use cases](https://developers.openai.com/codex/use-cases)
*   [Import to Codex](https://developers.openai.com/codex/import)
*   [Pricing](https://developers.openai.com/codex/pricing)
*   
Concepts
    *   [Prompting](https://developers.openai.com/codex/prompting)
    *   [Customization](https://developers.openai.com/codex/concepts/customization)
    *   
[Memories](https://developers.openai.com/codex/memories)
        *   [Chronicle](https://developers.openai.com/codex/memories/chronicle)

    *   
[Sandboxing](https://developers.openai.com/codex/concepts/sandboxing)
        *   [Auto-review](https://developers.openai.com/codex/concepts/sandboxing/auto-review)

    *   [Subagents](https://developers.openai.com/codex/concepts/subagents)
    *   [Workflows](https://developers.openai.com/codex/workflows)
    *   [Models](https://developers.openai.com/codex/models)
    *   [Cyber Safety](https://developers.openai.com/codex/concepts/cyber-safety)
    *   [Glossary](https://developers.openai.com/codex/glossary)

### Using Codex

*   
App
    *   [Overview](https://developers.openai.com/codex/app)
    *   [Features](https://developers.openai.com/codex/app/features)
    *   [Settings](https://developers.openai.com/codex/app/settings)
    *   [Review](https://developers.openai.com/codex/app/review)
    *   [Automations](https://developers.openai.com/codex/app/automations)
    *   [Worktrees](https://developers.openai.com/codex/app/worktrees)
    *   [Local Environments](https://developers.openai.com/codex/app/local-environments)
    *   [In-app browser](https://developers.openai.com/codex/app/browser)
    *   [Chrome extension](https://developers.openai.com/codex/app/chrome-extension)
    *   [Computer Use](https://developers.openai.com/codex/app/computer-use)
    *   [Appshots](https://developers.openai.com/codex/appshots)
    *   [Commands](https://developers.openai.com/codex/app/commands)
    *   [Windows](https://developers.openai.com/codex/app/windows)
    *   [Troubleshooting](https://developers.openai.com/codex/app/troubleshooting)

*   
IDE Extension
    *   [Overview](https://developers.openai.com/codex/ide)
    *   [Features](https://developers.openai.com/codex/ide/features)
    *   [Settings](https://developers.openai.com/codex/ide/settings)
    *   [IDE Commands](https://developers.openai.com/codex/ide/commands)
    *   [Slash commands](https://developers.openai.com/codex/ide/slash-commands)

*   
CLI
    *   [Overview](https://developers.openai.com/codex/cli)
    *   [Features](https://developers.openai.com/codex/cli/features)
    *   [Command Line Options](https://developers.openai.com/codex/cli/reference)
    *   [Slash commands](https://developers.openai.com/codex/cli/slash-commands)

*   
Web
    *   [Overview](https://developers.openai.com/codex/cloud)
    *   [Environments](https://developers.openai.com/codex/cloud/environments)
    *   [Internet Access](https://developers.openai.com/codex/cloud/internet-access)

*   
Integrations
    *   [GitHub](https://developers.openai.com/codex/integrations/github)
    *   [Slack](https://developers.openai.com/codex/integrations/slack)
    *   [Linear](https://developers.openai.com/codex/integrations/linear)

*   
Codex Security
    *   [Overview](https://developers.openai.com/codex/security)
    *   [Codex Security plugin](https://developers.openai.com/codex/security/plugin)
    *   
Codex Security cloud
        *   [Setup](https://developers.openai.com/codex/security/setup)
        *   [Improving the threat model](https://developers.openai.com/codex/security/threat-model)

    *   [FAQ](https://developers.openai.com/codex/security/faq)

### Configuration

*   
Config File
    *   [Config Basics](https://developers.openai.com/codex/config-basic)
    *   [Advanced Config](https://developers.openai.com/codex/config-advanced)
    *   [Config Reference](https://developers.openai.com/codex/config-reference)
    *   [Environment Variables](https://developers.openai.com/codex/environment-variables)
    *   [Sample Config](https://developers.openai.com/codex/config-sample)

*   [Permissions](https://developers.openai.com/codex/permissions)
*   [Speed](https://developers.openai.com/codex/speed)
*   [Rules](https://developers.openai.com/codex/rules)
*   [Hooks](https://developers.openai.com/codex/hooks)
*   [AGENTS.md](https://developers.openai.com/codex/guides/agents-md)
*   [MCP](https://developers.openai.com/codex/mcp)
*   
Plugins
    *   [Overview](https://developers.openai.com/codex/plugins)
    *   [Build plugins](https://developers.openai.com/codex/plugins/build)

*   [Sites](https://developers.openai.com/codex/sites)
*   [Skills](https://developers.openai.com/codex/skills)
*   [Subagents](https://developers.openai.com/codex/subagents)

### Administration

*   
Authentication
    *   [Overview](https://developers.openai.com/codex/auth)
    *   [Access tokens](https://developers.openai.com/codex/enterprise/access-tokens)

*   [Agent approvals & security](https://developers.openai.com/codex/agent-approvals-security)
*   [Remote connections](https://developers.openai.com/codex/remote-connections)
*   
Deployment
    *   [Amazon Bedrock](https://developers.openai.com/codex/amazon-bedrock)

*   
Enterprise
    *   [Admin Setup](https://developers.openai.com/codex/enterprise/admin-setup)
    *   [Governance](https://developers.openai.com/codex/enterprise/governance)
    *   [Managed configuration](https://developers.openai.com/codex/enterprise/managed-configuration)

*   [Windows](https://developers.openai.com/codex/windows)

### Automation

*   [Non-interactive Mode](https://developers.openai.com/codex/noninteractive)
*   [Codex SDK](https://developers.openai.com/codex/sdk)
*   [App Server](https://developers.openai.com/codex/app-server)
*   [MCP Server](https://developers.openai.com/codex/guides/agents-sdk)
*   [GitHub Action](https://developers.openai.com/codex/github-action)

### Learn

*   [Best practices](https://developers.openai.com/codex/learn/best-practices)
*   [Videos](https://developers.openai.com/codex/videos)
*   [Community](https://developers.openai.com/community)
*   
Blog
    *   [Using skills to accelerate OSS maintenance](https://developers.openai.com/blog/skills-agents-sdk)
    *   [Building frontend UIs with Codex and Figma](https://developers.openai.com/blog/building-frontend-uis-with-codex-and-figma)
    *   [View all](https://developers.openai.com/blog/topic/codex)

*   
Cookbooks
    *   [Build an Agent Improvement Loop with Traces, Evals, and Codex](https://developers.openai.com/cookbook/examples/agents_sdk/agent_improvement_loop)
    *   [Build iterative repair loops with Codex](https://developers.openai.com/cookbook/examples/codex/build_iterative_repair_loops_with_codex)
    *   [View all](https://developers.openai.com/cookbook/topic/codex)

*   [Building AI Teams](https://developers.openai.com/codex/guides/build-ai-native-engineering-team)

### Releases

*   [Changelog](https://developers.openai.com/codex/changelog)
*   [Feature Maturity](https://developers.openai.com/codex/feature-maturity)
*   [Open Source](https://developers.openai.com/codex/open-source)

*   [Home](https://developers.openai.com/codex/use-cases)
*   [Collections](https://developers.openai.com/codex/use-cases/collections)

 Apps SDK  Workspace Agents  Commerce  Ads 

*   [Home](https://developers.openai.com/apps-sdk)
*   [Quickstart](https://developers.openai.com/apps-sdk/quickstart)

### Core Concepts

*   [MCP Apps in ChatGPT](https://developers.openai.com/apps-sdk/mcp-apps-in-chatgpt)
*   [MCP Server](https://developers.openai.com/apps-sdk/concepts/mcp-server)
*   [UX principles](https://developers.openai.com/apps-sdk/concepts/ux-principles)
*   [UI guidelines](https://developers.openai.com/apps-sdk/concepts/ui-guidelines)

### Plan

*   [Research use cases](https://developers.openai.com/apps-sdk/plan/use-case)
*   [Define tools](https://developers.openai.com/apps-sdk/plan/tools)
*   [Design components](https://developers.openai.com/apps-sdk/plan/components)

### Build

*   [Set up your server](https://developers.openai.com/apps-sdk/build/mcp-server)
*   [Build your ChatGPT UI](https://developers.openai.com/apps-sdk/build/chatgpt-ui)
*   [Authenticate users](https://developers.openai.com/apps-sdk/build/auth)
*   [Manage state](https://developers.openai.com/apps-sdk/build/state-management)
*   [Monetize your app](https://developers.openai.com/apps-sdk/build/monetization)
*   [Examples](https://developers.openai.com/apps-sdk/build/examples)

### Deploy

*   [Deploy your app](https://developers.openai.com/apps-sdk/deploy)
*   [Connect from ChatGPT](https://developers.openai.com/apps-sdk/deploy/connect-chatgpt)
*   [Test your integration](https://developers.openai.com/apps-sdk/deploy/testing)
*   [Submit your app](https://developers.openai.com/apps-sdk/deploy/submission)

### Conversion apps

*   [Restaurant reservation spec](https://developers.openai.com/apps-sdk/guides/restaurant-reservation-conversion-spec)
*   [Product checkout spec](https://developers.openai.com/apps-sdk/guides/product-checkout-conversion-spec)

### Guides

*   [Optimize Metadata](https://developers.openai.com/apps-sdk/guides/optimize-metadata)
*   [Security & Privacy](https://developers.openai.com/apps-sdk/guides/security-privacy)
*   [Troubleshooting](https://developers.openai.com/apps-sdk/deploy/troubleshooting)

### Resources

*   [Changelog](https://developers.openai.com/apps-sdk/changelog)
*   [App submission guidelines](https://developers.openai.com/apps-sdk/app-submission-guidelines)
*   [Reference](https://developers.openai.com/apps-sdk/reference)

*   [Home](https://developers.openai.com/workspace-agents)

### Get started

*   [Trigger workspace agent runs](https://developers.openai.com/workspace-agents/trigger-runs)
*   [Authenticate with Workspace Agent access tokens](https://developers.openai.com/workspace-agents/authentication)

*   [Home](https://developers.openai.com/commerce)

### Guides

*   [Get started](https://developers.openai.com/commerce/guides/get-started)
*   [Best practices](https://developers.openai.com/commerce/guides/best-practices)

### File Upload

*   [Overview](https://developers.openai.com/commerce/specs/file-upload/overview)
*   [Products](https://developers.openai.com/commerce/specs/file-upload/products)

### API

*   [Overview](https://developers.openai.com/commerce/specs/api/overview)
*   [Feeds](https://developers.openai.com/commerce/specs/api/feeds)
*   [Products](https://developers.openai.com/commerce/specs/api/products)
*   [Promotions](https://developers.openai.com/commerce/specs/api/promotions)

*   [Ads Overview](https://developers.openai.com/ads)

### Measurement

*   [JavaScript Pixel](https://developers.openai.com/ads/measurement-pixel)
*   [Conversions API](https://developers.openai.com/ads/conversions-api)
*   [Supported events](https://developers.openai.com/ads/supported-events)

### Advertiser API

*   [Overview](https://developers.openai.com/ads/api-overview)
*   [Quickstart](https://developers.openai.com/ads/api-quickstart)
*   [Product feeds](https://developers.openai.com/ads/product-feeds)
*   [Campaign Targeting](https://developers.openai.com/ads/campaign-targeting)

### API Reference

*   [Authentication](https://developers.openai.com/ads/api-reference/authentication)
*   [Campaigns](https://developers.openai.com/ads/api-reference/campaigns)
*   [Ad Groups](https://developers.openai.com/ads/api-reference/ad-groups)
*   [Ads](https://developers.openai.com/ads/api-reference/ads)
*   [Ad Account](https://developers.openai.com/ads/api-reference/ad-account)
*   [Insights](https://developers.openai.com/ads/api-reference/insights)
*   [Files](https://developers.openai.com/ads/api-reference/files)

 Showcase  Blog  Cookbook  Learn  Community 

*   [Home](https://developers.openai.com/showcase)
*   [API examples](https://developers.openai.com/showcase/api-examples)
*   [Sites](https://developers.openai.com/showcase/sites)

*   [All posts](https://developers.openai.com/blog)

### Recent

*   [How Perplexity Brought Voice Search to Millions Using the Realtime API](https://developers.openai.com/blog/realtime-perplexity-computer)
*   [Designing delightful frontends with GPT-5.4](https://developers.openai.com/blog/designing-delightful-frontends-with-gpt-5-4)
*   [From prompts to products: One year of Responses](https://developers.openai.com/blog/one-year-of-responses)
*   [Using skills to accelerate OSS maintenance](https://developers.openai.com/blog/skills-agents-sdk)
*   [Building frontend UIs with Codex and Figma](https://developers.openai.com/blog/building-frontend-uis-with-codex-and-figma)

### Topics

*   [General](https://developers.openai.com/blog/topic/general)
*   [API](https://developers.openai.com/blog/topic/api)
*   [Apps SDK](https://developers.openai.com/blog/topic/apps-sdk)
*   [Audio](https://developers.openai.com/blog/topic/audio)
*   [Codex](https://developers.openai.com/blog/topic/codex)

*   [Home](https://developers.openai.com/cookbook)

### Topics

*   [Agents](https://developers.openai.com/cookbook/topic/agents)
*   [Evals](https://developers.openai.com/cookbook/topic/evals)
*   [Multimodal](https://developers.openai.com/cookbook/topic/multimodal)
*   [Text](https://developers.openai.com/cookbook/topic/text)
*   [Guardrails](https://developers.openai.com/cookbook/topic/guardrails)
*   [Optimization](https://developers.openai.com/cookbook/topic/optimization)
*   [ChatGPT](https://developers.openai.com/cookbook/topic/chatgpt)
*   [Codex](https://developers.openai.com/cookbook/topic/codex)
*   [gpt-oss](https://developers.openai.com/cookbook/topic/gpt-oss)

### Contribute

*   [Cookbook on GitHub](https://github.com/openai/openai-cookbook)

*   [Home](https://developers.openai.com/learn)
*   [OpenAI Developers plugin](https://developers.openai.com/learn/developers-codex-plugin)
*   [Docs MCP](https://developers.openai.com/learn/docs-mcp)

### Categories

*   [Demo apps](https://developers.openai.com/learn/code)
*   [Videos](https://developers.openai.com/learn/videos)

### Topics

*   [Agents](https://developers.openai.com/learn/agents)
*   [Audio & Voice](https://developers.openai.com/learn/audio)
*   [Computer Use](https://developers.openai.com/learn/cua)
*   [Codex](https://developers.openai.com/learn/codex)
*   [Evals](https://developers.openai.com/learn/evals)
*   [gpt-oss](https://developers.openai.com/learn/gpt-oss)
*   [Fine-tuning](https://developers.openai.com/learn/fine-tuning)
*   [Image generation](https://developers.openai.com/learn/imagegen)
*   [Scaling](https://developers.openai.com/learn/scaling)
*   [Tools](https://developers.openai.com/learn/tools)
*   [Video generation](https://developers.openai.com/learn/videogen)

*   [Community](https://developers.openai.com/community)

### Programs

*   [Codex Ambassadors](https://developers.openai.com/community/codex-ambassadors)
*   [Codex for Students](https://developers.openai.com/community/students)
*   [Codex for Open Source](https://developers.openai.com/community/codex-for-oss)

### Events

*   [Meetups](https://developers.openai.com/community/meetups)
*   [Hackathon Support](https://developers.openai.com/community/hackathons)

*   [Forum](https://community.openai.com/)
*   [Discord](https://discord.com/invite/openai)

[API Dashboard](https://platform.openai.com/login)

[API](https://developers.openai.com/api/docs)[API Reference](https://developers.openai.com/api/reference/overview)[Codex](https://developers.openai.com/codex)[ChatGPT](https://developers.openai.com/chatgpt)[Resources](https://developers.openai.com/learn)

*   [API Reference](https://developers.openai.com/api/reference)
*   
API Reference 
    *   [Introduction](https://developers.openai.com/api/reference/overview)
    *   [Authentication](https://developers.openai.com/api/reference/overview#authentication)
    *   [Workload identity tokens](https://developers.openai.com/api/reference/workload-identity-federation)
    *   [Debugging requests](https://developers.openai.com/api/reference/overview#debugging-requests)
    *   [Backwards compatibility](https://developers.openai.com/api/reference/overview#backwards-compatibility)

*   
Responses API 
    *   [Overview](https://developers.openai.com/api/reference/responses/overview)
    *   
Responses
        *   [Create a response](https://developers.openai.com/api/reference/resources/responses/methods/create)
        *   [Retrieve a response](https://developers.openai.com/api/reference/resources/responses/methods/retrieve)
        *   [Delete a response](https://developers.openai.com/api/reference/resources/responses/methods/delete)
        *   [List input items](https://developers.openai.com/api/reference/resources/responses/subresources/input_items/methods/list)
        *   [Count input tokens](https://developers.openai.com/api/reference/resources/responses/subresources/input_tokens/methods/count)
        *   [Cancel a response](https://developers.openai.com/api/reference/resources/responses/methods/cancel)
        *   [Compact a response](https://developers.openai.com/api/reference/resources/responses/methods/compact)

    *   
Conversations
        *   [Create a conversation](https://developers.openai.com/api/reference/resources/conversations/methods/create)
        *   [Retrieve a conversation](https://developers.openai.com/api/reference/resources/conversations/methods/retrieve)
        *   [Update a conversation](https://developers.openai.com/api/reference/resources/conversations/methods/update)
        *   [Delete a conversation](https://developers.openai.com/api/reference/resources/conversations/methods/delete)
        *   
Items
            *   [Create an item](https://developers.openai.com/api/reference/resources/conversations/subresources/items/methods/create)
            *   [Retrieve an item](https://developers.openai.com/api/reference/resources/conversations/subresources/items/methods/retrieve)
            *   [Delete an item](https://developers.openai.com/api/reference/resources/conversations/subresources/items/methods/delete)
            *   [List items](https://developers.openai.com/api/reference/resources/conversations/subresources/items/methods/list)

    *   [Streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events)

*   
Webhooks 
    *   [Events](https://developers.openai.com/api/reference/resources/webhooks)

*   
Platform APIs 
    *   
Audio
        *   [Create a transcription](https://developers.openai.com/api/reference/resources/audio/subresources/transcriptions/methods/create)
        *   [Create a translation](https://developers.openai.com/api/reference/resources/audio/subresources/translations/methods/create)
        *   [Create a speech](https://developers.openai.com/api/reference/resources/audio/subresources/speech/methods/create)
        *   [Create a voice](https://developers.openai.com/api/reference/resources/audio/subresources/voices/methods/create)
        *   
Voice Consents
            *   [Create a voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/create)
            *   [Retrieve a voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/retrieve)
            *   [Update a voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/update)
            *   [Delete a voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/delete)
            *   [List voice consents](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/list)

    *   
Videos
        *   [Create a video](https://developers.openai.com/api/reference/resources/videos/methods/create)
        *   [Create Character](https://developers.openai.com/api/reference/resources/videos/methods/create_character)
        *   [Get Character](https://developers.openai.com/api/reference/resources/videos/methods/get_character)
        *   [Retrieve a video](https://developers.openai.com/api/reference/resources/videos/methods/retrieve)
        *   [Delete a video](https://developers.openai.com/api/reference/resources/videos/methods/delete)
        *   [List videos](https://developers.openai.com/api/reference/resources/videos/methods/list)
        *   [Download Content](https://developers.openai.com/api/reference/resources/videos/methods/download_content)
        *   [Edit](https://developers.openai.com/api/reference/resources/videos/methods/edit)
        *   [Extend](https://developers.openai.com/api/reference/resources/videos/methods/extend)
        *   [Remix](https://developers.openai.com/api/reference/resources/videos/methods/remix)

    *   
Images
        *   [Generate an Image](https://developers.openai.com/api/reference/resources/images/methods/generate)
        *   [Edit an Image](https://developers.openai.com/api/reference/resources/images/methods/edit)
        *   [Create Variation](https://developers.openai.com/api/reference/resources/images/methods/create_variation)
        *   [Image generation streaming events](https://developers.openai.com/api/reference/resources/images/generation-streaming-events)
        *   [Image edit streaming events](https://developers.openai.com/api/reference/resources/images/edit-streaming-events)

    *   
Embeddings
        *   [Create an embedding](https://developers.openai.com/api/reference/resources/embeddings/methods/create)

    *   
Evals
        *   [Create an eval](https://developers.openai.com/api/reference/resources/evals/methods/create)
        *   [Retrieve an eval](https://developers.openai.com/api/reference/resources/evals/methods/retrieve)
        *   [Update an eval](https://developers.openai.com/api/reference/resources/evals/methods/update)
        *   [Delete an eval](https://developers.openai.com/api/reference/resources/evals/methods/delete)
        *   [List evals](https://developers.openai.com/api/reference/resources/evals/methods/list)
        *   
Runs
            *   [Create a run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/create)
            *   [Retrieve a run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/retrieve)
            *   [Delete a run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/delete)
            *   [List runs](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/list)
            *   [Cancel a run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/cancel)
            *   
Output Items
                *   [Retrieve an output item](https://developers.openai.com/api/reference/resources/evals/subresources/runs/subresources/output_items/methods/retrieve)
                *   [List output items](https://developers.openai.com/api/reference/resources/evals/subresources/runs/subresources/output_items/methods/list)

    *   
Fine Tuning
        *   
Jobs
            *   [Create a job](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/create)
            *   [Retrieve a job](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/retrieve)
            *   [List jobs](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/list)
            *   [List Events](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/list_events)
            *   [Cancel a job](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/cancel)
            *   [Pause](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/pause)
            *   [Resume](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/resume)
            *   
Checkpoints
                *   [List checkpoints](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/subresources/checkpoints/methods/list)

        *   
Checkpoints
            *   
Permissions
                *   [Create a permission](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/create)
                *   [Retrieve a permission](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/retrieve)
                *   [Delete a permission](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/delete)
                *   [List permissions](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/retrieve)

        *   
Alpha
            *   
Graders
                *   [Run](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/alpha/subresources/graders/methods/run)
                *   [Validate](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/alpha/subresources/graders/methods/validate)

    *   
Batches
        *   [Create a batch](https://developers.openai.com/api/reference/resources/batches/methods/create)
        *   [Retrieve a batch](https://developers.openai.com/api/reference/resources/batches/methods/retrieve)
        *   [List batches](https://developers.openai.com/api/reference/resources/batches/methods/list)
        *   [Cancel a batch](https://developers.openai.com/api/reference/resources/batches/methods/cancel)

    *   
Files
        *   [List files](https://developers.openai.com/api/reference/resources/files/methods/list)
        *   [Create a file](https://developers.openai.com/api/reference/resources/files/methods/create)
        *   [Retrieve a file](https://developers.openai.com/api/reference/resources/files/methods/retrieve)
        *   [Delete a file](https://developers.openai.com/api/reference/resources/files/methods/delete)
        *   [Retrieve file content](https://developers.openai.com/api/reference/resources/files/methods/content)

    *   
Uploads
        *   [Create an upload](https://developers.openai.com/api/reference/resources/uploads/methods/create)
        *   [Cancel an upload](https://developers.openai.com/api/reference/resources/uploads/methods/cancel)
        *   [Complete](https://developers.openai.com/api/reference/resources/uploads/methods/complete)
        *   
Parts
            *   [Create a part](https://developers.openai.com/api/reference/resources/uploads/subresources/parts/methods/create)

    *   
Models
        *   [Retrieve a model](https://developers.openai.com/api/reference/resources/models/methods/retrieve)
        *   [Delete a model](https://developers.openai.com/api/reference/resources/models/methods/delete)
        *   [List models](https://developers.openai.com/api/reference/resources/models/methods/list)

    *   
Moderations
        *   [Create a moderation](https://developers.openai.com/api/reference/resources/moderations/methods/create)

*   
Vector Stores 
    *   
Vector Stores
        *   [Create a vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/create)
        *   [Retrieve a vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/retrieve)
        *   [Update a vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/update)
        *   [Delete a vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/delete)
        *   [List vector stores](https://developers.openai.com/api/reference/resources/vector_stores/methods/list)
        *   [Search](https://developers.openai.com/api/reference/resources/vector_stores/methods/search)

    *   
Files
        *   [List files](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/list)
        *   [Create a file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/create)
        *   [Retrieve a file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/retrieve)
        *   [Update a file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/update)
        *   [Delete a file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/delete)
        *   [Retrieve file content](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/content)

    *   
File Batches
        *   [Create a file batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/create)
        *   [Retrieve a file batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/retrieve)
        *   [List Files](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/list_files)
        *   [Cancel a file batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/cancel)

*   
ChatKit 
    *   
Sessions
        *   [Create a session](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions/methods/create)
        *   [Cancel a session](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions/methods/cancel)

    *   
Threads
        *   [Retrieve a thread](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/retrieve)
        *   [Delete a thread](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/delete)
        *   [List Items](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/list_items)
        *   [List threads](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/list)

*   
Containers 
    *   
Containers
        *   [Create a container](https://developers.openai.com/api/reference/resources/containers/methods/create)
        *   [Retrieve a container](https://developers.openai.com/api/reference/resources/containers/methods/retrieve)
        *   [Delete a container](https://developers.openai.com/api/reference/resources/containers/methods/delete)
        *   [List containers](https://developers.openai.com/api/reference/resources/containers/methods/list)

    *   
Files
        *   [List files](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/list)
        *   [Create a file](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/create)
        *   [Retrieve a file](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/retrieve)
        *   [Delete a file](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/delete)
        *   
Content
            *   [Retrieve a content](https://developers.openai.com/api/reference/resources/containers/subresources/files/subresources/content/methods/retrieve)

*   
Skills 
    *   
Skills
        *   [Create a skill](https://developers.openai.com/api/reference/resources/skills/methods/create)
        *   [Retrieve a skill](https://developers.openai.com/api/reference/resources/skills/methods/retrieve)
        *   [Retrieve skill content](https://developers.openai.com/api/reference/resources/skills/subresources/content/methods/retrieve)
        *   [Update a skill](https://developers.openai.com/api/reference/resources/skills/methods/update)
        *   [Delete a skill](https://developers.openai.com/api/reference/resources/skills/methods/delete)
        *   [List skills](https://developers.openai.com/api/reference/resources/skills/methods/list)
        *   
Versions
            *   [Create skill version](https://developers.openai.com/api/reference/resources/skills/subresources/versions/methods/create)
            *   [Retrieve skill version](https://developers.openai.com/api/reference/resources/skills/subresources/versions/methods/retrieve)
            *   [Retrieve Skill Version Content](https://developers.openai.com/api/reference/resources/skills/subresources/versions/subresources/content/methods/retrieve)
            *   [Delete skill version](https://developers.openai.com/api/reference/resources/skills/subresources/versions/methods/delete)
            *   [List skill versions](https://developers.openai.com/api/reference/resources/skills/subresources/versions/methods/list)

*   
Realtime 
    *   
Translations
        *   
Client Secrets
            *   [Create a client secret](https://developers.openai.com/api/reference/resources/realtime/subresources/translations/subresources/client_secrets/methods/create)

        *   [Translation client events](https://developers.openai.com/api/reference/resources/realtime/translation-client-events)
        *   [Translation server events](https://developers.openai.com/api/reference/resources/realtime/translation-server-events)

    *   
Client Secrets
        *   [Create a client secret](https://developers.openai.com/api/reference/resources/realtime/subresources/client_secrets/methods/create)

    *   
Calls
        *   [Accept](https://developers.openai.com/api/reference/resources/realtime/subresources/calls/methods/accept)
        *   [Hangup](https://developers.openai.com/api/reference/resources/realtime/subresources/calls/methods/hangup)
        *   [Refer](https://developers.openai.com/api/reference/resources/realtime/subresources/calls/methods/refer)
        *   [Reject](https://developers.openai.com/api/reference/resources/realtime/subresources/calls/methods/reject)

    *   [Client events](https://developers.openai.com/api/reference/resources/realtime/client-events)
    *   [Server events](https://developers.openai.com/api/reference/resources/realtime/server-events)

*   
Administration 
    *   [Overview](https://developers.openai.com/api/reference/administration/overview)
    *   
Audit Logs
        *   [List audit logs](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/audit_logs/methods/list)

    *   
Admin API Keys
        *   [Create an admin API key](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/admin_api_keys/methods/create)
        *   [Retrieve an admin API key](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/admin_api_keys/methods/retrieve)
        *   [Delete an admin API key](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/admin_api_keys/methods/delete)
        *   [List admin API keys](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/admin_api_keys/methods/list)

    *   
Usage
        *   [Audio Speeches](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/audio_speeches)
        *   [Audio Transcriptions](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/audio_transcriptions)
        *   [Code Interpreter Sessions](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/code_interpreter_sessions)
        *   [Completions](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/completions)
        *   [Embeddings](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/embeddings)
        *   [Images](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/images)
        *   [Moderations](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/moderations)
        *   [Vector Stores](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/vector_stores)
        *   [File Search Calls](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/file_search_calls)
        *   [Web Search Calls](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/web_search_calls)
        *   [Costs](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/usage/methods/costs)

    *   
Invites
        *   [Create an invite](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/invites/methods/create)
        *   [Retrieve an invite](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/invites/methods/retrieve)
        *   [Delete an invite](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/invites/methods/delete)
        *   [List invites](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/invites/methods/list)

    *   
Users
        *   [Retrieve an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/methods/retrieve)
        *   [Update an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/methods/update)
        *   [Delete an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/methods/delete)
        *   [List users](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/methods/list)
        *   
Roles
            *   [Create a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/subresources/roles/methods/create)
            *   [Retrieve a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/subresources/roles/methods/retrieve)
            *   [Delete a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/subresources/roles/methods/delete)
            *   [List roles](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/users/subresources/roles/methods/list)

    *   
Groups
        *   [Create a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/methods/create)
        *   [Retrieve a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/methods/retrieve)
        *   [Update a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/methods/update)
        *   [Delete a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/methods/delete)
        *   [List groups](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/methods/list)
        *   
Users
            *   [Create an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/users/methods/create)
            *   [Retrieve an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/users/methods/retrieve)
            *   [Delete an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/users/methods/delete)
            *   [List users](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/users/methods/list)

        *   
Roles
            *   [Create a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/roles/methods/create)
            *   [Retrieve a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/roles/methods/retrieve)
            *   [Delete a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/roles/methods/delete)
            *   [List roles](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/groups/subresources/roles/methods/list)

    *   
Roles
        *   [Create a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/roles/methods/create)
        *   [Retrieve a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/roles/methods/retrieve)
        *   [Update a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/roles/methods/update)
        *   [Delete a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/roles/methods/delete)
        *   [List roles](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/roles/methods/list)

    *   
Data Retention
        *   [Retrieve a data retention](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/data_retention/methods/retrieve)
        *   [Update a data retention](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/data_retention/methods/update)

    *   
Spend Alerts
        *   [Create a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/spend_alerts/methods/create)
        *   [Retrieve a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/spend_alerts/methods/retrieve)
        *   [Update a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/spend_alerts/methods/update)
        *   [Delete a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/spend_alerts/methods/delete)
        *   [List spend alerts](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/spend_alerts/methods/list)

    *   
Certificates
        *   [Create a certificate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/create)
        *   [Retrieve a certificate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/retrieve)
        *   [Update a certificate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/update)
        *   [Delete a certificate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/delete)
        *   [List certificates](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/list)
        *   [Activate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/activate)
        *   [Deactivate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/certificates/methods/deactivate)

    *   
Projects
        *   [Create a project](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/methods/create)
        *   [Retrieve a project](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/methods/retrieve)
        *   [Update a project](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/methods/update)
        *   [List projects](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/methods/list)
        *   [Archive](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/methods/archive)
        *   
Users
            *   [Create an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/methods/create)
            *   [Retrieve an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/methods/retrieve)
            *   [Update an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/methods/update)
            *   [Delete an user](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/methods/delete)
            *   [List users](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/methods/list)
            *   
Roles
                *   [Create a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/subresources/roles/methods/create)
                *   [Retrieve a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/subresources/roles/methods/retrieve)
                *   [Delete a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/subresources/roles/methods/delete)
                *   [List roles](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/users/subresources/roles/methods/list)

        *   
Service Accounts
            *   [Create a service account](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/service_accounts/methods/create)
            *   [Retrieve a service account](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/service_accounts/methods/retrieve)
            *   [Update a service account](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/service_accounts/methods/update)
            *   [Delete a service account](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/service_accounts/methods/delete)
            *   [List service accounts](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/service_accounts/methods/list)

        *   
API Keys
            *   [Retrieve an API key](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/api_keys/methods/retrieve)
            *   [Delete an API key](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/api_keys/methods/delete)
            *   [List API keys](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/api_keys/methods/list)

        *   
Rate Limits
            *   [Update Rate Limit](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/rate_limits/methods/update_rate_limit)
            *   [List Rate Limits](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/rate_limits/methods/list_rate_limits)

        *   
Model Permissions
            *   [Retrieve a model permission](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/model_permissions/methods/retrieve)
            *   [Update a model permission](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/model_permissions/methods/update)
            *   [Delete a model permission](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/model_permissions/methods/delete)

        *   
Hosted Tool Permissions
            *   [Retrieve a hosted tool permission](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/hosted_tool_permissions/methods/retrieve)
            *   [Update a hosted tool permission](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/hosted_tool_permissions/methods/update)

        *   
Groups
            *   [Create a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/methods/create)
            *   [Retrieve a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/methods/retrieve)
            *   [Delete a group](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/methods/delete)
            *   [List groups](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/methods/list)
            *   
Roles
                *   [Create a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/subresources/roles/methods/create)
                *   [Retrieve a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/subresources/roles/methods/retrieve)
                *   [Delete a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/subresources/roles/methods/delete)
                *   [List roles](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/groups/subresources/roles/methods/list)

        *   
Roles
            *   [Create a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/roles/methods/create)
            *   [Retrieve a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/roles/methods/retrieve)
            *   [Update a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/roles/methods/update)
            *   [Delete a role](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/roles/methods/delete)
            *   [List roles](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/roles/methods/list)

        *   
Data Retention
            *   [Retrieve a data retention](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/data_retention/methods/retrieve)
            *   [Update a data retention](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/data_retention/methods/update)

        *   
Spend Alerts
            *   [Create a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/spend_alerts/methods/create)
            *   [Retrieve a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/spend_alerts/methods/retrieve)
            *   [Update a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/spend_alerts/methods/update)
            *   [Delete a spend alert](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/spend_alerts/methods/delete)
            *   [List spend alerts](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/spend_alerts/methods/list)

        *   
Certificates
            *   [List certificates](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/certificates/methods/list)
            *   [Activate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/certificates/methods/activate)
            *   [Deactivate](https://developers.openai.com/api/reference/resources/admin/subresources/organization/subresources/projects/subresources/certificates/methods/deactivate)

*   
Chat Completions 
    *   
Chat Completions
        *   [Overview](https://developers.openai.com/api/reference/chat-completions/overview)
        *   [Create a chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
        *   [Retrieve a chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/retrieve)
        *   [Update a chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/update)
        *   [Delete a chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/delete)
        *   [List chat completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/list)
        *   [List chat completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/subresources/messages/methods/list)
        *   [Streaming events](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events)

*   
Legacy 
    *   
Realtime Beta
        *   [Overview](https://developers.openai.com/api/reference/realtime-beta/overview)
        *   
Sessions
            *   [Create a session](https://developers.openai.com/api/reference/resources/realtime/subresources/sessions/methods/create)

        *   
Transcription Sessions
            *   [Create a transcription session](https://developers.openai.com/api/reference/resources/realtime/subresources/transcription_sessions/methods/create)

    *   
Assistants
        *   [Create an assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/create)
        *   [Retrieve an assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/retrieve)
        *   [Update an assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/update)
        *   [Delete an assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/delete)
        *   [List assistants](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/list)
        *   
Threads
            *   [Create a thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/create)
            *   [Create And Run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/create_and_run)
            *   [Retrieve a thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/retrieve)
            *   [Update a thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/update)
            *   [Delete a thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/delete)
            *   
Runs
                *   [Create a run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/create)
                *   [Retrieve a run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/retrieve)
                *   [Update a run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/update)
                *   [List runs](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/list)
                *   [Cancel a run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/cancel)
                *   [Submit Tool Outputs](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/submit_tool_outputs)
                *   
Steps
                    *   [Retrieve a step](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/retrieve)
                    *   [List steps](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/list)

            *   
Messages
                *   [Create a message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/create)
                *   [Retrieve a message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/retrieve)
                *   [Update a message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/update)
                *   [Delete a message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/delete)
                *   [List messages](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/list)

        *   [Assistants streaming events](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/streaming-events)

    *   
Completions
        *   [Create a completion](https://developers.openai.com/api/reference/resources/completions/methods/create)

[API Reference](https://developers.openai.com/api/reference)

[Chat](https://developers.openai.com/api/reference/resources/chat)

[Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions)

Copy Markdown

Open in **Claude**

Open in **ChatGPT**

Open in **Cursor**

* * *

**Copy Markdown**

**View as Markdown**

# Create chat completion

POST/chat/completions

**Starting a new project?** We recommend trying [Responses](https://developers.openai.com/docs/api-reference/responses) to take advantage of the latest OpenAI platform features. Compare [Chat Completions with Responses](https://developers.openai.com/docs/guides/responses-vs-chat-completions?api-mode=responses).

* * *

Creates a model response for the given chat conversation. Learn more in the [text generation](https://developers.openai.com/docs/guides/text-generation), [vision](https://developers.openai.com/docs/guides/vision), and [audio](https://developers.openai.com/docs/guides/audio) guides.

Parameter support can differ depending on the model used to generate the response, particularly for newer reasoning models. Parameters that are only supported for reasoning models are noted below. For the current state of unsupported parameters in reasoning models, [refer to the reasoning guide](https://developers.openai.com/docs/guides/reasoning).

Returns a chat completion object, or a streamed sequence of chat completion chunk objects if the request is streamed.

##### Body Parameters JSON Expand Collapse

messages: array of [ChatCompletionMessageParam](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_param%20%3E%20(schema))

A list of messages comprising the conversation so far. Depending on the [model](https://developers.openai.com/docs/models) you use, different message types (modalities) are supported, like [text](https://developers.openai.com/docs/guides/text-generation), [images](https://developers.openai.com/docs/guides/vision), and [audio](https://developers.openai.com/docs/guides/audio).

One of the following:

ChatCompletionDeveloperMessageParam object {content, role, name} 

Developer-provided instructions that the model should follow, regardless of messages sent by the user. With o1 models and newer, `developer` messages replace the previous `system` messages.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

The contents of the developer message.

One of the following:

TextContent = string

The contents of the developer message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%200)

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

An array of content parts with a defined type. For developer messages, only type `text` is supported.

text: string

The text content.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20text)

type: "text"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema)%20%3E%20(property)%20content)

role: "developer"

The role of the messages author, in this case `developer`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema)%20%3E%20(property)%20role)

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema)%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema))

ChatCompletionSystemMessageParam object {content, role, name} 

Developer-provided instructions that the model should follow, regardless of messages sent by the user. With o1 models and newer, use `developer` messages for this purpose instead.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

The contents of the system message.

One of the following:

TextContent = string

The contents of the system message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%200)

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

An array of content parts with a defined type. For system messages, only type `text` is supported.

text: string

The text content.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20text)

type: "text"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema)%20%3E%20(property)%20content)

role: "system"

The role of the messages author, in this case `system`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema)%20%3E%20(property)%20role)

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema)%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema))

ChatCompletionUserMessageParam object {content, role, name} 

Messages sent by an end user, containing prompts or additional context information.

content: string or array of [ChatCompletionContentPart](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema))

The contents of the user message.

One of the following:

TextContent = string

The text contents of the message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%200)

ArrayOfContentParts = array of [ChatCompletionContentPart](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema))

An array of content parts with a defined type. Supported options differ based on the [model](https://developers.openai.com/docs/models) being used to generate the response. Can contain text, image, or audio inputs.

One of the following:

ChatCompletionContentPartText object {text, type} 

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20text)

type: "text"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema))

ChatCompletionContentPartImage object {image_url, type} 

Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

image_url: object {url, detail} 

url: string

Either a URL of the image or the base64 encoded image data.

format uri

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20image_url%20%3E%20(property)%20url)

detail: optional "auto"or"low"or"high"

Specifies the detail level of the image. Learn more in the [Vision guide](https://developers.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).

One of the following:

"auto"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20image_url%20%3E%20(property)%20detail%20%3E%20(member)%200)

"low"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20image_url%20%3E%20(property)%20detail%20%3E%20(member)%201)

"high"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20image_url%20%3E%20(property)%20detail%20%3E%20(member)%202)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20image_url%20%3E%20(property)%20detail)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20image_url)

type: "image_url"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema))

ChatCompletionContentPartInputAudio object {input_audio, type} 

Learn about [audio inputs](https://developers.openai.com/docs/guides/audio).

input_audio: object {data, format} 

data: string

Base64 encoded audio data.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)%20%3E%20(property)%20input_audio%20%3E%20(property)%20data)

format: "wav"or"mp3"

The format of the encoded audio data. Currently supports “wav” and “mp3”.

One of the following:

"wav"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)%20%3E%20(property)%20input_audio%20%3E%20(property)%20format%20%3E%20(member)%200)

"mp3"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)%20%3E%20(property)%20input_audio%20%3E%20(property)%20format%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)%20%3E%20(property)%20input_audio%20%3E%20(property)%20format)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)%20%3E%20(property)%20input_audio)

type: "input_audio"

The type of the content part. Always `input_audio`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema))

FileContentPart object {file, type} 

Learn about [file inputs](https://developers.openai.com/docs/guides/text) for text generation.

file: object {file_data, file_id, filename} 

file_data: optional string

The base64 encoded file data, used when passing the file to the model as a string.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema)%20%3E%20(variant)%203%20%3E%20(property)%20file%20%3E%20(property)%20file_data)

file_id: optional string

The ID of an uploaded file to use as input.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema)%20%3E%20(variant)%203%20%3E%20(property)%20file%20%3E%20(property)%20file_id)

filename: optional string

The name of the file, used when passing the file to the model as a string.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema)%20%3E%20(variant)%203%20%3E%20(property)%20file%20%3E%20(property)%20filename)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema)%20%3E%20(variant)%203%20%3E%20(property)%20file)

type: "file"

The type of the content part. Always `file`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema)%20%3E%20(variant)%203%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema)%20%3E%20(variant)%203)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema)%20%3E%20(property)%20content)

role: "user"

The role of the messages author, in this case `user`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema)%20%3E%20(property)%20role)

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema)%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema))

ChatCompletionAssistantMessageParam object {role, audio, content, 4 more} 

Messages sent by the model in response to user messages.

role: "assistant"

The role of the messages author, in this case `assistant`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20role)

audio: optional object {id} 

Data about a previous audio response from the model. [Learn more](https://developers.openai.com/docs/guides/audio).

id: string

Unique identifier for a previous audio response from the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20audio%20%3E%20(property)%20id)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20audio)

content: optional string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } or[ChatCompletionContentPartRefusal](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)) { refusal, type } 

The contents of the assistant message. Required unless `tool_calls` or `function_call` is specified.

One of the following:

TextContent = string

The contents of the assistant message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%200)

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } or[ChatCompletionContentPartRefusal](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)) { refusal, type } 

An array of content parts with a defined type. Can be one or more of type `text`, or exactly one of type `refusal`.

One of the following:

ChatCompletionContentPartText object {text, type} 

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20text)

type: "text"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema))

ChatCompletionContentPartRefusal object {refusal, type} 

refusal: string

The refusal message generated by the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)%20%3E%20(property)%20refusal)

type: "refusal"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20content)

Deprecated function_call: optional object {arguments, name} 

Deprecated and replaced by `tool_calls`. The name and arguments of a function that should be called, as generated by the model.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20function_call%20%3E%20(property)%20arguments)

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20function_call%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20function_call)

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20name)

refusal: optional string

The refusal message by the assistant.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20refusal)

tool_calls: optional array of [ChatCompletionMessageToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_tool_call%20%3E%20(schema))

The tool calls generated by the model, such as function calls.

One of the following:

ChatCompletionMessageFunctionToolCall object {id, function, type} 

A call to a function tool created by the model.

id: string

The ID of the tool call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20id)

function: object {arguments, name} 

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20function%20%3E%20(property)%20arguments)

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20function%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20function)

type: "function"

The type of the tool. Currently, only `function` is supported.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema))

ChatCompletionMessageCustomToolCall object {id, custom, type} 

A call to a custom tool created by the model.

id: string

The ID of the tool call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20id)

custom: object {input, name} 

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20input)

name: string

The name of the custom tool to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20custom)

type: "custom"

The type of the tool. Always `custom`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema)%20%3E%20(property)%20tool_calls)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_assistant_message_param%20%3E%20(schema))

ChatCompletionToolMessageParam object {content, role, tool_call_id} 

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

The contents of the tool message.

One of the following:

TextContent = string

The contents of the tool message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%200)

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

An array of content parts with a defined type. For tool messages, only type `text` is supported.

text: string

The text content.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20text)

type: "text"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_message_param%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_message_param%20%3E%20(schema)%20%3E%20(property)%20content)

role: "tool"

The role of the messages author, in this case `tool`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_message_param%20%3E%20(schema)%20%3E%20(property)%20role)

tool_call_id: string

Tool call that this message is responding to.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_message_param%20%3E%20(schema)%20%3E%20(property)%20tool_call_id)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_message_param%20%3E%20(schema))

ChatCompletionFunctionMessageParam object {content, name, role} 

content: string

The contents of the function message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_message_param%20%3E%20(schema)%20%3E%20(property)%20content)

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_message_param%20%3E%20(schema)%20%3E%20(property)%20name)

role: "function"

The role of the messages author, in this case `function`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_message_param%20%3E%20(schema)%20%3E%20(property)%20role)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_message_param%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20messages%20%3E%20(schema))

model: string or"gpt-5.4"or"gpt-5.4-mini"or"gpt-5.4-nano"or 75 more

Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, and price points. Refer to the [model guide](https://developers.openai.com/docs/models) to browse and compare available models.

One of the following:

string

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%200)

"gpt-5.4"or"gpt-5.4-mini"or"gpt-5.4-nano"or 75 more

Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, and price points. Refer to the [model guide](https://developers.openai.com/docs/models) to browse and compare available models.

One of the following:

"gpt-5.4"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%200)

"gpt-5.4-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%201)

"gpt-5.4-nano"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%202)

"gpt-5.4-mini-2026-03-17"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%203)

"gpt-5.4-nano-2026-03-17"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%204)

"gpt-5.3-chat-latest"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%205)

"gpt-5.2"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%206)

"gpt-5.2-2025-12-11"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%207)

"gpt-5.2-chat-latest"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%208)

"gpt-5.2-pro"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%209)

"gpt-5.2-pro-2025-12-11"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2010)

"gpt-5.1"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2011)

"gpt-5.1-2025-11-13"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2012)

"gpt-5.1-codex"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2013)

"gpt-5.1-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2014)

"gpt-5.1-chat-latest"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2015)

"gpt-5"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2016)

"gpt-5-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2017)

"gpt-5-nano"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2018)

"gpt-5-2025-08-07"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2019)

"gpt-5-mini-2025-08-07"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2020)

"gpt-5-nano-2025-08-07"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2021)

"gpt-5-chat-latest"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2022)

"gpt-4.1"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2023)

"gpt-4.1-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2024)

"gpt-4.1-nano"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2025)

"gpt-4.1-2025-04-14"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2026)

"gpt-4.1-mini-2025-04-14"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2027)

"gpt-4.1-nano-2025-04-14"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2028)

"o4-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2029)

"o4-mini-2025-04-16"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2030)

"o3"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2031)

"o3-2025-04-16"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2032)

"o3-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2033)

"o3-mini-2025-01-31"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2034)

"o1"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2035)

"o1-2024-12-17"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2036)

"o1-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2037)

"o1-preview-2024-09-12"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2038)

"o1-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2039)

"o1-mini-2024-09-12"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2040)

"gpt-4o"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2041)

"gpt-4o-2024-11-20"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2042)

"gpt-4o-2024-08-06"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2043)

"gpt-4o-2024-05-13"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2044)

"gpt-4o-audio-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2045)

"gpt-4o-audio-preview-2024-10-01"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2046)

"gpt-4o-audio-preview-2024-12-17"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2047)

"gpt-4o-audio-preview-2025-06-03"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2048)

"gpt-4o-mini-audio-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2049)

"gpt-4o-mini-audio-preview-2024-12-17"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2050)

"gpt-4o-search-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2051)

"gpt-4o-mini-search-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2052)

"gpt-4o-search-preview-2025-03-11"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2053)

"gpt-4o-mini-search-preview-2025-03-11"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2054)

"chatgpt-4o-latest"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2055)

"codex-mini-latest"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2056)

"gpt-4o-mini"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2057)

"gpt-4o-mini-2024-07-18"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2058)

"gpt-4-turbo"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2059)

"gpt-4-turbo-2024-04-09"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2060)

"gpt-4-0125-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2061)

"gpt-4-turbo-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2062)

"gpt-4-1106-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2063)

"gpt-4-vision-preview"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2064)

"gpt-4"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2065)

"gpt-4-0314"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2066)

"gpt-4-0613"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2067)

"gpt-4-32k"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2068)

"gpt-4-32k-0314"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2069)

"gpt-4-32k-0613"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2070)

"gpt-3.5-turbo"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2071)

"gpt-3.5-turbo-16k"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2072)

"gpt-3.5-turbo-0301"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2073)

"gpt-3.5-turbo-0613"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2074)

"gpt-3.5-turbo-1106"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2075)

"gpt-3.5-turbo-0125"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2076)

"gpt-3.5-turbo-16k-0613"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201%20%3E%20(member)%2077)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema)%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20model%20%3E%20(schema))

audio: optional [ChatCompletionAudioParam](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)) { format, voice } 

Parameters for audio output. Required when audio output is requested with `modalities: ["audio"]`. [Learn more](https://developers.openai.com/docs/guides/audio).

format: "wav"or"aac"or"mp3"or 3 more

Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`, `opus`, or `pcm16`.

One of the following:

"wav"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format%20%3E%20(member)%200)

"aac"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format%20%3E%20(member)%201)

"mp3"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format%20%3E%20(member)%202)

"flac"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format%20%3E%20(member)%203)

"opus"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format%20%3E%20(member)%204)

"pcm16"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format%20%3E%20(member)%205)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20format)

voice: string or"alloy"or"ash"or"ballad"or 7 more or object {id} 

The voice the model uses to respond. Supported built-in voices are `alloy`, `ash`, `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`, `shimmer`, `marin`, and `cedar`. You may also provide a custom voice object with an `id`, for example `{ "id": "voice_1234" }`.

One of the following:

string

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%200)

"alloy"or"ash"or"ballad"or 7 more

One of the following:

"alloy"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%200)

"ash"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%201)

"ballad"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%202)

"coral"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%203)

"echo"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%204)

"sage"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%205)

"shimmer"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%206)

"verse"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%207)

"marin"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%208)

"cedar"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201%20%3E%20(member)%209)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%201)

ID object {id} 

Custom voice reference.

id: string

The custom voice ID, e.g. `voice_1234`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%202%20%3E%20(property)%20id)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice%20%3E%20(variant)%202)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio_param%20%3E%20(schema)%20%3E%20(property)%20voice)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20audio%20%3E%20(schema))

frequency_penalty: optional number

Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model’s likelihood to repeat the same line verbatim.

minimum-2

maximum 2

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20frequency_penalty%20%3E%20(schema))

Deprecated function_call: optional "none"or"auto"or[ChatCompletionFunctionCallOption](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_call_option%20%3E%20(schema)) { name } 

Deprecated in favor of `tool_choice`.

Controls which (if any) function is called by the model.

`none` means the model will not call a function and instead generates a message.

`auto` means the model can pick between generating a message or calling a function.

Specifying a particular function via `{"name": "my_function"}` forces the model to call that function.

`none` is the default when no functions are present. `auto` is the default if functions are present.

One of the following:

"none"or"auto"

`none` means the model will not call a function and instead generates a message. `auto` means the model can pick between generating a message or calling a function.

One of the following:

"none"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20function_call%20%3E%20(schema)%20%3E%20(variant)%200%20%3E%20(member)%200)

"auto"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20function_call%20%3E%20(schema)%20%3E%20(variant)%200%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20function_call%20%3E%20(schema)%20%3E%20(variant)%200)

ChatCompletionFunctionCallOption object {name} 

Specifying a particular function via `{"name": "my_function"}` forces the model to call that function.

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_call_option%20%3E%20(schema)%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_call_option%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20function_call%20%3E%20(schema))

Deprecated functions: optional array of object {name, description, parameters} 

Deprecated in favor of `tools`.

A list of functions the model may generate JSON inputs for.

name: string

The name of the function to be called. Must be a-z, A-Z, 0-9, or contain underscores and dashes, with a maximum length of 64.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20functions%20%3E%20(schema)%20%3E%20(items)%20%3E%20(property)%20name)

description: optional string

A description of what the function does, used by the model to choose when and how to call the function.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20functions%20%3E%20(schema)%20%3E%20(items)%20%3E%20(property)%20description)

parameters: optional [FunctionParameters](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_parameters%20%3E%20(schema))

The parameters the functions accepts, described as a JSON Schema object. See the [guide](https://developers.openai.com/docs/guides/function-calling) for examples, and the [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for documentation about the format.

Omitting `parameters` defines a function with an empty parameter list.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20functions%20%3E%20(schema)%20%3E%20(items)%20%3E%20(property)%20parameters)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20functions%20%3E%20(schema))

logit_bias: optional map[number]

Modify the likelihood of specified tokens appearing in the completion.

Accepts a JSON object that maps tokens (specified by their token ID in the tokenizer) to an associated bias value from -100 to 100. Mathematically, the bias is added to the logits generated by the model prior to sampling. The exact effect will vary per model, but values between -1 and 1 should decrease or increase likelihood of selection; values like -100 or 100 should result in a ban or exclusive selection of the relevant token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20logit_bias%20%3E%20(schema))

logprobs: optional boolean

Whether to return log probabilities of the output tokens or not. If true, returns the log probabilities of each output token returned in the `content` of `message`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20logprobs%20%3E%20(schema))

max_completion_tokens: optional number

An upper bound for the number of tokens that can be generated for a completion, including visible output tokens and [reasoning tokens](https://developers.openai.com/docs/guides/reasoning).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20max_completion_tokens%20%3E%20(schema))

Deprecated max_tokens: optional number

The maximum number of [tokens](https://developers.openai.com/tokenizer) that can be generated in the chat completion. This value can be used to control [costs](https://openai.com/api/pricing/) for text generated via API.

This value is now deprecated in favor of `max_completion_tokens`, and is not compatible with [o-series models](https://developers.openai.com/docs/guides/reasoning).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20max_tokens%20%3E%20(schema))

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be useful for storing additional information about the object in a structured format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings with a maximum length of 512 characters.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20metadata%20%3E%20(schema))

modalities: optional array of "text"or"audio"

Output types that you would like the model to generate. Most models are capable of generating text, which is the default:

`["text"]`

The `gpt-4o-audio-preview` model can also be used to [generate audio](https://developers.openai.com/docs/guides/audio). To request that this model generate both text and audio responses, you can use:

`["text", "audio"]`

One of the following:

"text"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20modalities%20%3E%20(schema)%20%3E%20(items)%20%3E%20(member)%200)

"audio"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20modalities%20%3E%20(schema)%20%3E%20(items)%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20modalities%20%3E%20(schema))

moderation: optional object {model} 

Configuration for running moderation on the request input and generated output.

model: string

The moderation model to use for moderated completions, e.g. ‘omni-moderation-latest’.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20moderation%20%3E%20(schema)%20%3E%20(property)%20model)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20moderation%20%3E%20(schema))

n: optional number

How many chat completion choices to generate for each input message. Note that you will be charged based on the number of generated tokens across all of the choices. Keep `n` as `1` to minimize costs.

minimum 1

maximum 128

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20n%20%3E%20(schema))

parallel_tool_calls: optional boolean

Whether to enable [parallel function calling](https://developers.openai.com/docs/guides/function-calling#configuring-parallel-function-calling) during tool use.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20parallel_tool_calls%20%3E%20(schema))

prediction: optional [ChatCompletionPredictionContent](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_prediction_content%20%3E%20(schema)) { content, type } 

Static predicted output content, such as the content of a text file that is being regenerated.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

The content that should be matched when generating a model response. If generated tokens would match this content, the entire model response can be returned much more quickly.

One of the following:

TextContent = string

The content used for a Predicted Output. This is often the text of a file you are regenerating with minor changes.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_prediction_content%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%200)

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type } 

An array of content parts with a defined type. Supported options differ based on the [model](https://developers.openai.com/docs/models) being used to generate the response. Can contain text inputs.

text: string

The text content.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20text)

type: "text"

The type of the content part.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_prediction_content%20%3E%20(schema)%20%3E%20(property)%20content%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_prediction_content%20%3E%20(schema)%20%3E%20(property)%20content)

type: "content"

The type of the predicted content you want to provide. This type is currently always `content`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_prediction_content%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prediction%20%3E%20(schema))

presence_penalty: optional number

Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model’s likelihood to talk about new topics.

minimum-2

maximum 2

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20presence_penalty%20%3E%20(schema))

prompt_cache_key: optional string

Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](https://developers.openai.com/docs/guides/prompt-caching).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prompt_cache_key%20%3E%20(schema))

prompt_cache_retention: optional "in_memory"or"24h"

The retention policy for the prompt cache. Set to `24h` to enable extended prompt caching, which keeps cached prefixes active for longer, up to a maximum of 24 hours. [Learn more](https://developers.openai.com/docs/guides/prompt-caching#prompt-cache-retention). For `gpt-5.5`, `gpt-5.5-pro`, and future models, only `24h` is supported.

For older models that support both `in_memory` and `24h`, the default depends on your organization’s data retention policy:

*   Organizations without ZDR enabled default to `24h`.
*   Organizations with ZDR enabled default to `in_memory` when `prompt_cache_retention` is not specified.

One of the following:

"in_memory"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prompt_cache_retention%20%3E%20(schema)%20%3E%20(member)%200)

"24h"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prompt_cache_retention%20%3E%20(schema)%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20prompt_cache_retention%20%3E%20(schema))

reasoning_effort: optional [ReasoningEffort](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema))

Constrains effort on reasoning for [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently supported values are `none`, `minimal`, `low`, `medium`, `high`, and `xhigh`. Reducing reasoning effort can result in faster responses and fewer tokens used on reasoning in a response.

*   `gpt-5.1` defaults to `none`, which does not perform reasoning. The supported reasoning values for `gpt-5.1` are `none`, `low`, `medium`, and `high`. Tool calls are supported for all reasoning values in gpt-5.1.
*   All models before `gpt-5.1` default to `medium` reasoning effort, and do not support `none`.
*   The `gpt-5-pro` model defaults to (and only supports) `high` reasoning effort.
*   `xhigh` is supported for all models after `gpt-5.1-codex-max`.

One of the following:

"none"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema)%20%2B%20(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema)%20%3E%20(member)%200)

"minimal"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema)%20%2B%20(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema)%20%3E%20(member)%201)

"low"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema)%20%2B%20(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema)%20%3E%20(member)%202)

"medium"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema)%20%2B%20(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema)%20%3E%20(member)%203)

"high"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema)%20%2B%20(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema)%20%3E%20(member)%204)

"xhigh"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema)%20%2B%20(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema)%20%3E%20(member)%205)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20reasoning_effort%20%3E%20(schema))

response_format: optional [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type } or[ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json_schema, type } or[ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type } 

An object specifying the format that the model must output.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensures the model will match your supplied JSON schema. Learn more in the [Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables the older JSON mode, which ensures the message the model generates is valid JSON. Using `json_schema` is preferred for models that support it.

One of the following:

ResponseFormatText object {type} 

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema))

ResponseFormatJSONSchema object {json_schema, type} 

JSON Schema response format. Used to generate structured JSON responses. Learn more about [Structured Outputs](https://developers.openai.com/docs/guides/structured-outputs).

json_schema: object {name, description, schema, strict} 

Structured Outputs configuration options, including a JSON Schema.

name: string

The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores and dashes, with a maximum length of 64.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)%20%3E%20(property)%20json_schema%20%3E%20(property)%20name)

description: optional string

A description of what the response format is for, used by the model to determine how to respond in the format.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)%20%3E%20(property)%20json_schema%20%3E%20(property)%20description)

schema: optional map[unknown]

The schema for the response format, described as a JSON Schema object. Learn how to build JSON schemas [here](https://json-schema.org/).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)%20%3E%20(property)%20json_schema%20%3E%20(property)%20schema)

strict: optional boolean

Whether to enable strict schema adherence when generating the output. If set to true, the model will always follow the exact schema defined in the `schema` field. Only a subset of JSON Schema is supported when `strict` is `true`. To learn more, read the [Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)%20%3E%20(property)%20json_schema%20%3E%20(property)%20strict)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)%20%3E%20(property)%20json_schema)

type: "json_schema"

The type of response format being defined. Always `json_schema`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema))

ResponseFormatJSONObject object {type} 

JSON object response format. An older method of generating JSON responses. Using `json_schema` is recommended for models that support it. Note that the model will not generate JSON without a system or user message instructing it to do so.

type: "json_object"

The type of response format being defined. Always `json_object`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20response_format%20%3E%20(schema))

safety_identifier: optional string

A stable identifier used to help detect users of your application that may be violating OpenAI’s usage policies. The IDs should be a string that uniquely identifies each user, with a maximum length of 64 characters. We recommend hashing their username or email address, in order to avoid sending us any identifying information. [Learn more](https://developers.openai.com/docs/guides/safety-best-practices#safety-identifiers).

maxLength 64

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20safety_identifier%20%3E%20(schema))

Deprecated seed: optional number

This feature is in Beta. If specified, our system will make a best effort to sample deterministically, such that repeated requests with the same `seed` and parameters should return the same result. Determinism is not guaranteed, and you should refer to the `system_fingerprint` response parameter to monitor changes in the backend.

minimum-9223372036854776000

maximum 9223372036854776000

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20seed%20%3E%20(schema))

service_tier: optional "auto"or"default"or"flex"or 2 more

Specifies the processing type used for serving the request.

*   If set to ‘auto’, then the request will be processed with the service tier configured in the Project settings. Unless otherwise configured, the Project will use ‘default’.
*   If set to ‘default’, then the request will be processed with the standard pricing and performance for the selected model.
*   If set to ‘[flex](https://developers.openai.com/docs/guides/flex-processing)’ or ‘[priority](https://openai.com/api-priority-processing/)’, then the request will be processed with the corresponding service tier.
*   When not set, the default behavior is ‘auto’.

When the `service_tier` parameter is set, the response body will include the `service_tier` value based on the processing mode actually used to serve the request. This response value may be different from the value set in the parameter.

One of the following:

"auto"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20service_tier%20%3E%20(schema)%20%3E%20(member)%200)

"default"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20service_tier%20%3E%20(schema)%20%3E%20(member)%201)

"flex"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20service_tier%20%3E%20(schema)%20%3E%20(member)%202)

"scale"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20service_tier%20%3E%20(schema)%20%3E%20(member)%203)

"priority"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20service_tier%20%3E%20(schema)%20%3E%20(member)%204)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20service_tier%20%3E%20(schema))

stop: optional string or array of string

Not supported with latest reasoning models `o3` and `o4-mini`.

Up to 4 sequences where the API will stop generating further tokens. The returned text will not contain the stop sequence.

One of the following:

string

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stop%20%3E%20(schema)%20%3E%20(variant)%200)

array of string

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stop%20%3E%20(schema)%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stop%20%3E%20(schema))

store: optional boolean

Whether or not to store the output of this chat completion request for use in our [model distillation](https://developers.openai.com/docs/guides/distillation) or [evals](https://developers.openai.com/docs/guides/evals) products.

Supports text and image inputs. Note: image inputs over 8MB will be dropped.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20store%20%3E%20(schema))

stream: optional boolean

If set to true, the model response data will be streamed to the client as it is generated using [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format). See the [Streaming section below](https://developers.openai.com/docs/api-reference/chat/streaming) for more information, along with the [streaming responses](https://developers.openai.com/docs/guides/streaming-responses) guide for more information on how to handle the streaming events.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stream%20%3E%20(schema))

stream_options: optional [ChatCompletionStreamOptions](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_stream_options%20%3E%20(schema)) { include_obfuscation, include_usage } 

Options for streaming response. Only set this when you set `stream: true`.

include_obfuscation: optional boolean

When true, stream obfuscation will be enabled. Stream obfuscation adds random characters to an `obfuscation` field on streaming delta events to normalize payload sizes as a mitigation to certain side-channel attacks. These obfuscation fields are included by default, but add a small amount of overhead to the data stream. You can set `include_obfuscation` to false to optimize for bandwidth if you trust the network links between your application and the OpenAI API.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stream_options%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_stream_options%20%3E%20(schema)%20%3E%20(property)%20include_obfuscation)

include_usage: optional boolean

If set, an additional chunk will be streamed before the `data: [DONE]` message. The `usage` field on this chunk shows the token usage statistics for the entire request, and the `choices` field will always be an empty array.

All other chunks will also include a `usage` field, but with a null value. **NOTE:** If the stream is interrupted, you may not receive the final usage chunk which contains the total token usage for the request.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stream_options%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_stream_options%20%3E%20(schema)%20%3E%20(property)%20include_usage)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20stream_options%20%3E%20(schema))

temperature: optional number

What sampling temperature to use, between 0 and 2. Higher values like 0.8 will make the output more random, while lower values like 0.2 will make it more focused and deterministic. We generally recommend altering this or `top_p` but not both.

minimum 0

maximum 2

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20temperature%20%3E%20(schema))

tool_choice: optional [ChatCompletionToolChoiceOption](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_choice_option%20%3E%20(schema))

Controls which (if any) tool is called by the model. `none` means the model will not call any tool and instead generates a message. `auto` means the model can pick between generating a message or calling one or more tools. `required` means the model must call one or more tools. Specifying a particular tool via `{"type": "function", "function": {"name": "my_function"}}` forces the model to call that tool.

`none` is the default when no tools are present. `auto` is the default if tools are present.

One of the following:

ToolChoiceMode = "none"or"auto"or"required"

`none` means the model will not call any tool and instead generates a message. `auto` means the model can pick between generating a message or calling one or more tools. `required` means the model must call one or more tools.

One of the following:

"none"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_choice_option%20%3E%20(schema)%20%3E%20(variant)%200%20%3E%20(member)%200)

"auto"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_choice_option%20%3E%20(schema)%20%3E%20(variant)%200%20%3E%20(member)%201)

"required"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_choice_option%20%3E%20(schema)%20%3E%20(variant)%200%20%3E%20(member)%202)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool_choice_option%20%3E%20(schema)%20%3E%20(variant)%200)

ChatCompletionAllowedToolChoice object {allowed_tools, type} 

Constrains the tools available to the model to a pre-defined set.

allowed_tools: [ChatCompletionAllowedTools](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)) { mode, tools } 

Constrains the tools available to the model to a pre-defined set.

mode: "auto"or"required"

Constrains the tools available to the model to a pre-defined set.

`auto` allows the model to pick from among the allowed tools and generate a message.

`required` requires the model to call one or more of the allowed tools.

One of the following:

"auto"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)%20%3E%20(property)%20allowed_tools%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)%20%3E%20(property)%20mode%20%3E%20(member)%200)

"required"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)%20%3E%20(property)%20allowed_tools%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)%20%3E%20(property)%20mode%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)%20%3E%20(property)%20allowed_tools%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)%20%3E%20(property)%20mode)

tools: array of map[unknown]

A list of tool definitions that the model should be allowed to call.

For the Chat Completions API, the list of tool definitions might look like:

```
[
  { "type": "function", "function": { "name": "get_weather" } },
  { "type": "function", "function": { "name": "get_time" } }
]
```

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)%20%3E%20(property)%20allowed_tools%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)%20%3E%20(property)%20tools)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)%20%3E%20(property)%20allowed_tools)

type: "allowed_tools"

Allowed tool configuration type. Always `allowed_tools`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema))

ChatCompletionNamedToolChoice object {function, type} 

Specifies a tool the model should use. Use to force the model to call a specific function.

function: object {name} 

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice%20%3E%20(schema)%20%3E%20(property)%20function%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice%20%3E%20(schema)%20%3E%20(property)%20function)

type: "function"

For function calling, the type is always `function`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice%20%3E%20(schema))

ChatCompletionNamedToolChoiceCustom object {custom, type} 

Specifies a tool the model should use. Use to force the model to call a specific custom tool.

custom: object {name} 

name: string

The name of the custom tool to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice_custom%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice_custom%20%3E%20(schema)%20%3E%20(property)%20custom)

type: "custom"

For custom tool calling, the type is always `custom`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice_custom%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema)%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice_custom%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tool_choice%20%3E%20(schema))

tools: optional array of [ChatCompletionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_tool%20%3E%20(schema))

A list of tools the model may call. You can provide either [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools) or [function tools](https://developers.openai.com/docs/guides/function-calling).

One of the following:

ChatCompletionFunctionTool object {function, type} 

A function tool that can be used to generate a response.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict } 

name: string

The name of the function to be called. Must be a-z, A-Z, 0-9, or contain underscores and dashes, with a maximum length of 64.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)%20%3E%20(property)%20function%20%2B%20(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)%20%3E%20(property)%20name)

description: optional string

A description of what the function does, used by the model to choose when and how to call the function.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)%20%3E%20(property)%20function%20%2B%20(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)%20%3E%20(property)%20description)

parameters: optional [FunctionParameters](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_parameters%20%3E%20(schema))

The parameters the functions accepts, described as a JSON Schema object. See the [guide](https://developers.openai.com/docs/guides/function-calling) for examples, and the [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for documentation about the format.

Omitting `parameters` defines a function with an empty parameter list.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)%20%3E%20(property)%20function%20%2B%20(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)%20%3E%20(property)%20parameters)

strict: optional boolean

Whether to enable strict schema adherence when generating the function call. If set to true, the model will follow the exact schema defined in the `parameters` field. Only a subset of JSON Schema is supported when `strict` is `true`. Learn more about Structured Outputs in the [function calling guide](https://developers.openai.com/docs/guides/function-calling).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)%20%3E%20(property)%20function%20%2B%20(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)%20%3E%20(property)%20strict)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)%20%3E%20(property)%20function)

type: "function"

The type of the tool. Currently, only `function` is supported.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema))

ChatCompletionCustomTool object {custom, type} 

A custom tool that processes input using a specified format.

custom: object {name, description, format} 

Properties of the custom tool.

name: string

The name of the custom tool, used to identify it in tool calls.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20name)

description: optional string

Optional description of the custom tool, used to provide more context.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20description)

format: optional object {type} or object {grammar, type} 

The input format for the custom tool. Default is unconstrained text.

One of the following:

TextFormat object {type} 

Unconstrained free-form text.

type: "text"

Unconstrained text format. Always `text`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%200%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%200)

GrammarFormat object {grammar, type} 

A grammar defined by the user.

grammar: object {definition, syntax} 

Your chosen grammar.

definition: string

The grammar definition.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201%20%3E%20(property)%20grammar%20%3E%20(property)%20definition)

syntax: "lark"or"regex"

The syntax of the grammar definition. One of `lark` or `regex`.

One of the following:

"lark"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201%20%3E%20(property)%20grammar%20%3E%20(property)%20syntax%20%3E%20(member)%200)

"regex"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201%20%3E%20(property)%20grammar%20%3E%20(property)%20syntax%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201%20%3E%20(property)%20grammar%20%3E%20(property)%20syntax)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201%20%3E%20(property)%20grammar)

type: "grammar"

Grammar format. Always `grammar`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20format)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20custom)

type: "custom"

The type of the custom tool. Always `custom`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20tools%20%3E%20(schema))

top_logprobs: optional number

An integer between 0 and 20 specifying the maximum number of most likely tokens to return at each token position, each with an associated log probability. In some cases, the number of returned tokens may be fewer than requested. `logprobs` must be set to `true` if this parameter is used.

minimum 0

maximum 20

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20top_logprobs%20%3E%20(schema))

top_p: optional number

An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered.

We generally recommend altering this or `temperature` but not both.

minimum 0

maximum 1

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20top_p%20%3E%20(schema))

Deprecated user: optional string

This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifier for your end-users. Used to boost cache hit rates by better bucketing similar requests and to help OpenAI detect and prevent abuse. [Learn more](https://developers.openai.com/docs/guides/safety-best-practices#safety-identifiers).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20user%20%3E%20(schema))

verbosity: optional "low"or"medium"or"high"

Constrains the verbosity of the model’s response. Lower values will result in more concise responses, while higher values will result in more verbose responses. Currently supported values are `low`, `medium`, and `high`.

One of the following:

"low"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20verbosity%20%3E%20(schema)%20%3E%20(member)%200)

"medium"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20verbosity%20%3E%20(schema)%20%3E%20(member)%201)

"high"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20verbosity%20%3E%20(schema)%20%3E%20(member)%202)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20verbosity%20%3E%20(schema))

web_search_options: optional object {search_context_size, user_location} 

This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](https://developers.openai.com/docs/guides/tools-web-search?api-mode=chat).

search_context_size: optional "low"or"medium"or"high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20search_context_size%20%3E%20(member)%200)

"medium"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20search_context_size%20%3E%20(member)%201)

"high"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20search_context_size%20%3E%20(member)%202)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20search_context_size)

user_location: optional object {approximate, type} 

Approximate location parameters for the search.

approximate: object {city, country, region, timezone} 

Approximate location parameters for the search.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location%20%3E%20(property)%20approximate%20%3E%20(property)%20city)

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location%20%3E%20(property)%20approximate%20%3E%20(property)%20country)

region: optional string

Free text input for the region of the user, e.g. `California`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location%20%3E%20(property)%20approximate%20%3E%20(property)%20region)

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location%20%3E%20(property)%20approximate%20%3E%20(property)%20timezone)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location%20%3E%20(property)%20approximate)

type: "approximate"

The type of location approximation. Always `approximate`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema)%20%3E%20(property)%20user_location)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(method)%20create%20%3E%20(params)%200.non_streaming%20%3E%20(param)%20web_search_options%20%3E%20(schema))

##### Returns Expand Collapse

ChatCompletion object {id, choices, created, 6 more} 

Represents a chat completion response returned by model, based on the provided input.

id: string

A unique identifier for the chat completion.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20id)

choices: array of object {finish_reason, index, logprobs, message} 

A list of chat completion choices. Can be more than one if `n` is greater than 1.

finish_reason: "stop"or"length"or"tool_calls"or 2 more

The reason the model stopped generating tokens. This will be `stop` if the model hit a natural stop point or a provided stop sequence, `length` if the maximum number of tokens specified in the request was reached, `content_filter` if content was omitted due to a flag from our content filters, `tool_calls` if the model called a tool, or `function_call` (deprecated) if the model called a function. Read the [Model Spec](https://model-spec.openai.com/2025-12-18.html) for more.

One of the following:

"stop"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20finish_reason%20%3E%20(member)%200)

"length"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20finish_reason%20%3E%20(member)%201)

"tool_calls"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20finish_reason%20%3E%20(member)%202)

"content_filter"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20finish_reason%20%3E%20(member)%203)

"function_call"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20finish_reason%20%3E%20(member)%204)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20finish_reason)

index: number

The index of the choice in the list of choices.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20index)

logprobs: object {content, refusal} 

Log probability information for the choice.

content: array of [ChatCompletionTokenLogprob](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)) { token, bytes, logprob, top_logprobs } 

A list of message content tokens with log probability information.

token: string

The token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20token)

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20bytes)

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20logprob)

top_logprobs: array of object {token, bytes, logprob} 

List of the most likely tokens and their log probability, at this token position. The number of entries may be fewer than the requested `top_logprobs`.

token: string

The token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs%20%3E%20(items)%20%3E%20(property)%20token)

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs%20%3E%20(items)%20%3E%20(property)%20bytes)

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs%20%3E%20(items)%20%3E%20(property)%20logprob)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20logprobs%20%3E%20(property)%20content)

refusal: array of [ChatCompletionTokenLogprob](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)) { token, bytes, logprob, top_logprobs } 

A list of message refusal tokens with log probability information.

token: string

The token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20token)

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20bytes)

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20logprob)

top_logprobs: array of object {token, bytes, logprob} 

List of the most likely tokens and their log probability, at this token position. The number of entries may be fewer than the requested `top_logprobs`.

token: string

The token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs%20%3E%20(items)%20%3E%20(property)%20token)

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs%20%3E%20(items)%20%3E%20(property)%20bytes)

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs%20%3E%20(items)%20%3E%20(property)%20logprob)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)%20%3E%20(property)%20top_logprobs)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20logprobs%20%3E%20(property)%20refusal)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20logprobs)

message: [ChatCompletionMessage](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)) { content, refusal, role, 4 more } 

A chat completion message generated by the model.

content: string

The contents of the message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20content)

refusal: string

The refusal message generated by the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20refusal)

role: "assistant"

The role of the author of this message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20role)

annotations: optional array of object {type, url_citation} 

Annotations for the message, when applicable, as when using the [web search tool](https://developers.openai.com/docs/guides/tools-web-search?api-mode=chat).

type: "url_citation"

The type of the URL citation. Always `url_citation`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations%20%3E%20(items)%20%3E%20(property)%20type)

url_citation: object {end_index, start_index, title, url} 

A URL citation when using web search.

end_index: number

The index of the last character of the URL citation in the message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations%20%3E%20(items)%20%3E%20(property)%20url_citation%20%3E%20(property)%20end_index)

start_index: number

The index of the first character of the URL citation in the message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations%20%3E%20(items)%20%3E%20(property)%20url_citation%20%3E%20(property)%20start_index)

title: string

The title of the web resource.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations%20%3E%20(items)%20%3E%20(property)%20url_citation%20%3E%20(property)%20title)

url: string

The URL of the web resource.

format uri

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations%20%3E%20(items)%20%3E%20(property)%20url_citation%20%3E%20(property)%20url)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations%20%3E%20(items)%20%3E%20(property)%20url_citation)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20annotations)

audio: optional [ChatCompletionAudio](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio%20%3E%20(schema)) { id, data, expires_at, transcript } 

If the audio output modality is requested, this object contains data about the audio response from the model. [Learn more](https://developers.openai.com/docs/guides/audio).

id: string

Unique identifier for this audio response.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20audio%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio%20%3E%20(schema)%20%3E%20(property)%20id)

data: string

Base64 encoded audio bytes generated by the model, in the format specified in the request.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20audio%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio%20%3E%20(schema)%20%3E%20(property)%20data)

expires_at: number

The Unix timestamp (in seconds) for when this audio response will no longer be accessible on the server for use in multi-turn conversations.

format unixtime

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20audio%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio%20%3E%20(schema)%20%3E%20(property)%20expires_at)

transcript: string

Transcript of the audio generated by the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20audio%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio%20%3E%20(schema)%20%3E%20(property)%20transcript)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20audio)

Deprecated function_call: optional object {arguments, name} 

Deprecated and replaced by `tool_calls`. The name and arguments of a function that should be called, as generated by the model.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20function_call%20%3E%20(property)%20arguments)

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20function_call%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20function_call)

tool_calls: optional array of [ChatCompletionMessageToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_tool_call%20%3E%20(schema))

The tool calls generated by the model, such as function calls.

One of the following:

ChatCompletionMessageFunctionToolCall object {id, function, type} 

A call to a function tool created by the model.

id: string

The ID of the tool call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20id)

function: object {arguments, name} 

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20function%20%3E%20(property)%20arguments)

name: string

The name of the function to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20function%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20function)

type: "function"

The type of the tool. Currently, only `function` is supported.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema))

ChatCompletionMessageCustomToolCall object {id, custom, type} 

A call to a custom tool created by the model.

id: string

The ID of the tool call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20id)

custom: object {input, name} 

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20input)

name: string

The name of the custom tool to call.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20custom%20%3E%20(property)%20name)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20custom)

type: "custom"

The type of the tool. Always `custom`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema))

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message%20%2B%20(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)%20%3E%20(property)%20tool_calls)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices%20%3E%20(items)%20%3E%20(property)%20message)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20choices)

created: number

The Unix timestamp (in seconds) of when the chat completion was created.

format unixtime

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20created)

model: string

The model used for the chat completion.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20model)

object: "chat.completion"

The object type, which is always `chat.completion`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20object)

moderation: optional object {input, output} 

Moderation results for the request input and generated output, if moderated completions were requested.

input: object {model, results, type} or object {code, message, type} 

Moderation for the request input.

One of the following:

ModerationResults object {model, results, type} 

Successful moderation results for the request input or generated output.

model: string

The moderation model used to generate the results.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20model)

results: array of object {categories, category_applied_input_types, category_scores, 3 more} 

A list of moderation results.

categories: map[boolean]

A dictionary of moderation categories to booleans, True if the input is flagged under this category.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20categories)

category_applied_input_types: map[array of "text"or"image"]

Which modalities of input are reflected by the score for each category.

One of the following:

"text"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_applied_input_types%20%3E%20(items)%20%3E%20(items)%20%3E%20(member)%200)

"image"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_applied_input_types%20%3E%20(items)%20%3E%20(items)%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_applied_input_types)

category_scores: map[number]

A dictionary of moderation categories to scores.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_scores)

flagged: boolean

A boolean indicating whether the content was flagged by any category.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20flagged)

model: string

The moderation model that produced this result.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20model)

type: "moderation_result"

The object type, which was always `moderation_result` for successful moderation results.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20results)

type: "moderation_results"

The object type, which is always `moderation_results`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%200)

Error object {code, message, type} 

An error produced while attempting moderation.

code: string

The error code.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%201%20%3E%20(property)%20code)

message: string

The error message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%201%20%3E%20(property)%20message)

type: "error"

The object type, which is always `error`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%201%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20input)

output: object {model, results, type} or object {code, message, type} 

Moderation for the generated output.

One of the following:

ModerationResults object {model, results, type} 

Successful moderation results for the request input or generated output.

model: string

The moderation model used to generate the results.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20model)

results: array of object {categories, category_applied_input_types, category_scores, 3 more} 

A list of moderation results.

categories: map[boolean]

A dictionary of moderation categories to booleans, True if the input is flagged under this category.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20categories)

category_applied_input_types: map[array of "text"or"image"]

Which modalities of input are reflected by the score for each category.

One of the following:

"text"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_applied_input_types%20%3E%20(items)%20%3E%20(items)%20%3E%20(member)%200)

"image"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_applied_input_types%20%3E%20(items)%20%3E%20(items)%20%3E%20(member)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_applied_input_types)

category_scores: map[number]

A dictionary of moderation categories to scores.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20category_scores)

flagged: boolean

A boolean indicating whether the content was flagged by any category.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20flagged)

model: string

The moderation model that produced this result.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20model)

type: "moderation_result"

The object type, which was always `moderation_result` for successful moderation results.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results%20%3E%20(items)%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20results)

type: "moderation_results"

The object type, which is always `moderation_results`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%200)

Error object {code, message, type} 

An error produced while attempting moderation.

code: string

The error code.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%201%20%3E%20(property)%20code)

message: string

The error message.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%201%20%3E%20(property)%20message)

type: "error"

The object type, which is always `error`.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%201%20%3E%20(property)%20type)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output%20%3E%20(variant)%201)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation%20%3E%20(property)%20output)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20moderation)

service_tier: optional "auto"or"default"or"flex"or 2 more

Specifies the processing type used for serving the request.

*   If set to ‘auto’, then the request will be processed with the service tier configured in the Project settings. Unless otherwise configured, the Project will use ‘default’.
*   If set to ‘default’, then the request will be processed with the standard pricing and performance for the selected model.
*   If set to ‘[flex](https://developers.openai.com/docs/guides/flex-processing)’ or ‘[priority](https://openai.com/api-priority-processing/)’, then the request will be processed with the corresponding service tier.
*   When not set, the default behavior is ‘auto’.

When the `service_tier` parameter is set, the response body will include the `service_tier` value based on the processing mode actually used to serve the request. This response value may be different from the value set in the parameter.

One of the following:

"auto"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20service_tier%20%3E%20(member)%200)

"default"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20service_tier%20%3E%20(member)%201)

"flex"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20service_tier%20%3E%20(member)%202)

"scale"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20service_tier%20%3E%20(member)%203)

"priority"

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20service_tier%20%3E%20(member)%204)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20service_tier)

Deprecated system_fingerprint: optional string

This fingerprint represents the backend configuration that the model runs with.

Can be used in conjunction with the `seed` request parameter to understand when backend changes have been made that might impact determinism.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20system_fingerprint)

usage: optional [CompletionUsage](https://developers.openai.com/api/reference/resources/completions#(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)) { completion_tokens, prompt_tokens, total_tokens, 2 more } 

Usage statistics for the completion request.

completion_tokens: number

Number of tokens in the generated completion.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20completion_tokens)

prompt_tokens: number

Number of tokens in the prompt.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20prompt_tokens)

total_tokens: number

Total number of tokens used in the request (prompt + completion).

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20total_tokens)

completion_tokens_details: optional object {accepted_prediction_tokens, audio_tokens, reasoning_tokens, rejected_prediction_tokens} 

Breakdown of tokens used in a completion.

accepted_prediction_tokens: optional number

When using Predicted Outputs, the number of tokens in the prediction that appeared in the completion.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20completion_tokens_details%20%3E%20(property)%20accepted_prediction_tokens)

audio_tokens: optional number

Audio input tokens generated by the model.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20completion_tokens_details%20%3E%20(property)%20audio_tokens)

reasoning_tokens: optional number

Tokens generated by the model for reasoning.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20completion_tokens_details%20%3E%20(property)%20reasoning_tokens)

rejected_prediction_tokens: optional number

When using Predicted Outputs, the number of tokens in the prediction that did not appear in the completion. However, like reasoning tokens, these tokens are still counted in the total completion tokens for purposes of billing, output, and context window limits.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20completion_tokens_details%20%3E%20(property)%20rejected_prediction_tokens)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20completion_tokens_details)

prompt_tokens_details: optional object {audio_tokens, cached_tokens} 

Breakdown of tokens used in the prompt.

audio_tokens: optional number

Audio input tokens present in the prompt.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20prompt_tokens_details%20%3E%20(property)%20audio_tokens)

cached_tokens: optional number

Cached tokens present in the prompt.

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20prompt_tokens_details%20%3E%20(property)%20cached_tokens)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage%20%2B%20(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)%20%3E%20(property)%20prompt_tokens_details)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage)

[](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema))

Default Image input Streaming Functions Logprobs 

### Create chat completion

HTTP

HTTP HTTP

HTTP HTTP

TypeScript TypeScript

Python Python

Java Java

Go Go

Ruby Ruby

CLI Tool CLI Tool

```
curl https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "VAR_chat_model_id",
    "messages": [
      {
        "role": "developer",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Hello!"
      }
    ]
  }'
```

```
{
  "id": "chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT",
  "object": "chat.completion",
  "created": 1741569952,
  "model": "gpt-5.4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?",
        "refusal": null,
        "annotations": []
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 19,
    "completion_tokens": 10,
    "total_tokens": 29,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  },
  "service_tier": "default"
}
```

### Create chat completion

HTTP

HTTP HTTP

HTTP HTTP

TypeScript TypeScript

Python Python

Java Java

Go Go

Ruby Ruby

CLI Tool CLI Tool

```
curl https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "What is in this image?"
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/2560px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg"
            }
          }
        ]
      }
    ],
    "max_tokens": 300
  }'
```

```
{
  "id": "chatcmpl-B9MHDbslfkBeAs8l4bebGdFOJ6PeG",
  "object": "chat.completion",
  "created": 1741570283,
  "model": "gpt-5.4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The image shows a wooden boardwalk path running through a lush green field or meadow. The sky is bright blue with some scattered clouds, giving the scene a serene and peaceful atmosphere. Trees and shrubs are visible in the background.",
        "refusal": null,
        "annotations": []
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 1117,
    "completion_tokens": 46,
    "total_tokens": 1163,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  },
  "service_tier": "default"
}
```

### Create chat completion

HTTP

HTTP HTTP

HTTP HTTP

TypeScript TypeScript

Python Python

Java Java

Go Go

Ruby Ruby

CLI Tool CLI Tool

```
curl https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "VAR_chat_model_id",
    "messages": [
      {
        "role": "developer",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Hello!"
      }
    ],
    "stream": true
  }'
```

```
{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o-mini", "system_fingerprint": "fp_44709d6fcb", "choices":[{"index":0,"delta":{"role":"assistant","content":""},"logprobs":null,"finish_reason":null}]}

{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o-mini", "system_fingerprint": "fp_44709d6fcb", "choices":[{"index":0,"delta":{"content":"Hello"},"logprobs":null,"finish_reason":null}]}

....

{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o-mini", "system_fingerprint": "fp_44709d6fcb", "choices":[{"index":0,"delta":{},"logprobs":null,"finish_reason":"stop"}]}
```

### Create chat completion

HTTP

HTTP HTTP

HTTP HTTP

TypeScript TypeScript

Python Python

Java Java

Go Go

Ruby Ruby

CLI Tool CLI Tool

```
curl https://api.openai.com/v1/chat/completions \
-H "Content-Type: application/json" \
-H "Authorization: Bearer $OPENAI_API_KEY" \
-d '{
  "model": "gpt-5.4",
  "messages": [
    {
      "role": "user",
      "content": "What is the weather like in Boston today?"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_current_weather",
        "description": "Get the current weather in a given location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state, e.g. San Francisco, CA"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}'
```

```
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1699896916,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_current_weather",
              "arguments": "{\n\"location\": \"Boston, MA\"\n}"
            }
          }
        ]
      },
      "logprobs": null,
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 82,
    "completion_tokens": 17,
    "total_tokens": 99,
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  }
}
```

### Create chat completion

HTTP

HTTP HTTP

HTTP HTTP

TypeScript TypeScript

Python Python

Java Java

Go Go

Ruby Ruby

CLI Tool CLI Tool

```
curl https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "VAR_chat_model_id",
    "messages": [
      {
        "role": "user",
        "content": "Hello!"
      }
    ],
    "logprobs": true,
    "top_logprobs": 2
  }'
```

```
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1702685778,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?"
      },
      "logprobs": {
        "content": [
          {
            "token": "Hello",
            "logprob": -0.31725305,
            "bytes": [72, 101, 108, 108, 111],
            "top_logprobs": [
              {
                "token": "Hello",
                "logprob": -0.31725305,
                "bytes": [72, 101, 108, 108, 111]
              },
              {
                "token": "Hi",
                "logprob": -1.3190403,
                "bytes": [72, 105]
              }
            ]
          },
          {
            "token": "!",
            "logprob": -0.02380986,
            "bytes": [
              33
            ],
            "top_logprobs": [
              {
                "token": "!",
                "logprob": -0.02380986,
                "bytes": [33]
              },
              {
                "token": " there",
                "logprob": -3.787621,
                "bytes": [32, 116, 104, 101, 114, 101]
              }
            ]
          },
          {
            "token": " How",
            "logprob": -0.000054669687,
            "bytes": [32, 72, 111, 119],
            "top_logprobs": [
              {
                "token": " How",
                "logprob": -0.000054669687,
                "bytes": [32, 72, 111, 119]
              },
              {
                "token": "<|end|>",
                "logprob": -10.953937,
                "bytes": null
              }
            ]
          },
          {
            "token": " can",
            "logprob": -0.015801601,
            "bytes": [32, 99, 97, 110],
            "top_logprobs": [
              {
                "token": " can",
                "logprob": -0.015801601,
                "bytes": [32, 99, 97, 110]
              },
              {
                "token": " may",
                "logprob": -4.161023,
                "bytes": [32, 109, 97, 121]
              }
            ]
          },
          {
            "token": " I",
            "logprob": -3.7697225e-6,
            "bytes": [
              32,
              73
            ],
            "top_logprobs": [
              {
                "token": " I",
                "logprob": -3.7697225e-6,
                "bytes": [32, 73]
              },
              {
                "token": " assist",
                "logprob": -13.596657,
                "bytes": [32, 97, 115, 115, 105, 115, 116]
              }
            ]
          },
          {
            "token": " assist",
            "logprob": -0.04571125,
            "bytes": [32, 97, 115, 115, 105, 115, 116],
            "top_logprobs": [
              {
                "token": " assist",
                "logprob": -0.04571125,
                "bytes": [32, 97, 115, 115, 105, 115, 116]
              },
              {
                "token": " help",
                "logprob": -3.1089056,
                "bytes": [32, 104, 101, 108, 112]
              }
            ]
          },
          {
            "token": " you",
            "logprob": -5.4385737e-6,
            "bytes": [32, 121, 111, 117],
            "top_logprobs": [
              {
                "token": " you",
                "logprob": -5.4385737e-6,
                "bytes": [32, 121, 111, 117]
              },
              {
                "token": " today",
                "logprob": -12.807695,
                "bytes": [32, 116, 111, 100, 97, 121]
              }
            ]
          },
          {
            "token": " today",
            "logprob": -0.0040071653,
            "bytes": [32, 116, 111, 100, 97, 121],
            "top_logprobs": [
              {
                "token": " today",
                "logprob": -0.0040071653,
                "bytes": [32, 116, 111, 100, 97, 121]
              },
              {
                "token": "?",
                "logprob": -5.5247097,
                "bytes": [63]
              }
            ]
          },
          {
            "token": "?",
            "logprob": -0.0008108172,
            "bytes": [63],
            "top_logprobs": [
              {
                "token": "?",
                "logprob": -0.0008108172,
                "bytes": [63]
              },
              {
                "token": "?\n",
                "logprob": -7.184561,
                "bytes": [63, 10]
              }
            ]
          }
        ]
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 9,
    "total_tokens": 18,
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  },
  "system_fingerprint": null
}
```

##### Returns Examples

```
{
  "id": "chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT",
  "object": "chat.completion",
  "created": 1741569952,
  "model": "gpt-5.4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?",
        "refusal": null,
        "annotations": []
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 19,
    "completion_tokens": 10,
    "total_tokens": 29,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  },
  "service_tier": "default"
}
```
