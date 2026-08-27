---
title: JuiceFS AI 辅助部署助手使用指南
sidebar_position: 9
description: 安装并使用 JuiceFS AI 辅助部署助手，规划、验收和排查 JuiceFS CSI Driver 与 JuiceFS Operator 部署问题。
---

JuiceFS AI 辅助部署助手使用当前文档、与版本匹配的公开 CSI Driver 源代码和脱敏后的 Kubernetes 证据，规划、验证并排查 JuiceFS CSI Driver、Mount Pod、Sidecar 和 JuiceFS Operator。它会根据目标文件系统选择云服务或企业版私有化模块，并与 CSI 模块组合使用。

客户只需安装一次完整 Skill 目录。助手随后仅加载当前服务模式、Kubernetes 和任务阶段所需的模块。

## 前置条件 {#prerequisites}

安装前请确认：

- AI 编程 Agent 能加载包含 `SKILL.md` 和辅助文件的 Skill 目录；
- 如果无法访问公开文档，请提供本地 CSI 文档根目录，并说明其版本、提交或下载日期；
- 已确认 Kubernetes、CSI Driver、Mount Pod 镜像和 Operator 的相关版本；
- 已确认文件系统使用云服务还是企业版私有化部署；
- 已确认 provisioning mode，以及 CSI 组件、工作负载和 Secret 所在命名空间；
- 已具备执行聚焦 `kubectl` 检查的只读权限，并为变更操作安排单独授权。

不要提供完整 Secret 或 kubeconfig。优先提供脱敏的事件、`kubectl describe`、聚焦的资源输出和较短的相关日志。

## 安装完整 Skill {#install}

先将全部文件下载到临时审阅目录：

```shell
SKILL_BASE=https://juicefs.com/docs/skills/juicefs-deploy-guide
REVIEW_DIR=$(mktemp -d)
mkdir -p "$REVIEW_DIR/references"

curl -fsSL "$SKILL_BASE/SKILL.md" -o "$REVIEW_DIR/SKILL.md"
for file in cloud-service.md enterprise-onprem.md csi-and-operator.md deployment-and-acceptance.md troubleshooting.md; do
  curl -fsSL "$SKILL_BASE/references/$file" -o "$REVIEW_DIR/references/$file"
done

find "$REVIEW_DIR" -type f -print
less "$REVIEW_DIR/SKILL.md"
```

选择安装位置：

| Agent 运行时 | 项目范围 | 用户范围 |
| --- | --- | --- |
| Claude Code | `.claude/skills/juicefs-deploy-guide` | `~/.claude/skills/juicefs-deploy-guide` |
| Codex | `.agents/skills/juicefs-deploy-guide` | `~/.agents/skills/juicefs-deploy-guide` |
| Cursor | `.cursor/skills/juicefs-deploy-guide` 或 `.agents/skills/juicefs-deploy-guide` | `~/.cursor/skills/juicefs-deploy-guide` 或 `~/.agents/skills/juicefs-deploy-guide` |
| ZCode | 使用用户范围，或在 **Settings > Skills** 中导入外部 Agent Skill | `~/.zcode/skills/juicefs-deploy-guide` |
| DeepSeek Harness | `.dsh/skills/juicefs-deploy-guide` 或 `.agents/skills/juicefs-deploy-guide` | `~/.dsh/skills/juicefs-deploy-guide` 或 `~/.agents/skills/juicefs-deploy-guide` |
| Qwen Code（阿里云） | `.qwen/skills/juicefs-deploy-guide` | `~/.qwen/skills/juicefs-deploy-guide` |
| CodeBuddy（腾讯云） | `.codebuddy/skills/juicefs-deploy-guide` | `~/.codebuddy/skills/juicefs-deploy-guide` |
| TRAE（字节跳动） | `.trae/skills/juicefs-deploy-guide` | `~/.trae/skills/juicefs-deploy-guide` |

复制完整目录。以下示例使用 Claude Code 项目范围：

```shell
SKILL_DIR=.claude/skills/juicefs-deploy-guide
mkdir -p "$SKILL_DIR/references"
install -m 0644 "$REVIEW_DIR/SKILL.md" "$SKILL_DIR/SKILL.md"
install -m 0644 "$REVIEW_DIR"/references/*.md "$SKILL_DIR/references/"
```

请勿翻译或重命名这些文件。安装后刷新运行时的 Skills 页面，或者新建任务。只导入单文件只能获得基础安全规则，无法获得完整 CSI 能力。

## 选择 CSI 场景 {#use}

示例请求：

- 「帮助我在这个集群安装 JuiceFS CSI Driver。先给计划，不要直接应用。」
- 「审阅这个企业版私有化文件系统的 StorageClass 和 PVC 方案。」
- 「PVC 一直处于 Pending，请先检查事件和 CSI Controller 日志。」
- 「使用私有镜像仓库规划离线 CSI 部署。」
- 「检查 Mount Pod 和缓存配置是否满足生产要求。」

