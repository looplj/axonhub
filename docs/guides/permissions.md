# Fine-grained Permission

## Overview
AxonHub provides role-based access control (RBAC) so that organizations can tailor API access, feature visibility, and resource quotas to specific teams or workloads. Fine-grained rules allow administrators to enforce least-privilege policies, protect sensitive data, and monitor usage across projects.

## Key Concepts
- **Roles** – Collections of permissions that define a user or API key's capabilities.
- **Scopes** – Granular privileges such as managing channels, issuing API keys, or viewing traces.
- **Projects** – Logical containers that tie together datasets, model profiles, and API activity.
- **API Keys** – Tokens issued per project or user that inherit role scopes and can be rotated at any time.

## Common Policies
1. **Separation of Duties** – Assign operational teams read-only access to traces while keeping configuration changes limited to administrators.
2. **Quota Guardrails** – Combine rate limits and per-model cost ceilings to prevent runaway spend.
3. **Environment Isolation** – Create dedicated projects for staging and production, mapping distinct model profiles and upstream credentials.

## Best Practices
- Rotate API keys regularly and revoke unused credentials from the admin console.
- Use service accounts with minimal scopes for automation pipelines and CI/CD flows.
- Enable auditing to capture every administrative change for compliance investigations.

## Related Resources
- [Chat Completions API](../api-reference/chat-completions.md)
- [Claude Code & Codex Integration](claude-code-integration.md)

---

# 细粒度权限

## 概述
AxonHub 通过基于角色的访问控制（RBAC）为组织提供精细化的权限管理，便于按团队或业务场景管控 API 访问、功能可见性和资源配额。管理员可以以最小必要权限策略保护敏感数据，同时追踪每个项目的使用情况。

## 核心概念
- **角色**：一组权限的集合，定义用户或 API 密钥可以执行的操作。
- **作用域**：更细粒度的权限点，例如渠道管理、API 密钥签发、追踪查看等。
- **项目**：数据集、模型配置文件和 API 活动的逻辑容器，支持跨团队隔离。
- **API 密钥**：按项目或用户签发的访问令牌，继承角色作用域，可随时轮换。

## 常见策略
1. **职责分离**：为运维团队配置只读追踪权限，将配置变更权限限制给管理员。
2. **配额防护**：结合调用频次限制和模型成本上限，避免预算超支。
3. **环境隔离**：为测试和生产环境创建独立项目，分别绑定不同的模型配置文件和上游凭证。

## 最佳实践
- 定期轮换 API 密钥，并在管理后台删除不再使用的凭证。
- 为自动化流水线和 CI/CD 工作流使用权限最小的服务账号。
- 启用审计日志，记录每一次后台管理操作，满足合规检查需求。

## 相关资源
- [Chat Completions API](../api-reference/chat-completions.md)
- [Claude Code / Codex 集成指南](claude-code-integration.md)
