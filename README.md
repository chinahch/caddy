# caddy · XBoard-compatible node

本项目是自有 XBoard-Node 定制工程的可追溯迁移版，**不是 Caddy Web Server**。程序与服务保留 `caddy` / `caddyctl` 名称，以兼容历史安装路径。

配套面板：[Luma](https://github.com/chinahch/luma)。源码发布为预览版，不等于已在生产或 128 MiB ARM64 节点验收。

## 来源与功能

- 原工程：[chinahch/php-fpm-core](https://github.com/chinahch/php-fpm-core)，固定 `c8058a2f14060d8d2d12adaaef218ff558a1df98`。
- 双 IP 补丁来源：原工程历史提交 `2a5ae50eed9bb400995601293a4ebd484920b0fd`，不能只复制当前 main 重建。
- 上游：[cedar2025/Xboard-Node](https://github.com/cedar2025/Xboard-Node)，固定 `0a29338e1f102a462363ce3527417029f89bab28`。
- Node / Machine、多逻辑节点、sing-box / Xray、XHTTP 与内核原生 IP 选择修复。
- Luma `landing-v1` / `protocols-v1` 配置契约，完整 Xray 出站传输参数、sing-box detour、显式内核选择，以及 AnyTLS REALITY 修复。
- [独立连接器](connector/README.md)：受控 cloudflared 与每用户 mtg，配置租约、私密文件、落盘批次与去重上报；原生节点仍使用单独标记的旧上报方式。
- 可选域名审计默认不编入；使用 `--audit` 才编入，并仍需显式环境变量才开启。它独立于计费统计，不保证记录完整。

见 [来源说明](docs/provenance.md)、[兼容矩阵](docs/compatibility-matrix.md) 和 [审计限制](DOMAIN-AUDIT-README.md)。

## 节点服务器安装（无需 Docker）

面板“机器管理 → 安装 / 接入”生成固定版本安装命令。节点以 root 执行，按提示粘贴机器令牌；安装器下载原生 Linux amd64/arm64 二进制及 SHA256 清单，校验后注册 systemd/OpenRC 服务。节点无需 Go、Python 或 Docker。面板地址必须能从节点访问，不能填写 localhost。

预编译安装包见 [Releases](https://github.com/chinahch/caddy/releases)。版本 `v0.1.1-preview` 同时提供精确依赖清单、对应模块与 Go 源码、许可证/通知及构建来源。CF/MTProxy 连接器仍独立安装。

重置机器令牌后选择“已有 Caddy：只更新本机器令牌并重启服务”。安装器校验并临时运行对应版本的 caddyctl，只更新面板地址/机器 ID 精确匹配的私有凭据，不改写 config.yml、不替换运行中的节点二进制；其他绑定保留。面板域名发生变化时应按迁移流程核对旧绑定，不能作为同一个地址自动更新。

## 从固定源码构建

需要 Git、Python 3.12+、Go 1.26.10。推荐在 Linux 构建并执行测试。

```sh
git clone https://github.com/chinahch/caddy.git
cd caddy
git clone https://github.com/cedar2025/Xboard-Node.git .work/upstream
git -C .work/upstream checkout --detach 0a29338e1f102a462363ce3527417029f89bab28
python3 scripts/prepare_source.py --upstream .work/upstream --out .work/prepared
cd .work/prepared
go test ./internal/...
cd ../..
python3 scripts/build.py --source .work/prepared --arch amd64 --out dist
python3 scripts/build.py --source .work/prepared --arch arm64 --out dist
cat dist/SHA256SUMS-* > dist/SHA256SUMS
```

准备流程从 Git 提交导出，既不修改上游 checkout，也不复制其未提交文件；输出目录已存在时拒绝覆盖。构建输出包含二进制、规范化安装包、SHA256 与来源清单。相同源码/补丁、Go 工具链和依赖才具有可重建意义，不保证任意 Go 版本产生相同字节。

## 安装与回滚

先在隔离测试机器验证协议、配置、资源占用和断线行为。可使用 Release 中的原生安装包，也可将自行构建包放在受控 HTTPS 地址，并显式提供其校验值。

```sh
# 使用上一步在本机生成的包；ARM64 改用相应文件名。Alpine 先安装 bash。
export CADDY_PACKAGE_URL="file://$PWD/dist/caddy-linux-amd64v1.tar.gz"
export CADDY_PACKAGE_SHA256="$(sha256sum dist/caddy-linux-amd64v1.tar.gz | cut -d' ' -f1)"
bash install-v2.sh --mode node --panel https://panel.example.com \
  --token YOUR_TEST_NODE_TOKEN --node-id 1 --node-type vless --kernel xray
```

Machine 模式改为 `--mode machine --machine-id 1`，使用面板生成的 Machine 令牌。重复安装保留现有 Machine 配置与凭据；旧版本自动升级目前禁用，因为上游进程 `/healthz` 不能证明各逻辑节点的内核已就绪。请在隔离机器验证后安排手工升级与回滚。首次安装先校验 SHA256、内容和版本，再安装二进制。未提供独立 SHA256 时从同路径 `SHA256SUMS` 读取；它提供传输完整性校验，不等同于独立签名或来源真实性证明。

服务路径：`/etc/caddy/config.yml`、`/etc/caddy/credentials.env`、`/usr/local/bin/caddy`、`/usr/local/bin/caddyctl`；systemd `caddy.service` 或 OpenRC `caddy`。避免与真正的 Caddy Web Server 共用这些路径。

升级前自行备份程序、配置、凭据和服务文件。失败时使用已保存的旧程序与配置恢复，并验证实际进程、连接与流量，而不只检查版本输出。不要在正在承载用户的节点上试验低内存候选包。

## 测试与已知限制

```sh
export CADDY_TEST_UPSTREAM="$PWD/.work/upstream"
python3 -m unittest discover -s tests -p 'test_*.py'
bash tests/test-checksum.sh
bash tests/test-installer-guards.sh
bash tests/test-install-v2.sh
```

本机验证：Linux amd64/arm64 交叉编译通过；配置、tracker、双内核、控制面测试通过。审查修复后，CLI YAML round-trip、安装保护/校验/配置保留回归通过（Windows Git Bash）；Linux systemd/OpenRC 与 128 MiB ARM64 运行尚未验证。以 CI 和后续实际运行结果为准，不能把交叉编译通过视为兼容验收。

新增本机验证：23 个 Xray 原生变体和 6 个 sing-box 组合均完成真实本地代理回显往返，包括六种 ENC、AnyTLS REALITY、SS2022、Hysteria2 和 XHTTP/H3。连接器测试覆盖真实进程启动/租约停止，固定 mtg 接受生成配置并提供本机指标。此结果不代替 CF 账户、Telegram 外网客户端或生产网络验收。完整功能和部署条件见 [Luma 协议文档](https://github.com/chinahch/luma/blob/main/docs/protocols-exits.md)。

旧上报使用窗口增量，失败量只恢复到内存，没有可靠持久批次身份。因此 Luma 的旧协议适配明确标记 `legacy`，不保证重试去重与断线不丢量。新增可靠批次 API 不表示当前节点已自动切换到它。

`docs/legacy/` 保留原始安装脚本与工作流，仅供追溯，不是新项目的执行入口，也不会自动发布旧仓库资产。

## 许可

节点定制代码按 MPL-2.0 提供，保留上游项目归属；第三方 Go 依赖遵循各自许可。见 [LICENSE](LICENSE) 与 [第三方说明](THIRD_PARTY_NOTICES.md)。
