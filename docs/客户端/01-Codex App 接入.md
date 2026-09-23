# 夜莺 Router 接入 Codex App

**测试环境**

| 项目 | 内容 |
| --- | --- |
| 操作系统 | Windows 10 Home China 25H2 |
| 系统版本 | Build 26200.9457，64 位 |
| 客户端 | Codex App Windows 客户端 |
| Codex CLI | `0.155.0-alpha.9.2` |

### 1. 下载 Codex App

请前往 OpenAI 官方 Codex 页面下载：

```text
https://openai.com/codex
```

Windows 用户也可以通过 Microsoft Store 安装。

### 2. 打开 config.toml

推荐从 Codex App 设置里打开配置文件，避免找错目录。

打开 Codex App 后，点击左下角账号或工作区入口。

![点击左下角账号入口](assets/codex-yeying-router/01-open-account-menu.png)

在弹出的菜单中点击 `Settings`。

![点击 Settings](assets/codex-yeying-router/02-open-settings.png)

在设置页左侧搜索框输入：

```text
config.toml
```

![搜索 config.toml](assets/codex-yeying-router/03-search-config-toml.png)

点击左侧搜索结果中的 `config.toml`，再点击右侧的 `Open config.toml`。

![点击 Open config.toml](assets/codex-yeying-router/04-open-config-toml.png)

如果需要手动查找，也可以打开：

```text
C:\Users\你的用户名\.codex
```

在这个文件夹中找到或新建：

```text
config.toml
```

注意文件名必须是 `config.toml`，不要保存成 `config.toml.txt`。

### 3. 写入配置

打开 `config.toml` 后，将下面内容复制进去，并把 `YOUR_YEYING_ROUTER_API_KEY` 替换为自己的夜莺 Router API Key。

```toml
model = "gpt-5.6-sol"
model_provider = "yeying"

[model_providers.yeying]
name = "Yeying Router"
base_url = "https://router.yeying.pub/v1"
wire_api = "responses"
experimental_bearer_token = "YOUR_YEYING_ROUTER_API_KEY"
```

保存后，完全退出 Codex App，再重新打开。

### 4. 验证

新建一个 Codex 会话，发送：

```text
请只回复 OK
```

如果模型正常回复，说明 Codex App 已经通过夜莺 Router 调用模型。

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

