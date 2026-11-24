# 使用指导

本手册说明如何在 macOS、Linux、Windows环境中使用夜莺 Router。
本文给出指导示例，模型版本、客户端界面等可能随版本变化，具体以实际情况为准。

## 使用前

- API Base URL：`https://router.yeying.pub`。
- API Key 为您在夜莺 Router 获取的令牌(YOUR_YEYING_ROUTER_API_KEY)，在配置时请注意替换。
- 请求中的模型名必须是令牌可访问的模型，目前各个厂家会不定期进行迭代，配置时请注意。
- Router 按协议和端点转发，不会把不支持的端点自动转换成另一种协议。
- OpenAI 兼容客户端使用 `https://router.yeying.pub/v1`。
- Claude Code 的`ANTHROPIC_BASE_URL` 填 `https://router.yeying.pub`，客户端会请求 `/v1/messages`。


## CLI配置

### Codex CLI

#### 安装

从 [Codex CLI 官方文档](https://developers.openai.com/codex/cli) 查看当前安装方式。
Codex CLI 可在 macOS、Linux 和 Windows 使用；npm 安装需要先安装 Node.js LTS。
Windows环境如果原生终端环境不兼容，可使用 WSL。

```bash
npm install -g @openai/codex
```

安装后运行 `codex --version` 检查命令是否可用。

#### 配置

在 macOS/Linux 的 `~/.codex/config.toml`，或 Windows 的 `%USERPROFILE%\.codex\config.toml` 中写入：

```toml
model = "gpt-6-sol"
model_provider = "yeying"

[model_providers.yeying]
name = "Yeying Router"
base_url = "https://router.yeying.pub/v1"
wire_api = "responses"
experimental_bearer_token = "YOUR_YEYING_ROUTER_API_KEY"
```

若配置文件已有其他内容，只合并这些字段，并避免重复定义同一个配置段。保存后重新打开终端。

#### 验证

在项目目录运行 `codex`，新建会话并发送 `请只回复 OK`。收到模型回复即表示连通。若客户端提示模型不存在，请先确认该模型在令牌可用范围内，且支持 Responses 端点。

### Claude Code

#### 安装

查看 [Claude Code 官方安装文档](https://docs.anthropic.com/en/docs/claude-code/setup)。
macOS/Linux 可使用官方安装脚本。
Windows 可使用官方 PowerShell 安装程序，也可按官方文档使用 npm 安装，npm 方式需要 Node.js LTS。

```bash
# macOS / Linux
curl -fsSL https://claude.ai/install.sh | bash
```

```powershell
# Windows PowerShell
irm https://claude.ai/install.ps1 | iex
```

安装后运行 `claude --version` 检查命令是否可用。

#### 配置

在用户配置文件 `~/.claude/settings.json`（Windows 对应 `%USERPROFILE%\.claude\settings.json`）中加入：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://router.yeying.pub",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_YEYING_ROUTER_API_KEY"
  }
}
```

保留已有设置并合并 `env` 字段。Anthropic Base URL 不要附加 `/v1`。

#### 验证

打开新终端，在项目目录运行 `claude`，发送 `reply exactly OK`。收到 `OK` 表示调用成功。


## APP配置

### ChatGPT

#### 安装

从 [ChatGPT 下载页面](https://chatgpt.com/download/) 获取客户端安装文件。


#### 配置

ChatGPT 官方客户端和网页端使用 OpenAI 自有服务，不提供自定义 API Base URL 或第三方 Router API Key 配置。
经过笔者在Ubuntu 24.04.5 LTS环境中实际验证，只要按照前述配置Codex CLI之后，安装的ChatGPT客户端同样可以使用。

#### 验证

登录 ChatGPT 客户端并发送一条消息，可验证服务是否可用。



### Codex App

#### 安装

从 [OpenAI Codex 页面](https://openai.com/codex) 获取 macOS 或 Windows 客户端；Windows 也可从 Microsoft Store 安装。
笔者在Ubuntu 24.04.5 LTS环境中打开上述页面，下载的为ChatGPT客户端。

#### 配置

首次启动时选择“使用其他方式登录”并输入夜莺 Router API Key。登录页的 Key 不会写入 `config.toml`，还需在 macOS/Linux 的 `~/.codex/config.toml` 或 Windows 的 `%USERPROFILE%\.codex\config.toml` 配置：

```toml
model = "gpt-6-sol"
model_provider = "yeying"

[model_providers.yeying]
name = "Yeying Router"
base_url = "https://router.yeying.pub/v1"
wire_api = "responses"
experimental_bearer_token = "YOUR_YEYING_ROUTER_API_KEY"
```

也可在 Codex App Settings 中搜索 `config.toml` 并打开配置文件。保存后完全退出并重新打开应用。

#### 验证

新建会话，发送 `请只回复 OK`。如果成功回复，说明 Codex App 已连通 Router。遇到 401 时先检查配置文件中的令牌，再完全重启应用。

### Cursor

#### 安装

从 [Cursor 下载页](https://cursor.com/) 安装：

- macOS：下载 Apple Silicon、Intel 或 Universal `.dmg`，拖入 Applications。
- Windows：下载 x64 或 ARM64 安装包并运行安装程序。
- Linux：下载官方 Linux 版本 笔者在Ubuntu 24.04.5 LTS环境中打开上述页面，下载的为cursor_3.30.14_amd64.deb文件。

#### 配置

打开 Cursor Settings → Models → API Keys，在 OpenAI API Key 中填写 Router API Key，并开启 `Override OpenAI Base URL`，地址填写 `https://router.yeying.pub/v1`。启用或添加一个已授权且可用的模型，例如 `gpt-6-sol`，并保存设置。

