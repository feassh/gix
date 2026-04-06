# gix

`gix` 是一个用 Go 编写的 Git CLI 助手，目标是在不破坏原生 Git 心智模型的前提下，把高频但重复的操作变得更顺手，尤其是：

- 基于 staged diff 生成 commit message
- 简化 `add / commit / push / sync / tag / branch` 流程
- 记住项目级和全局级配置
- 提供单二进制安装、补全、自更新和发布能力

## 特性概览

- 单二进制 CLI，适合直接下载分发
- 支持 macOS、Linux、Windows
- 支持 `amd64`、`arm64`
- 支持自动安装到用户 bin 目录
- 支持自动配置 shell completion
- 支持从 GitHub Releases 自更新
- 支持英文和简体中文输出
- 默认保留 Git 原生行为，尽量少引入额外概念

## 项目结构

项目按职责拆分为几层：

- `cmd/gix`
  CLI 程序入口
- `internal/app`
  命令编排、参数解析、用户交互流程
- `internal/git`
  Git 命令适配层
- `internal/ai`
  AI commit message 生成与 fallback 策略
- `internal/config`
  全局配置、项目配置和 TOML 编解码
- `internal/ui`
  终端输出、颜色、交互和格式化显示
- `internal/i18n`
  多语言文案和系统语言检测
- `internal/install`
  用户级安装、卸载、PATH 管理
- `internal/shellcompletion`
  shell completion 生成、安装和卸载
- `internal/update`
  GitHub Releases 检查、下载、校验和自更新
- `internal/version`
  semver/tag 相关逻辑
- `internal/buildinfo`
  编译时注入的版本和发布仓库信息

## 当前支持的命令

### 初始化与维护

```bash
gix init
gix install
gix uninstall
gix self-update
gix self-update --check
gix version
gix help
```

### 工作区与提交流

```bash
gix status
gix st
gix add
gix commit
gix push
gix ac
gix acp
gix sync
```

### 标签

```bash
gix tag create
gix tag list
gix tag push
gix tag delete
gix tag checkout
gix tag branch
```

### 分支

```bash
gix branch list
gix branch new
gix branch switch
gix branch delete
gix branch rename
gix branch cleanup
```

### 仓库设置

```bash
gix repo info
gix repo remote
gix repo set-default-remote
gix repo set-main-branch
```

### 配置

```bash
gix config init
gix config get
gix config set
gix config list
```

### 补全

```bash
gix completion
gix completion uninstall
gix completion print

# 兼容旧用法
gix completion bash
gix completion zsh
gix completion fish
gix completion powershell
```

## 快速开始

### 1. 初始化仓库

如果当前目录还不是 Git 仓库：

```bash
gix init
```

它会：

- 执行 `git init`
- 自动生成 `.git/gix.toml`

如果当前目录已经是 Git 仓库，但还没初始化 gix：

```bash
gix init
```

它只会执行 `gix config init` 的逻辑。

如果 Git 和 gix 都已初始化，它会提示并忽略。

### 2. 安装到用户 PATH

第一次从下载目录运行：

```bash
./gix install
```

Windows 一般是：

```powershell
.\gix.exe install
```

默认行为：

- 复制当前二进制到用户 bin 目录
- macOS / Linux 安装到 `~/.local/bin/gix`
- Windows 安装到 `%USERPROFILE%\bin\gix.exe`
- 自动尝试把用户 bin 目录加入用户级 `PATH`
- 自动安装当前 shell 的 completion

安装完成后，重新打开一个终端，通常就可以直接使用：

```bash
gix status
```

### 3. 常见提交流程

```bash
gix add
gix commit
```

或者一步完成：

```bash
gix ac
```

如果还要推送：

```bash
gix acp
```

## 命令说明

### `gix status` / `gix st`

显示当前仓库的摘要信息，包括：

- 当前仓库名
- 当前分支
- upstream
- ahead/behind
- staged / unstaged / untracked 数量
- 最新 tag
- 默认 remote

### `gix add`

支持常见 Git add 场景：

```bash
gix add
gix add -A
gix add --all
gix add -u
gix add --update
gix add -p
gix add --patch
```

### `gix commit`

基于 staged diff 生成 commit message。

