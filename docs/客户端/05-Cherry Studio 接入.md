# 夜莺 Router 接入 Cherry Studio

**测试环境**

| 项目 | 内容 |
| --- | --- |
| 操作系统 | Windows 10 Home China 25H2 |
| 系统版本 | Build 26200.9457，64 位 |
| 客户端 | Cherry Studio Windows 客户端 |

Cherry Studio 是桌面模型客户端，适合用于日常对话、多模型切换和模型可用性验证。

### 1. 下载 Cherry Studio

打开 Cherry Studio 官网下载并安装：

```text
https://cherry-ai.com
```

### 2. 打开模型服务设置

打开 Cherry Studio 后，进入模型服务或模型提供方设置页面。

如果列表中有 OpenRouter 或 OpenAI Compatible 类型，可以直接使用对应类型；如果没有，可以添加一个 OpenAI 兼容的自定义服务。

### 3. 填写夜莺 Router 配置

| 配置项 | 填写内容 |
| --- | --- |
| API 地址 / Base URL | `https://router.yeying.pub/v1` |
| API Key | 夜莺 Router API Key |
| 模型 | `gpt-5.6-sol` |

保存配置后，将该模型设为当前聊天模型。

### 4. 验证

在 Cherry Studio 对话里发送：

```text
reply exactly OK
```

如果返回 `OK`，说明 Cherry Studio 已经通过夜莺 Router 调用模型。

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