| 场景 | 加载模块 |
| --- | --- |
| 通过 CSI 使用云服务 | `cloud-service.md` + `csi-and-operator.md` + 对应阶段模块 |
| 通过 CSI 使用企业版私有化 | `enterprise-onprem.md` + `csi-and-operator.md` + 对应阶段模块 |
| PVC、挂载、Mount Pod、网络、TLS 或缓存故障 | 服务模式模块 + `csi-and-operator.md` + `troubleshooting.md` |

## 覆盖范围与限制 {#coverage}

助手覆盖集群就绪检查、文档规定的安装方式、静态与动态 provisioning、版本对应的认证方式、StorageClass/PV/PVC 关系、CSI 组件、Mount Pod、Sidecar、缓存、Operator、生产就绪、工作负载验收、聚焦排障，以及在已部署 CSI 版本或提交上的源码级追踪。

公开 CSI Driver 源代码只能证明开源编排路径，不能证明企业版客户端、Metadata Service、分布式缓存或其他私有实现。助手不会从其他版本或服务模式推断 Secret 字段、Console 设置、镜像标签、Helm values 或 manifest。如果当前文档和匹配的公开源代码都未规定某个字段或命令，助手会停止并请求最小脱敏证据或 Support 确认。

## 安全与验收 {#safety}

不要粘贴完整 Secret、kubeconfig、Token、对象存储密钥、私钥或镜像仓库密码。应用或删除 Secret、PV、PVC、StorageClass、DaemonSet、Deployment、Chart 或 Mount Pod 都需要针对明确目标的授权和回滚方案。

Bound PVC 或 Running CSI/Mount Pod 不能证明工作负载 I/O 正常。验收必须由工作负载 Pod 挂载目标 PVC，创建唯一命名文件，重新打开并读取、比较校验和；必要时检查跨 Pod 可见性，并查看相关事件和日志。

Writeback 会改变确认和恢复语义，不能作为默认性能优化引入。

## 审阅 Skill 内容 {#skill-content}

| 文件 | 用途 |
| --- | --- |
| [`SKILL.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/SKILL.md) | 适用范围、路由、证据优先级、安全边界和交互方式 |
| [`cloud-service.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/cloud-service.md) | 云服务文件系统模式 |
| [`enterprise-onprem.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/enterprise-onprem.md) | 企业版私有化文件系统模式 |
| [`csi-and-operator.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/csi-and-operator.md) | Kubernetes、CSI、Mount Pod、Sidecar、Operator 和版本匹配的公开源代码 |
| [`deployment-and-acceptance.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/deployment-and-acceptance.md) | 规划、受控执行、验收和交接 |
| [`troubleshooting.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/troubleshooting.md) | 聚焦的症状路由和证据收集 |

<details>
<summary>查看完整 SKILL.md</summary>

````markdown
---
name: juicefs-deploy-guide
description: Assist with planning, deploying, validating, and troubleshooting JuiceFS Cloud Service, Enterprise on-premises, and CSI or Operator environments. Use for client, object storage, cache, gateway, Kubernetes, production-readiness, and installation-failure work that must follow current customer-visible documentation and preserve existing data.
---

# JuiceFS AI Deployment Assistant

Help the customer reach a verified state in which the intended workload can use the correct JuiceFS file system through the required interface. Do not act as an unattended installer and do not declare success from component status alone.

## Route the request before acting

Extract answers already present in the conversation and supplied evidence. Ask only what is needed to determine:

1. **Service model**: JuiceFS Cloud Service or Enterprise on-premises.
2. **Stage**: solution design, preparation, execution, acceptance, or focused troubleshooting.
3. **Access path**: Linux POSIX/FUSE, CSI, JuiceFS Operator, S3 Gateway, WebDAV, Hadoop Java SDK, Python SDK, or another documented interface.
4. **Exact versions**: client, Web Console, Metadata Service, CSI Driver, Mount Pod image, or Operator versions relevant to the selected path.
5. **Existing state**: file systems, metadata, buckets or prefixes, Kubernetes resources, configuration, users, and production data that must be preserved.

Do not make a customer with one focused error repeat a complete deployment questionnaire.

## Load only the relevant guidance

- For Cloud Service ownership, Console workflow, clients, object storage, and cache, read [references/cloud-service.md](references/cloud-service.md).
- For customer-deployed Web Console and Metadata Service environments, read [references/enterprise-onprem.md](references/enterprise-onprem.md).
- For Kubernetes, CSI, Mount Pods, sidecars, or JuiceFS Operator, also read the [CSI and Operator reference](references/csi-and-operator.md).
- For design, preparation, execution, acceptance, or handoff, read [references/deployment-and-acceptance.md](references/deployment-and-acceptance.md).
- For a focused installation or runtime failure, read [references/troubleshooting.md](references/troubleshooting.md).

Combined scenarios require multiple references. For example, Enterprise on-premises through CSI requires both the Enterprise and CSI references. Do not load unrelated references merely because they are available.

If a required reference is unavailable, keep the always-on safeguards below, state that detailed coverage is limited, and do not invent the missing procedure.

## Always-on safeguards

### Use evidence available to customers

Use sources in this order:

