## Context

W1-03 是第一个真实业务流模块：把"注册 Git 模块 → 发布到服务目录"的链路打通。

**现有架构约束**（已落地）：
- `server/core/catalog/validator.go`（D40 JSON Schema 校验器）已实现 + 有 testdata
- `server/core/registry/`（空目录，等实现）
- `server/api/connect/catalog.go`（占位 handler，`UnimplementedCatalogServiceHandler`）
- `server/api/connect/` 无 registry handler
- `server/data/repo/{module,module_version,catalog}.go`（W1-02 落地，含 CreateWithVersion 事务）
- proto 契约：`contracts/platform/v1/{registry,catalog}/{dto,srv}.proto` 已冻结
- `RegistryAdminService.RegisterModule(git_source, module_path, version, provider, name, description, owner_team_id)`

**D25 零侵入原则**（关键约束）：
- 模块 `variables.tf` 全 scalar，`main.tf` 单实例，`outputs.tf` scalar 输出
- `variables_contract_json` 只存从 variables.tf 提取的**纯 scalar 契约**（name/type/default/description）
- 不要求模块作者按平台规范修改（社区模块直接用）

## Goals / Non-Goals

**Goals:**
- 实现 core/registry：Git clone + HCL 解析 → scalar 契约 + 状态机
- 实现 GitProvider 真实版（go-git）
- 补全 core/catalog：Publish + FormSchema + Defaults + Visibility
- 接 api/connect handler 到 DB
- fixture 模块测试

**Non-Goals:**
- 不做 terraform validate 真实执行（MVP 用 HCL 解析校验代替）
- 不做 D23 凭据注入完整实现（MVP 用环境变量）
- 不做 module_dependencies 自动推断（W2）
- 不改 DB schema / proto 契约
- 不做批量注册脚本（task 3.5 YAGNI）

## Decisions

### D1：HCL 解析用 terraform-config-inspect（非 hcl/v2 或 terraform CLI）

**决策**：用 `github.com/hashicorp/terraform-config-inspect/tfconfig` 解析 `.tf` 文件。

**理由**：
- **HashiCorp 官方库**，专门为"读取 tf 模块结构"设计（terrafmt / terraform-ls 都用它）
- 输出 `tfconfig.Module` struct，含 `VariableInputs map[string]*Variable`（Name/Type/Default/Description/Sensitive）——正好匹配我们要的纯 scalar 契约
- 轻量（纯 Go 解析，不调 terraform CLI，不需 provider 下载）
- 覆盖 variables.tf + outputs.tf（contract 需要两者的 scalar 提取）

**备选**：①hcl/v2（底层，需手写提取 200+ 行）②terraform CLI（需装 terraform + init，慢且重）。tfconfig 是 sweet spot。

**影响**：go.mod 新增 `github.com/hashicorp/terraform-config-inspect`。

### D2：ContractExtractor 输出纯 scalar（D25 零侵入）

**决策**：ContractExtractor 从 tfconfig.Module 提取后，**只保留 scalar 字段**（string/number/bool），丢弃复杂类型（list/map/object）的 default 值（但保留 type 声明）。

```go
type VariableContract struct {
    Name        string `json:"name"`
    Type        string `json:"type"`         // raw HCL type string
    Default     any    `json:"default"`      // scalar only; complex → nil
    Description string `json:"description"`
    Sensitive   bool   `json:"sensitive"`
    Required    bool   `json:"required"`     // default == nil
}
```

**理由**（D25）：
- 原子模块保持单实例语义，复杂类型 default 不进契约（避免 codegen 处理嵌套）
- cardinality 是 catalog 层配置（per_instance_fields），不是模块契约
- 社区模块（terraform-aws-modules / alicloud 官方）直接复用，零适配

**备选**：保留完整 default（复杂类型）→ codegen 难处理 + 契约膨胀。

### D3：FormSchemaGenerator 从契约裁剪（非全暴露）

**决策**：`form_schema_json` 从 `variables_contract_json` 裁剪出"用户可见字段"。裁剪规则：
- **隐藏**：`sensitive=true`（占位符，运行期注入）、`required=true` 且无 default 且平台可推断（如 region 从 env 推断）
- **暴露**：`required=true` 且平台不可推断（用户必填）、`required=false` 且有 default（用户可改）

