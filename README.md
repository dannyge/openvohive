# OpenVoHive

> **Fork 声明**：本仓库 fork 自 [iniwex5/vohive](https://github.com/iniwex5/vohive)，并进行深度改造和维护。
>
> 原项目部分功能（VoWiFi 等）依赖未开源的闭源库，导致项目无法独立编译运行。本 fork 移除了这些闭源依赖及关联子系统，使项目可正常编译、构建和部署。
>
> 相较原项目的主要变更：
> - **修复**：移除闭源依赖，恢复项目可编译可运行状态
> - **移除**：VoWiFi 子系统、HTTP/SOCKS5 代理引擎、上游代理相关功能、冗余通知渠道
> - **清理**：重构项目结构、清理残留桩代码、移除 CI 工作流、前端的 "我, 许可..." 弹窗和自毁 
> - **精简**：聚焦模组管理与短信核心能力
>
> 我提倡各位拉代码到自己本地可控的编译运行, 而不是使用我提供 (你真的敢相信我发的二进制就一定是由这份源码编译出来的嘛?!) 或者无法追踪的二进制, 毕竟这是一个比较隐私和敏感的项目
>
> 不过当前项目仅仅能跑而已, 很多地方都不够优雅, 而且我不喜欢写单测, 后续挑个时间根据原理重构一个全新的项目完全脱离原项目...不过短期我没有太多精力为这个项目...
>
> 提交包含大量变更是因为做了 commit 合并, 之前是想挂这个仓库在自己账号下面的, 但是后面看见原作者在他仓库下面的 issues 回复, 害怕被他报复所以才选择现在这种方式...我不敢用真实信息来提交代码...
>
> 玩得开心!

---

[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)
[![Vue 3](https://img.shields.io/badge/Vue-3-42b883?logo=vue.js)](web/package.json)
[![License](https://img.shields.io/badge/License-PolyForm--Noncommercial--1.0.0-blue.svg)](LICENSE)

面向高通 4G/LTE/5G 模组（Quectel EC20/EC25/EC21/EG25/EM20 等）的综合管理与代理服务平台。

## 核心特性

| 模块 | 说明 |
| --- | --- |
| 多模组并发管理 | USB 热插拔自动发现（ttyUSB 等），多设备实时状态监控 |
| 通信与短信中心 | 统一界面/API 处理 AT 短信收发、会话与联系人管理、USSD 交互，短信落库可查 |
| eSIM 管理 | 通过 AT 指令通道直接管理 eSIM 芯片，支持 Profile 下载、启用/停用、重命名、删除 |
| 全渠道通知 | 重要短信及系统告警可推送至 Telegram、Email、Webhook 等 (砍了很多没必要的, 比如 QQ 和飞书. 想要使用飞书直接用 Webhook POST 就行) |
| SIP 语音网关 | 内建 SIP Registrar，兼容 Linphone 等软电话，支持 CS Call 通话管理 (这个我的大疆模块没有音频没法测试) |
| 多架构构建 | 原生支持 amd64/arm64/arm7 跨平台编译，路由器到边缘节点均可部署 |

## 典型应用场景

- **统一接码/验证码中心**：Web 界面或 API 并行收发多卡短信，并通过 Webhook/Bot 实时推送到个人终端。

## 架构与技术栈

- **Backend**：Go 1.26+（Gin、GORM、Viper、sipgo、euicc-go）
- **Frontend**：Bun + Vue 3 + Vite + TailwindCSS + Element Plus
- **Database**：SQLite（`vohive.db`）

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/openvohive/openvohive.git
cd vohive

# 配置
cp config/config.example.yaml config/config.yaml
# 编辑 config/config.yaml 修改端口和账号密码

# 构建前端（需要 bun）
cd web && bun install && bun run build && cd ..

# 构建后端
go generate ./...
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath .

# 运行
./vohive -c config/config.yaml
```

也可使用 Docker 构建：

```bash
bash Dockerbuild
```

> `Dockerdeploy` 为维护者自用部署脚本，可忽略。

## 免责声明

- **用途定位**：本项目主要面向个人学习、技术研究与功能测试场景，不建议直接用于生产环境或关键业务系统；由此产生的部署及使用风险由使用者自行承担。
- **非官方项目**：VoHive 为第三方独立开发的开源软件，与 Quectel（高通模组厂商）、高通公司及其他任何模组/芯片厂商均无官方关联、授权或合作关系，亦不对模组硬件本身的功能、质量或安全性负责。
- **合规使用**：使用本项目搭建的服务时，请自行确保符合所在地区的法律法规及电信运营商的服务条款，不得用于任何违法违规用途。因违规使用造成的一切法律责任由使用者自行承担，与本项目作者及贡献者无关。
- **无担保**：本软件按"现状"提供，不附带任何明示或暗示的担保，包括但不限于适销性、特定用途适用性及不侵权担保。因使用或无法使用本软件（含数据丢失、设备异常、业务中断等）造成的任何直接或间接损失，作者及贡献者不承担任何责任。

## License

本项目基于 [PolyForm Noncommercial License 1.0.0](LICENSE) 开源，**仅限非商业用途**：

- 可自由查看、使用、修改、分发源码用于个人学习、研究、测试等非商业场景
- **禁止任何形式的商业使用**（包括但不限于销售、提供付费服务、用于盈利性产品或业务）
- 如需商业授权，请联系原作者另行协商

本项目 fork 自 [iniwex5/vohive](https://github.com/iniwex5/vohive)，原项目版权归 iniwex5 所有。

> Required Notice: Copyright iniwex5 (https://github.com/iniwex5/vohive)