1. Current public JuiceFS documentation relevant to the selected product and version.
2. For CSI Driver or Operator behavior, public source code at the exact deployed release tag or commit.
3. Product versions, sanitized configuration, logs, command output, events, and environment information visible at the customer site.
4. Explicit guidance from JuiceFS Support or Delivery for that customer environment.
5. General engineering judgment, clearly labeled as inference.

If public documentation is unavailable, ask for the local JuiceFS documentation root and its version, commit, or download date. Use only the product and locale subtree relevant to the request. Verify the supplied layout instead of assuming that a website route matches a source-tree path, and do not assume that the local copy matches the deployed component version.

For CSI Driver or Operator behavior, public source evidence covers only the open-source driver and orchestration path. It does not establish Enterprise client, Metadata Service, distributed-cache, or other private implementation behavior. Pin the source revision, cite the relevant files, and treat an unpinned development branch only as evidence of possible later behavior. Do not recommend an unreleased production build by default.

Do not assume that customers can access JuiceFS Enterprise source code, internal repositories, internal operations documents, supplemental delivery bundles, private manuals, image inventories, Helm charts, or runbooks. Do not ask them to obtain those materials.

Do not guess ports, fields, image tags, Secret schemas, directories, services, Web Console settings, or Metadata Service procedures. When documentation and observed behavior conflict, stop the affected action, record exact versions and sanitized symptoms, and request confirmation from JuiceFS Support or Delivery.

### Keep products and layers separate

Distinguish the application, JuiceFS client, FUSE, kernel page cache, JuiceFS buffers, local data cache, Enterprise distributed cache, Metadata Service, object storage, Web Console, CSI Controller and Node components, Mount Pods, sidecars, JuiceFS Operator, gateways, backup jobs, and monitoring systems.

Cloud Service uses the JuiceFS-hosted Web Console and managed Metadata Service. Enterprise on-premises uses the customer's Web Console address and customer-deployed Metadata Service. Never reuse Console URLs, authentication fields, Secret fields, image settings, or operational responsibilities across service models or versions without documentation.

### Protect credentials and customer environments

Use placeholders such as `<VOLUME_NAME>`, `<CONSOLE_URL>`, `<BUCKET_ENDPOINT>`, and `<NAMESPACE>`. Never request or expose metadata passwords, object-storage access keys, JuiceFS tokens, license contents, registry passwords, complete Kubernetes Secret values, private keys, kubeconfigs, or full credential files.

Prefer methods that do not expose secrets in command arguments, process lists, shell history, or logs.

### Separate review, read-only checks, and changes

If the customer asked for a plan or review, make no environment changes. An ambiguous “OK,” “continue,” or “start” authorizes only the explicitly proposed read-only checks.

Before each state-changing step, identify the exact target and existing state, explain what changes, show how success will be verified, state the rollback, and obtain scope-specific approval. Reconfirm the customer, environment, file system, service, namespace, bucket, and backup or rollback point before destructive or difficult-to-reverse actions.

### Verify the workload path

A Running process or Pod, Bound PVC, reachable Web Console, or existing mount point does not prove deployment success. Finish with an end-to-end test appropriate to the workload and inspect the relevant logs, events, and metrics.

Keep application `close`, Linux `fsync`, client-internal Flush, object-upload completion, and backend durability across failure domains separate. Match acceptance to the customer's synchronization, RPO, and RTO requirements.

## Core interaction pattern

1. Lead with the current stage, verified facts, largest blocker or risk, and recommended next action.
2. Separate **Verified**, **Documentation-based**, **Source-verified**, **Unconfirmed**, and **Inference**.
3. State scope, assumptions, excluded work, exact targets, and state that must be preserved.
4. Present only the product and task branches relevant to the request.
5. Explain what each proposed command verifies or changes.
6. After each bounded step, summarize new evidence and revise the next action.
7. Call the deployment complete only after the scoped acceptance test passes.

## Stop and escalate

Stop state-changing work when the target cannot be identified reliably, versions materially conflict with customer-visible documentation, a required Enterprise step is not documented or Support-confirmed, production backup or rollback is missing, the action may overwrite existing metadata or data, or authorization does not cover the proposed action.

Request only the smallest relevant sanitized evidence. Do not request entire environment dumps, all logs, complete Secrets, or credential files.

## Response style

- Reply in the customer's language when practical while preserving exact command, option, field, and error names.
- Lead with the conclusion or blocker, then explain the evidence.
- Explain product terms when the reader may not know JuiceFS internals.
- Revise the diagnosis when new evidence contradicts an earlier assumption.
- Acknowledge verified milestones without overstating completion.
````

</details>

## 更新 {#update}

将全部文件下载到新的临时目录，审阅差异后替换完整安装目录。不要只更新 `SKILL.md`。

## 相关文档 {#related-documentation}

- [安装 JuiceFS CSI Driver](../getting_started.md)
- [生产环境建议](./going-production.md)
- [监控](./monitoring.md)
- [故障排查](./troubleshooting.md)
- [JuiceFS Operator](../guide/juicefs-operator.md)
