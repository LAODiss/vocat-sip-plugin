# VoCat SIP Plugin

一个为 VoCat 提供 SIP/PBX 功能的插件，使 Zoiper、Groundwire 等 SIP 客户端能够通过蜂窝调制解调器拨打/接收电话和收发短信。

## 功能特性

- **SIP 注册/认证** - 支持 Digest 认证，兼容标准 SIP 客户端
- **语音通话** - 支持内部呼叫、外呼、来电、保持、转接
- **短信收发** - 通过 SIP MESSAGE 发送/接收短信，桥接到蜂窝网络
- **多账号管理** - 支持多个 SIP 账号，每个账号绑定不同的蜂窝设备
- **实时通话监控** - Web 界面显示实时通话状态、时长、录音
- **WebSocket 实时推送** - 通话事件、账号状态实时更新
- **TLS/SIPS 支持** - 支持加密 SIP 信令
- **NAT 穿透** - 支持 STUN/ICE，适配各种网络环境

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                        VoCat Core                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│  │  Modem   │  │  SMS     │  │  Voice   │  │  Device  │     │
│  │ Manager  │◄─┤  Store   │◄─┤  Call    │◄─┤ Manager  │     │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘     │
└─────────────────────────────────────────────────────────────┘
                              ▲
                    Plugin API (HTTP + WebSocket)
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    vocat-sip Plugin                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  SIP Server  │  │  HTTP API    │  │  WebSocket   │       │
│  │  (UDP/TCP)   │◄─┤  (REST)      │◄─┤  (Events)    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│         │                  │                  │              │
│         ▼                  ▼                  ▼              │
│  ┌──────────────────────────────────────────────┐           │
│  │           Account / Call Manager             │           │
│  └──────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
                              ▲
                    SIP Clients (Zoiper, Groundwire, etc.)
```

## 安装

### 前置要求

1. VoCat 已安装并运行
2. 启用开发者模式：
   ```bash
   vocat develop enable
   systemctl restart vocat
   ```

### 方式一：从预构建包安装（推荐）

1. 下载最新的 `vocat-sip-plugin.zip` 从 [Releases](https://github.com/your-repo/vocat-sip-plugin/releases)
2. 在 VoCat Web UI 中进入 `扩展` 页面
3. 点击 `上传插件` 选择 zip 文件
4. 启用插件

### 方式二：从源码构建

```bash
# 克隆仓库
git clone https://github.com/your-repo/vocat-sip-plugin
cd vocat-sip-plugin

# 构建
./build.sh

# 生成的包在 dist/vocat-sip-plugin.zip
```

## 配置

### SIP 服务器设置

在 VoCat Web UI 中进入 `SIP 设置` 页面配置：

| 设置项 | 默认值 | 说明 |
|--------|--------|------|
| 监听地址 | 0.0.0.0 | SIP 服务器绑定的 IP 地址 |
| SIP 端口 | 5060 | SIP 信令端口 (UDP/TCP) |
| RTP 端口范围 | 10000-10200 | 媒体流端口范围 (必须为偶数) |
| SIP 域名 | vocat.local | SIP URI 域名部分 |
| 认证 Realm | VoCat SIP | Digest 认证 realm |
| 启用 TLS | 关闭 | 启用 SIPS 加密传输 |

### 创建 SIP 账号

1. 进入 `SIP 账号` 页面
2. 点击 `添加账号`
3. 填写信息：
   - **SIP 用户名**: 如 `1001`、`user001`
   - **认证用户名**: 可选，默认同用户名
   - **密码**: SIP 认证密码
   - **显示名称**: 来电显示的名称
   - **关联设备**: 选择用于通话的蜂窝调制解调器
   - **启用账号**: 开启后即可注册

### 客户端配置 (Zoiper / Groundwire)

#### 基本设置
```
服务器地址: YOUR_VOCAT_SERVER_IP
SIP 端口: 5060 (默认)
传输协议: UDP (或 TLS)
用户名: 1001
密码: ********
域名: vocat.local
```

#### 高级设置 (Zoiper)
```
账号类型: SIP UDP
认证用户名: 1001 (可选)
Realm: VoCat SIP
端口: 5060
传输: UDP
启用 rport: 是
启用 STUN: 是
STUN 服务器: stun.l.google.com:19302
保持连接间隔: 30 秒
编解码器: opus, G.722, PCMU, PCMA
```

#### 高级设置
```
账号类型: SIP
用户名: 1001
密码: ********
域/服务器: YOUR_VOCAT_SERVER_IP
端口: 5060
传输: UDP
启用 rport: 开启
STUN 服务器: stun.l.google.com:19302
保持在线: 30 秒
编解码器优先级: opus > G.722 > PCMU > PCMA
```

## 使用指南

### 拨打电话

1. 在 SIP 客户端拨号键盘输入号码
2. 点击拨打
3. 通话通过关联的蜂窝设备路由到蜂窝网络

### 接听电话

1. 来电时 SIP 客户端会响铃
2. 点击接听
3. 语音通过 RTP 流传输

### 发送短信

1. 在 SIP 客户端发送 SIP MESSAGE
2. 目标 URI: `sip:+86138xxxxxxxx@vocat.local`
3. 短信通过关联设备发送到蜂窝网络

### 查看通话记录

在 `通话记录` 页面可以查看：
- 所有通话历史（来电/去电/未接）
- 通话时长、状态、录音
- 按方向、状态筛选
- 搜索号码

## 开发

### 项目结构

```
vocat-sip-plugin/
├── vocat-plugin.json        # 插件清单
├── backend/                 # Go 后端
│   ├── main.go             # 入口点
│   ├── go.mod              # 依赖
│   └── *.go                # 其他模块
├── frontend/               # React 前端
│   ├── src/
│   │   ├── pages/          # 页面组件
│   │   ├── hooks.ts        # React hooks
│   │   ├── api.ts          # API 客户端
│   │   └── styles.css      # 样式
│   ├── package.json
│   └── vite.config.ts
├── assets/                 # 构建产物 (由构建生成)
└── build.sh               # 构建脚本
```

### 本地开发

```bash
# 后端开发
cd backend
go run main.go -config /path/to/config

# 前端开发
cd frontend
npm run dev
# 访问 http://localhost:3000
# 配置 vite.config.ts 中的 proxy 指向后端
```

### 扩展开发

插件使用 VoCat 的扩展系统：
- 后端作为独立进程运行，通过 `VOCAT_PLUGIN_LISTEN` 环境变量获取监听地址
- 前端资源通过 `/plugin-assets/vocat-sip/...` 提供
- 通过 WebSocket 推送实时事件到前端

## 故障排查

### SIP 客户端无法注册
1. 检查防火墙：UDP 5060 是否开放
2. 检查用户名/密码是否正确
3. 检查域名是否匹配服务器配置
4. 查看后端日志：`journalctl -u vocat -f | grep sip`

### 单向音频 / 无音频
1. 检查 RTP 端口范围是否在防火墙开放
2. 启用 STUN/ICE
3. 如果在 NAT 后，配置端口转发
4. 检查编解码器协商

### 短信发送失败
1. 确认关联设备支持短信功能
2. 检查设备 SIM 卡状态
3. 查看 VoCat 短信日志

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 相关链接

- [VoCat 官方仓库](https://github.com/MengMengCode/VoCat)
- [SIP 协议规范 RFC 3261](https://tools.ietf.org/html/rfc3261)
- [Zoiper 官网](https://www.zoiper.com/)
- [Groundwire 官网](https://www.acrobits.net/groundwire/)