支持：

- AI 生成
- fallback 启发式生成
- 手动编辑
- 重新生成
- dry-run
- amend

示例：

```bash
gix commit
gix commit -y
gix commit --dry-run
gix commit --amend
gix commit --scope cli
gix commit --type feat
gix commit --lang zh
gix commit --style conventional
```

### `gix push`

推送当前分支，支持 remote、branch、tags、set-upstream、force-with-lease：

```bash
gix push
gix push --remote origin
gix push --branch main
gix push --set-upstream
gix push --tags
gix push --force
```

### `gix ac`

等价于：

```bash
gix add
gix commit
```

### `gix acp`

等价于：

```bash
gix add
gix commit
gix push
```

### `gix sync`

用于同步远端更新，默认优先走 rebase 流程。

支持：

- 从 `origin` 或 `upstream` 拉取
- 指定 remote / branch
- `--merge` 或 `--rebase`
- `--only-fetch`
- 同步后自动 push

示例：

```bash
gix sync
gix sync --from-upstream
gix sync --remote origin --branch main
gix sync --merge
gix sync --only-fetch
gix sync --push
```

### `gix tag`

#### 创建标签

```bash
gix tag create
gix tag create --name v1.2.3
gix tag create --auto
gix tag create --message "release v1.2.3"
gix tag create --push
gix tag create --from main
```

默认支持根据 `tag.current` 或当前最新 tag 自动递增版本。

#### 查看标签

```bash
gix tag list
gix tag list --latest
gix tag list --sort version
gix tag list --sort date
```

#### 推送标签

```bash
gix tag push
gix tag push --name v1.2.3
gix tag push --all
```

#### 删除标签

```bash
gix tag delete --name v1.2.3
gix tag delete --name v1.2.3 --remote
gix tag delete --name v1.2.3 --yes
```

#### 从标签检出 / 建分支

```bash
gix tag checkout v1.2.3
gix tag branch v1.2.3 release/v1.2.x
```

### `gix branch`

```bash
gix branch list
gix branch new feature/login
gix branch switch main
gix branch delete feature/old
gix branch delete --force feature/old
gix branch rename old-name new-name
gix branch cleanup
```

`branch cleanup` 会先列出已合并分支，再要求确认。

### `gix repo`

```bash
gix repo info
gix repo remote
gix repo set-default-remote origin
gix repo set-main-branch main
```

用于维护 gix 记住的仓库级信息。

### `gix config`

```bash
gix config init
gix config get commit.language
gix config get --global ai.model
gix config set commit.language zh
gix config set --global ai.model gpt-5-mini
gix config list
gix config list --global
gix config list --project
```

## 配置文件

### 配置路径

全局配置：

```bash
~/.gix/config.toml
```

项目配置：

```bash
.git/gix.toml
```

### 配置优先级

从低到高：

1. 默认值
2. 全局配置
3. 项目配置

### 常见配置项

```toml
[ai]
provider = "openai"
model = "gpt-5-mini"
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
timeout = 30
language = "en"
thinking = true

[commit]
style = "conventional"
language = "en"
default_type = ""
default_scope = ""
with_body = false
confirm = true
max_subject_length = 72

[push]
default_remote = "origin"

[ui]
color = true
interactive = true

[project]
name = ""
main_branch = "main"
default_remote = "origin"
upstream_remote = "upstream"

[tag]
enabled = true
prefix = "v"
pattern = "semver"
current = ""
auto_increment = "patch"
annotated = true
push_after_create = false

[self_update]
repo = ""
base_url = "https://api.github.com"
```

### 推荐配置示例

```bash
gix config set --global ai.model gpt-5-mini
gix config set --global ai.api_key sk-...
gix config set --global ai.language zh
gix config set --global ai.thinking true
gix config set commit.default_scope cli
gix config set project.main_branch main
gix config set --global self_update.repo your-org/gix
```

## AI Commit Message

如果配置了 `ai.api_key`，`gix commit` 会调用 OpenAI-compatible 的 `/v1/chat/completions` 接口，并使用 `stream` 方式生成 commit message。

当前实现要点：

