# 夜莺 Router 接入 Chatbox

**测试环境**

| 项目 | 内容 |
| --- | --- |
| 操作系统 | Windows 10 Home China 25H2 |
| 系统版本 | Build 26200.9457，64 位 |
| 客户端 | Chatbox Windows 客户端 |

Chatbox 是普通聊天客户端，适合用于日常对话和模型可用性验证。

### 1. 下载 Chatbox

打开 Chatbox 官网下载并安装：

```text
https://chatboxai.app
```

### 2. 打开模型提供方设置

打开 Chatbox 后，进入：

```text
设置 → 模型提供方 → OpenAI
```

### 3. 填写夜莺 Router 配置

在 OpenAI 配置页中填写：

| 配置项 | 填写内容 |
| --- | --- |
| API 密钥 | 夜莺 Router API Key |
| API 主机 | `https://router.yeying.pub/v1` |
| 模型 | `gpt-5.6-sol` |

![Chatbox OpenAI 配置](assets/chatbox-yeying-router/01-openai-provider-config.png)

如果模型列表中没有 `gpt-5.6-sol`，可以点击 `新建` 手动添加模型。

### 4. 验证

填写完成后，点击页面中的 `检查`。

如果检查通过，说明 Chatbox 已经可以通过夜莺 Router 调用模型。

也可以回到聊天页面，选择 `gpt-5.6-sol`，发送：

```text
reply exactly OK
```

如果返回 `OK`，说明聊天调用正常。

---

## 五、常见问题

### 401 Unauthorized

如果出现 401，通常是 Codex App 没有正确带上夜莺 Router API Key。

请重点检查 `config.toml`：

```toml
model = "gpt-5.6-sol"
model_provider = "yeying"

[model_providers.yeying]
name = "Yeying Router"
base_url = "https://router.yeying.pub/v1"
wire_api = "responses"
experimental_bearer_token = "YOUR_YEYING_ROUTER_API_KEY"
```

需要确认：

- `experimental_bearer_token` 已替换为真实 API Key。
- `base_url` 是 `https://router.yeying.pub/v1`。
- 保存配置后，已经完全退出 Codex App 并重新打开。
- 不要额外添加 `requires_openai_auth = true`。