#### 验证

在新会话中发送 `reply exactly OK`，收到模型回复表示接入成功。笔者在Ubuntu 24.04.5 LTS环境安装后使用时显示使用的模型为Grok 4.7, 选择模型会提示“Upgrade to Pro”，笔者并没有进行升级。


### Cherry Studio

#### 安装

从 [Cherry Studio 官网](https://cherry-ai.com) 下载 macOS、Windows 或 Linux 对应安装包并完成安装。

#### 配置

进入模型服务设置，添加 Provider, Provider Name可以自定义，填写Router API Key， 配置OpenAI Base URL `https://router.yeying.pub/v1`，然后选择可用模型名。保存后将该模型设为当前对话模型。

#### 验证

新建对话并发送 `reply exactly OK`。收到回复表示接入成功。


## VS Code插件

先从 [Visual Studio Code 下载页](https://code.visualstudio.com/download) 安装适用于 macOS、Linux 或 Windows 的 VS Code，再在扩展市场安装对应插件。
macOS快捷键为 `Cmd+Shift+X`，Windows/Linux快捷键为 `Ctrl+Shift+X`。

### Cline

#### 安装

在 VS Code 扩展市场搜索并安装 [Cline](https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev)，然后重新加载 VS Code。该插件的配置方式在 macOS、Linux、Windows 一致。

#### 配置

打开 Cline 面板，选择 Bring my own API key，并填写：

| 配置项 | 内容 |
| --- | --- |
| API Provider | `OpenAI Compatible` |
| Base URL | `https://router.yeying.pub/v1` |
| API Key | 夜莺 Router API Key |
| Model ID | 令牌可用的模型，如 `gpt-6-sol` |

保存配置并返回 Cline 对话。

#### 验证

发送 `reply exactly OK`。收到 `OK` 表示调用成功。若出现 403 或额度不足，检查令牌额度、套餐和模型权限。

### Continue

#### 安装

在 VS Code 扩展市场搜索并安装 [Continue](https://marketplace.visualstudio.com/items?itemName=Continue.continue)，然后重新加载 VS Code。插件在 macOS、Linux、Windows 上使用相同配置。

#### 配置

在用户主目录的 `.continue/config.yaml`（Windows 为 `%USERPROFILE%\.continue\config.yaml`）创建或更新配置，将 API Key 替换为自己的 Router API Key：

```yaml
name: Yeying Router
version: 1.0.0
schema: v1

models:
  - name: Yeying gpt-6-sol
    provider: openai
    model: gpt-6-sol
    apiBase: https://router.yeying.pub/v1
    apiKey: YOUR_YEYING_ROUTER_API_KEY
    roles:
      - chat
      - edit
      - apply
    capabilities:
      - tool_use
```

保存文件并重启 VS Code。

#### 验证

打开 Continue 面板并发送 `reply exactly OK`。收到 `OK` 表示接入成功。



## 常见问题

### Codex CLI / Codex App：401 Unauthorized

- 检查 `config.toml` 中 `experimental_bearer_token` 是否为 Router API Key，且没有多余空格。
- 检查 Base URL 填写是否正确，是否以 `/v1` 结尾、`wire_api` 是否为 `responses`。
- 保存后完全退出并重新打开客户端，再新建会话测试。
- 不要额外添加 `requires_openai_auth = true`。

### Claude Code：认证失败或请求发往官方服务

- 检查 `ANTHROPIC_AUTH_TOKEN` 和 `ANTHROPIC_BASE_URL` 是否在 Claude Code 启动环境中生效。
- Anthropic Base URL 使用 `https://router.yeying.pub`，不要附加 `/v1`。
- 如果环境变量保存在 shell 配置中，重开终端后再运行 `claude`。
- 确认令牌和模型支持 `/v1/messages`。

### Gemini CLI：不能经 Router 调用

当前 Router 未开放 Google Gemini 原生 API 路径。`GOOGLE_GEMINI_BASE_URL`、`/v1beta` 都不能作为 Router 的可用接入方式。请用 Google 官方服务验证 Gemini CLI，或改用本手册中支持的 OpenAI/Anthropic 协议客户端验证 Router。

### Cursor / Cline / Continue / Chatbox / Cherry Studio：401 或模型不存在

- OpenAI 兼容 Base URL 应为 `https://router.yeying.pub/v1`。
- 确认 API Key 是 Router 令牌，而不是上游供应商 Key。
- 确认模型 ID 与令牌权限一致，并且该模型支持客户端所调用的端点。
- 保存配置并重启客户端或 VS Code。

### 403 Forbidden、额度不足或模型无权限

请求通常已到达 Router，但令牌的额度、套餐或模型权限不足。检查令牌状态和可用模型范围，必要时联系管理员。



### 人工协助

记录客户端名称和版本、操作系统、完整错误信息、脱敏后的相关配置，以及请求使用的模型和端点，再联系管理员排查。切记不要发送完整 API Key。