输出 JSON Schema（Draft 2020-12，D40 校验器兼容）：
```json
{
  "type": "object",
  "properties": {
    "instance_type": {"type": "string", "description": "...", "default": "rds.mysql.s2.large"},
    "storage_size": {"type": "number", "description": "..."}
  },
  "required": ["instance_type"]
}
```

**理由**：用户不需要看到所有 36 个变量（region/credentials 平台注入）。form_schema 是"用户操作面板"，不是"完整变量清单"。

**备选**：全暴露（用户困惑）。

### D4：GitProvider 实现用 go-git（非 exec git CLI）

**决策**：`core/adapters/git/gogit.go` 用 `github.com/go-git/go-git/v5`（已在 go.mod）实现 GitProvider。

**理由**：
- go-git 纯 Go，无外部 git 依赖（process 模式 Executor 不需装 git）
- 已在 go.mod（W1-01 时引入）
- 支持 clone/fetch/checkout（满足 module 注册需求）

**备选**：exec git CLI（需装 git，process 模式违背零依赖）。

**凭据**：MVP 用环境变量（GIT_SSH_COMMAND / GIT_TOKEN），D23 完整凭据注入留 W2。

### D5：registry handler 在 api/connect/（非 internal/）

**决策**：`api/connect/registry.go` 实现 `RegistryAdminServiceHandler`，接 RegistryService。

**理由**：
- RegistryAdminService 是 Admin 层（proto 定义），需 admin 角色（Connect 拦截器校验）
- catalog handler 已在 api/connect/，保持一致
- core/registry 只做业务逻辑，不感知 RPC

### D6：terraform validate 降级（MVP HCL 解析代替）

**决策**：MVP 阶段，模块注册的"校验"降级为 HCL 解析成功 = validated。真实 terraform validate（init + validate）留 W2（需装 terraform + provider mirror）。

**状态机**：`pending_validation`（注册时）→ `validated`（HCL 解析成功）/ `validation_failed`（解析失败）。

**理由**：terraform validate 需要 provider 下载（网络 + 版本管理），MVP 用 HCL 语法校验已能过滤 90% 错误（语法错/变量缺失）。

## Risks / Trade-offs

- **[terraform-config-inspect 不解析表达式] → 契约的 default 可能不完整**：tfconfig 对 complex default 返回 `cty.Value`，我们只取 scalar。风险可控——D25 要求原子模块 scalar-only，复杂 default 是反模式。
- **[GitProvider 凭据简陋] → SSH/token 仅环境变量**：MVP 够用（CI/本地开发），D23 完整凭据注入（Vault/KMS）留 W2。
- **[FormSchema 裁剪规则可能不全] → testdata fixture 覆盖典型场景**：用 alicloud RDS / ECS 模块 fixture 验证裁剪逻辑。
- **[HCL 解析降级不等于 terraform validate] → 部分语义错误漏检**：MVP 可接受，W2 补真实 validate。
- **[registry handler 需 admin 角色] → 拦截器配置**：RegistryAdminService.* 需 admin，W1-03 实现 handler + 基础鉴权 stub（真实 OIDC 留 W2）。

## Migration Plan

1. 引入 `terraform-config-inspect` 依赖
2. 实现 GitProvider（go-git）+ ContractExtractor（tfconfig）
3. 实现 RegistryService（注入 ModuleRepo + GitProvider）
4. 实现 CatalogService（注入 CatalogRepo + FormSchemaGenerator）
5. 新建 api/connect/registry.go + 改造 catalog.go
6. 写 testdata fixture（2-3 个典型 tf 模块）
7. 测试：契约提取 + form_schema 裁剪 + defaults + visibility
8. `go build ./... && go vet ./... && go test ./server/core/{registry,catalog}/...`

**回滚**：core/registry + GitProvider 实现删除即回退到 noop；handler 改回占位。无 schema 变更。

## Open Questions

- **FormSchema 裁剪规则是否需要 catalog_item 级覆盖？** 当前裁剪从契约自动生成，catalog_item 可在 publish 时手动覆盖 form_schema_json。倾向：自动生成 + 允许手动覆盖（MVP 默认自动）。
- **module_path 在 proto 是单独字段，但 GitProvider clone 整个 repo 后需要 cd 到子目录？** 倾向：clone repo → tfconfig.Load 那个 module_path 子目录。tfconfig 支持 LoadDir(path)。
