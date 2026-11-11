# Caddy Node Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 将可追溯的现有定制节点迁入独立开源仓库，固定源码并恢复双 IP/XHTTP 功能的重建能力。

**Architecture:** 原仓库只读。新仓库保留脚本/补丁来源记录，不搬运历史二进制；构建从固定上游提交开始，按明确顺序应用 XHTTP、原生 IP 选择及 strict 补丁。审计作为显式选项。

**Tech Stack:** Go 1.26、Python 3、Git、GitHub Actions，Linux amd64/arm64。

**Spec:** ../specs/2026-11-12-open-source-design.md（已批准，面板为 Luma）

## Global Constraints
- 固定上游 0a29338e1f102a462363ce3527417029f89bab28。
- 当前基线 c8058a2f14060d8d2d12adaaef218ff558a1df98；双 IP 来源 2a5ae50eed9bb400995601293a4ebd484920b0fd。
- 不修改 php-fpm-core，不连接生产节点；审计默认关闭。
- 保留 caddy/caddyctl、现有配置路径与服务兼容性。

## Review Focus
- 补丁因源码漂移失败必须终止，不输出半成品。
- 旧 Machine 配置/凭据不得被隐式替换。
- 新仓库安装器不得继续取旧仓库的默认安装包。
- 所有发布/构建来源可追溯，不混用审计与非审计功能。
- ARM64 编译与 128 MiB 运行验证分别报告。

## Task 1: 来源与确定性源码准备
**Files:** source.lock.json、scripts/prepare_source.py、scripts/apply_ipfamily.py、patches/strict_ipfamily.py、tests/test_prepare_source.py、docs/provenance.md。
**Interfaces:** prepare_source.prepare(source: Path, audit: bool=False) -> None；输入必须是固定 SHA 的干净上游 checkout，调用后能运行 Go 内核回归。
- [ ] 从历史工作流提取 IP 原生变换到独立脚本，保留原内容与来源。
- [ ] 写集成测试：复制真实固定源码，执行 prepare，验证 Go 相关单测；不同 SHA、二次执行应拒绝。
- [ ] 运行测试观察尚无 prepare 入口时失败。
- [ ] 实现 prepare，校验 git SHA/工作区状态；按 XHTTP -> 原生 IP -> strict -> CLI -> 可选 audit 顺序处理。
- [ ] 在真实固定源码副本上准备成功，二次执行拒绝，运行内核回归。

## Task 2: 可重复构建、安装与发布边界
**Files:** scripts/build.py、.github/workflows/ci.yml、.github/workflows/build.yml、install-v2.sh、README.md、LICENSE、THIRD_PARTY_NOTICES.md、docs/compatibility-matrix.md。
**Interfaces:** python scripts/build.py --source PATH --arch amd64|arm64 --out PATH；输出 caddy/caddyctl、tar.gz、SHA256SUMS、build-info.json。
- [ ] 验证现有安装脚本测试（失败时保留真实平台限制）。
- [ ] 实现固定版本号和源码时间、规范化归档、SHA256 清单。
- [ ] 安装默认新仓库固定预览 tag，并在替换二进制前校验包 SHA256；保留原有版本与 Machine 校验。
- [ ] 将历史自动发布工作流移到 docs/legacy，避免新仓库推送触发旧资产发布。
- [ ] 构建 Linux amd64/arm64；amd64 可运行环境才执行 smoke，无法执行时明确未验证。
- [ ] 提交独立 feat/bootstrap-node 分支；原仓库保持不变。

## Task 3: 公开仓库交付
**Files:** README.md、docs/audit.md、docs/compatibility-matrix.md。
- [ ] 核对来源声明与公开文件，无实际凭据与运行数据。
- [ ] 创建/查重 chinahch/caddy Public；推送经检查分支或初始 main，不覆盖已有仓库。
- [ ] 如发布通路要求用户登录，保留完整本地工程与具体阻塞，继续不依赖远端的工作。
- [ ] 记录仓库链接、commit、真实构建/测试结果及未验证项。