- `base_url` 采用 `https://xxx.com/v1` 形式
- `ai.thinking=true` 时请求 `reasoning_effort=high`
- `ai.thinking=false` 时请求 `reasoning_effort=none`
- 实际请求路径为 `/chat/completions`
- 流式解析主内容
- 如果服务端返回 reasoning / thinking 增量，也会在终端单独显示思考区域
- 如果服务端不返回 reasoning，只显示主内容区域

如果没有配置 API Key，命令依然可用，但会自动退回到内置 fallback 生成器。

这意味着：

- 不会因为没有配置 API Key 而让 `gix commit` 完全不可用
- 可以先把命令流用起来，再按需接入 AI

## 多语言

当前支持：

- English
- 简体中文

默认语言为英文，但会按操作系统语言自动切换。也可以手动覆盖：

```bash
GIX_LANG=en gix help
GIX_LANG=zh gix help
```

多语言主要覆盖：

- help
- 安装/卸载/自更新提示
- completion 提示
- 常见命令输出
- 大部分受控错误信息

Git 原始输出和系统底层错误不会强行翻译。

## Shell Completion

`gix` 现在支持：

- 命令补全
- flag 补全
- branch 补全
- tag 补全
- remote 补全

推荐方式：

```bash
gix completion
```

它会自动检测当前 shell 并安装 completion。

卸载：

```bash
gix completion uninstall
```

打印脚本：

```bash
gix completion print
gix completion print zsh
gix completion print bash
gix completion print fish
gix completion print powershell
```

兼容旧写法：

```bash
gix completion zsh
gix completion bash
```

## 安装与卸载

### 安装

```bash
./gix install
```

安装时会：

- 复制当前二进制到用户 bin
- 更新用户 PATH
- 更新当前进程 PATH
- 安装 shell completion

### 卸载

```bash
gix uninstall
```

卸载时会：

- 删除安装后的二进制
- 清理 PATH 配置块
- 清理 completion 配置

## 自更新

```bash
gix self-update --check
gix self-update
gix self-update --version v0.2.0
```

自更新流程：

1. 读取当前版本
2. 查询 GitHub Releases
3. 找到当前平台匹配的二进制资源
4. 下载二进制和 checksum 文件
5. 校验完整性
6. 安装到用户 bin 目录

如果你不是通过正式 Release 构建出来的二进制使用 `gix`，需要显式配置：

```bash
gix config set --global self_update.repo your-org/gix
```

## GitHub Releases 与自动发布

项目内已经包含 GitHub Actions 发布工作流：

```text
.github/workflows/release.yml
```

默认支持：

- macOS / Linux / Windows
- `amd64` / `arm64`

发布方式：

1. 推送 `v*` tag
2. GitHub Actions 自动交叉编译
3. 生成对应平台二进制
4. 生成 checksum 文件
5. 上传到 GitHub Release

`self-update` 依赖该 Release 产物命名规则。

## 开发与测试

### 本地运行

```bash
go run ./cmd/gix help
```

### 运行测试

```bash
go test ./...
```

如果你的环境对默认 Go build cache 有限制，可以这样：

```bash
GOCACHE=/tmp/gix-go-cache go test ./...
```

### 格式化

```bash
gofmt -w ./cmd ./internal
```

## 常见问题

### 1. 为什么第一次安装要用 `./gix install`？

因为下载后的当前目录通常不在 `PATH` 里。安装完成后，重新打开终端即可直接使用 `gix`。

### 2. 为什么已经是中文系统，输出还是英文？

优先级是：

1. `GIX_LANG`
2. 系统语言检测
3. locale 环境变量

可以先验证：

```bash
GIX_LANG=zh gix help
```

### 3. 没有配置 OpenAI API Key 还能用吗？

可以。`gix commit` 会退回到 fallback 生成逻辑。

### 4. completion 安装后没有立即生效怎么办？

通常重新打开终端就可以。如果还不生效，手动 source 对应 shell 配置文件即可。

## 当前边界

当前版本仍然有这些明确边界：

- 不尝试替代完整 Git，只做常见辅助流
- 不翻译 Git 自身的原始输出
- 不对所有系统底层错误做本地化
- AI commit 生成目前主要聚焦在 v1 的提交流程

这也是有意为之，优先把最常用、最稳定、最容易维护的部分做好。
