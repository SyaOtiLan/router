# 夜莺 Router 接入 Cline

**测试环境**

| 项目 | 内容 |
| --- | --- |
| 操作系统 | Windows 10 Home China 25H2 |
| 系统版本 | Build 26200.9457，64 位 |
| 编辑器 | VS Code Windows 版 |
| 插件 | Cline |
| 扩展 ID | `saoudrizwan.claude-dev` |
| 测试版本 | `4.1.19` |

### 1. 安装 Cline

打开 VS Code 扩展市场，搜索并安装：

```text
Cline
```

也可以打开扩展页面：

```text
https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev
```

### 2. 选择自带 API Key

打开 Cline 面板，选择：

```text
Bring my own API key
```

然后点击：

```text
Continue
```

![选择 Bring my own API key](assets/cline-yeying-router/01-bring-own-api-key.png)

### 3. 填写配置

| 配置项 | 填写内容 |
| --- | --- |
| API Provider | `OpenAI Compatible` |
| Base URL | `https://router.yeying.pub/v1` |
| OpenAI Compatible API Key | 夜莺 Router API Key |
| Model ID | `gpt-5.6-sol` |

![填写 OpenAI Compatible 配置](assets/cline-yeying-router/02-openai-compatible-config.png)

填写完成后点击 `Continue`。

### 4. 验证

在 Cline 对话里发送：

```text
reply exactly OK
```

如果返回 `OK`，说明 Cline 已经通过夜莺 Router 调用模型。

如果返回 `403 Forbidden` 或“令牌额度不足”，通常表示请求已经到达夜莺 Router，但当前 API Key 的额度、套餐或模型权限不足，请联系管理员检查该 Key 的额度和模型权限。

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

