# 夜莺 Router 接入 Continue

**测试环境**

| 项目 | 内容 |
| --- | --- |
| 操作系统 | Windows 10 Home China 25H2 |
| 系统版本 | Build 26200.9457，64 位 |
| 编辑器 | VS Code Windows 版 |
| 插件 | Continue |
| 扩展 ID | `continue.continue` |
| 测试版本 | `2.0.0` |

### 1. 安装 Continue

打开 VS Code 扩展市场，搜索并安装：

```text
Continue
```

也可以打开扩展页面：

```text
https://marketplace.visualstudio.com/items?itemName=Continue.continue
```

### 2. 打开配置目录

打开 Windows 文件资源管理器，在地址栏输入：

```text
C:\Users\你的用户名\.continue
```

如果没有 `.continue` 文件夹，可以手动创建。

### 3. 配置 config.yaml

在 `.continue` 文件夹中创建或打开：

```text
config.yaml
```

将下面内容复制进去，并把 `YOUR_YEYING_ROUTER_API_KEY` 替换为自己的夜莺 Router API Key。

```yaml
name: Yeying Router
version: 1.0.0
schema: v1

models:
  - name: Yeying gpt-5.6-sol
    provider: openai
    model: gpt-5.6-sol
    apiBase: https://router.yeying.pub/v1
    apiKey: YOUR_YEYING_ROUTER_API_KEY
    roles:
      - chat
      - edit
      - apply
    capabilities:
      - tool_use
```

保存后，重启 VS Code。

### 4. 验证

打开 Continue 面板，发送：

```text
reply exactly OK
```

如果返回 `OK`，说明 Continue 已经通过夜莺 Router 调用模型。

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

