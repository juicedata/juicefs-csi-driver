---
title: JuiceFS AI Deployment Assistant User Guide
sidebar_position: 9
description: Install and use the JuiceFS AI Deployment Assistant for JuiceFS CSI Driver and JuiceFS Operator deployment, acceptance, and troubleshooting.
---

The JuiceFS AI Deployment Assistant uses current documentation, version-matched public CSI Driver source code, and sanitized Kubernetes evidence to plan, verify, and troubleshoot JuiceFS CSI Driver, Mount Pods, sidecars, and JuiceFS Operator. It combines the CSI module with either the Cloud Service or Enterprise on-premises module that matches the target file system.

Install the complete skill directory once. The assistant then loads only the service-model, Kubernetes, and task-stage modules required by the request.

## Prerequisites {#prerequisites}

Before installation:

- use an AI coding agent that can load a skill directory containing `SKILL.md` and supporting files;
- ensure the agent can access the official online [JuiceFS CSI Driver documentation](https://juicefs.com/docs/csi/introduction), [CSI Driver source](https://github.com/juicedata/juicefs-csi-driver), and [JuiceFS Operator source](https://github.com/juicedata/juicefs-operator);
- identify the Kubernetes, CSI Driver, Mount Pod image, and Operator versions relevant to the task;
- know whether the file system uses Cloud Service or Enterprise on-premises;
- know the provisioning mode and namespaces for CSI components, workloads, and referenced Secrets;
- arrange read access for focused `kubectl` checks and separate authorization for changes.

Do not provide a complete Secret or kubeconfig. Prefer sanitized events, `kubectl describe`, focused resource output, and short relevant log tails.

## Install the complete skill {#install}

Download all files to a temporary review directory:

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

Choose an installation target:

| Agent runtime | Project scope | User scope |
| --- | --- | --- |
| Claude Code | `.claude/skills/juicefs-deploy-guide` | `~/.claude/skills/juicefs-deploy-guide` |
| Codex | `.agents/skills/juicefs-deploy-guide` | `~/.agents/skills/juicefs-deploy-guide` |
| Cursor | `.cursor/skills/juicefs-deploy-guide` or `.agents/skills/juicefs-deploy-guide` | `~/.cursor/skills/juicefs-deploy-guide` or `~/.agents/skills/juicefs-deploy-guide` |
| ZCode | Use user scope or import an external Agent Skill from **Settings > Skills** | `~/.zcode/skills/juicefs-deploy-guide` |
| DeepSeek Harness | `.dsh/skills/juicefs-deploy-guide` or `.agents/skills/juicefs-deploy-guide` | `~/.dsh/skills/juicefs-deploy-guide` or `~/.agents/skills/juicefs-deploy-guide` |
| Qwen Code (Alibaba Cloud) | `.qwen/skills/juicefs-deploy-guide` | `~/.qwen/skills/juicefs-deploy-guide` |
| CodeBuddy (Tencent Cloud) | `.codebuddy/skills/juicefs-deploy-guide` | `~/.codebuddy/skills/juicefs-deploy-guide` |
| TRAE (ByteDance) | `.trae/skills/juicefs-deploy-guide` | `~/.trae/skills/juicefs-deploy-guide` |

Copy the complete directory. This example uses Claude Code project scope:

```shell
SKILL_DIR=.claude/skills/juicefs-deploy-guide
mkdir -p "$SKILL_DIR/references"
install -m 0644 "$REVIEW_DIR/SKILL.md" "$SKILL_DIR/SKILL.md"
install -m 0644 "$REVIEW_DIR"/references/*.md "$SKILL_DIR/references/"
```

Do not translate or rename the files. Refresh the runtime's Skills page or start a new task after installation. Single-file import provides only the core safeguards, not complete CSI coverage.

## Choose a CSI scenario {#use}

Example requests:

- “Help me install JuiceFS CSI Driver in this cluster. Plan first and do not apply anything.”
- “Review this StorageClass and PVC plan for an Enterprise on-premises file system.”
- “My PVC is Pending. Inspect events and CSI Controller logs before proposing changes.”
- “Compare this behavior with the source code for our deployed CSI Driver release.”
- “Check whether our Mount Pod and cache configuration are production-ready.”

| Scenario | Modules loaded |
| --- | --- |
| Cloud Service through CSI | `cloud-service.md` + `csi-and-operator.md` + the matching stage module |
| Enterprise on-premises through CSI | `enterprise-onprem.md` + `csi-and-operator.md` + the matching stage module |
| Focused PVC, mount, Mount Pod, network, TLS, or cache failure | Service-model module + `csi-and-operator.md` + `troubleshooting.md` |

## Coverage and limits {#coverage}

The assistant covers cluster readiness, documented installation, static and dynamic provisioning, edition-specific authentication, StorageClass/PV/PVC relationships, CSI components, Mount Pods, sidecars, cache, Operator, production readiness, workload acceptance, focused troubleshooting, and source-level tracing at the deployed CSI release or commit. It uses official online documentation and source by default. Only when the customer explicitly identifies a modified fork may it use the customer-provided repository URL or local source root and exact commit, labeling the result as customer-modified behavior.

Public CSI Driver source establishes only the open-source orchestration path; it does not establish Enterprise client, Metadata Service, distributed-cache, or other private implementation behavior. The assistant does not infer Secret fields, Console settings, image tags, Helm values, or manifests from another edition or version. If current documentation and matching public source do not establish a field or command, it stops and requests minimal sanitized evidence or Support confirmation.

## Safety and acceptance {#safety}

Never paste complete Secrets, kubeconfigs, tokens, object-storage keys, private keys, or registry passwords. Applying or deleting a Secret, PV, PVC, StorageClass, DaemonSet, Deployment, chart, or Mount Pod requires a target-specific approval and rollback.

A Bound PVC or Running CSI or Mount Pod does not prove workload I/O. Acceptance uses a workload Pod mounting the intended PVC, writing a uniquely named file, reopening and reading it, comparing a checksum, checking cross-Pod visibility when required, and inspecting relevant events and logs.

Writeback changes acknowledgement and recovery behavior and must not be introduced as a default performance optimization.

## Review the skill contents {#skill-content}

| File | Purpose |
| --- | --- |
| [`SKILL.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/SKILL.md) | Scope, routing, evidence order, safety boundaries, and interaction pattern |
| [`cloud-service.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/cloud-service.md) | Cloud Service file-system model |
| [`enterprise-onprem.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/enterprise-onprem.md) | Enterprise on-premises file-system model |
| [`csi-and-operator.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/csi-and-operator.md) | Kubernetes, CSI, Mount Pods, sidecars, Operator, and version-matched public source |
| [`deployment-and-acceptance.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/deployment-and-acceptance.md) | Planning, controlled execution, acceptance, and handoff |
| [`troubleshooting.md`](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/troubleshooting.md) | Focused symptom routing and evidence collection |

<details>
<summary>View the complete SKILL.md</summary>

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

- For Cloud Service ownership, Console workflow, clients, object storage, and cache, read the [Cloud Service reference](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/cloud-service.md).
- For customer-deployed Web Console and Metadata Service environments, read the [Enterprise on-premises reference](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/enterprise-onprem.md).
- For Kubernetes, CSI, Mount Pods, sidecars, or JuiceFS Operator, also read the [CSI and Operator reference](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/csi-and-operator.md).
- For design, preparation, execution, acceptance, or handoff, read the [deployment and acceptance reference](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/deployment-and-acceptance.md).
- For a focused installation or runtime failure, read the [troubleshooting reference](https://juicefs.com/docs/skills/juicefs-deploy-guide/references/troubleshooting.md).

Combined scenarios require multiple references. For example, Enterprise on-premises through CSI requires both the Enterprise and CSI references. Do not load unrelated references merely because they are available.

If an official online reference cannot be reached, report the exact URL and access failure. Ask the customer to restore access or contact JuiceFS Support or Delivery; do not silently substitute a local checkout or invent the missing procedure.

## Always-on safeguards

### Use evidence available to customers

Use sources in this order:

1. Current official online JuiceFS documentation relevant to the selected product and version.
2. For CSI Driver or Operator behavior, official public source code at the exact deployed release tag or commit.
3. Product versions, sanitized configuration, logs, command output, events, and environment information visible at the customer site.
4. Explicit guidance from JuiceFS Support or Delivery for that customer environment.
5. General engineering judgment, clearly labeled as inference.

Use official online documentation and public source URLs by default. Do not ask for or infer a local documentation or source root. Only when the customer explicitly states that the deployment uses a modified fork may the customer provide that fork's repository URL or local source root and exact commit. Label all resulting findings as customer-modified behavior and compare them with the official online documentation and source.

For CSI Driver or Operator behavior, inspect the official online [JuiceFS CSI Driver repository](https://github.com/juicedata/juicefs-csi-driver) and [JuiceFS Operator repository](https://github.com/juicedata/juicefs-operator). Public source evidence covers only the open-source driver and orchestration path. It does not establish Enterprise client, Metadata Service, distributed-cache, or other private implementation behavior. Pin the source revision, cite the repository, revision, file, and symbol or lines, and treat an unpinned development branch only as evidence of possible later behavior. Do not recommend an unreleased production build by default.

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

## Update {#update}

Download all files to a new temporary directory, review the changes, and replace the complete installed directory. Do not update only `SKILL.md`.

## Related documentation {#related-documentation}

- [Install JuiceFS CSI Driver](https://juicefs.com/docs/csi/getting_started)
- [Production recommendations](https://juicefs.com/docs/csi/administration/going-production)
- [Monitoring](https://juicefs.com/docs/csi/monitoring)
- [Troubleshooting](https://juicefs.com/docs/csi/troubleshooting)
- [JuiceFS Operator](https://juicefs.com/docs/csi/guide/juicefs-operator)
