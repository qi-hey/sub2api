# Grok Forbidden 筛选与批量删除实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让管理员可以服务器端筛选所有 Grok Forbidden 账号，并在数量校验和危险确认后一次请求分批删除全部匹配账号。

**架构：** 账号列表沿用现有 `status` 查询参数，新增特殊值 `forbidden`，仓储层将其翻译为 Grok 用量快照 JSONB 中的 HTTP 403 条件。新增专用批量删除接口，服务端重新解析同一过滤条件、校验前端确认数量，再通过现有 `DeleteAccount` 以有界并发执行删除。前端只在 Forbidden 过滤器生效时展示危险操作按钮。

**技术栈：** Go 1.26、Gin、Ent/PostgreSQL JSONB、Vue 3、TypeScript、Vitest、vue-i18n。

---

### 任务 1：服务器端 Forbidden 列表过滤

**文件：**
- 修改：`backend/internal/service/account.go`
- 修改：`backend/internal/repository/account_repo.go`
- 测试：`backend/internal/repository/account_repo_integration_test.go`

- [ ] **步骤 1：编写失败的仓储集成测试**

创建 Grok 403、Grok 200、OpenAI 403 快照和普通 active 账号，调用 `ListWithFilters(..., status="forbidden", ...)`，断言只返回 Grok 403 账号且总数为 1。

- [ ] **步骤 2：运行测试验证失败**

运行：

```powershell
go test -tags integration ./internal/repository -run 'TestAccountRepoSuite/TestListWithFilters_Forbidden' -count=1
```

预期：FAIL，当前实现把 `forbidden` 当作 `accounts.status`，返回 0 条。

- [ ] **步骤 3：实现最小过滤逻辑**

在 service 账号常量中定义 `AccountStatusForbiddenFilter = "forbidden"`。仓储状态 switch 增加分支，要求 `platform = grok` 且 `extra.grok_usage_snapshot.status_code = 403`，其余状态分支保持不变。

- [ ] **步骤 4：运行测试验证通过**

重复步骤 2 命令，预期 PASS。

- [ ] **步骤 5：提交**

```powershell
git add backend/internal/service/account.go backend/internal/repository/account_repo.go backend/internal/repository/account_repo_integration_test.go
git commit -m "feat: filter forbidden Grok accounts"
```

### 任务 2：安全的 Forbidden 全量删除接口

**文件：**
- 修改：`backend/internal/handler/admin/account_handler.go`
- 修改：`backend/internal/handler/admin/admin_service_stub_test.go`
- 修改：`backend/internal/handler/admin/account_handler_list_test.go`
- 修改：`backend/internal/server/routes/admin.go`

- [ ] **步骤 1：编写失败的处理器测试**

覆盖以下行为：

```text
非 forbidden 条件 -> 400，DeleteAccount 零调用
实时数量与 expected_count 不同 -> 409，DeleteAccount 零调用
匹配数量一致 -> 200，所有匹配 ID 各删除一次
个别删除失败 -> 200，success/failed/failed_ids 统计准确
```

- [ ] **步骤 2：运行测试验证失败**

运行：

```powershell
go test ./internal/handler/admin -run 'TestAccountHandlerBulkDeleteForbidden' -count=1
```

预期：FAIL，路由和处理器尚不存在。

- [ ] **步骤 3：实现请求校验、目标解析和有界删除**

新增 `POST /accounts/bulk-delete-forbidden`。请求必须包含 `filters.status=forbidden` 和非负 `expected_count`。按 500 条分页解析目标 ID，限制最多 5000 条；实时数量不等于预期时返回 `FORBIDDEN_COUNT_CHANGED` 409。使用 4 个 worker 调用现有 `DeleteAccount`，聚合成功和失败结果。

- [ ] **步骤 4：运行处理器测试验证通过**

重复步骤 2 命令，预期 PASS。

- [ ] **步骤 5：提交**

```powershell
git add backend/internal/handler/admin/account_handler.go backend/internal/handler/admin/admin_service_stub_test.go backend/internal/handler/admin/account_handler_list_test.go backend/internal/server/routes/admin.go
git commit -m "feat: bulk delete filtered forbidden accounts"
```

### 任务 3：前端筛选与危险操作入口

**文件：**
- 修改：`frontend/src/components/admin/account/AccountTableFilters.vue`
- 修改：`frontend/src/components/admin/account/AccountBulkActionsBar.vue`
- 修改：`frontend/src/views/admin/AccountsView.vue`
- 修改：`frontend/src/api/admin/accounts.ts`
- 修改：`frontend/src/i18n/locales/zh/admin/accounts.ts`
- 修改：`frontend/src/i18n/locales/en/admin/accounts.ts`
- 创建：`frontend/src/components/admin/account/__tests__/AccountTableFilters.forbidden.spec.ts`
- 修改：`frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`

- [ ] **步骤 1：编写失败的组件和页面测试**

断言状态选项包含 `forbidden`；选择该状态时平台同步为 `grok`；仅 Forbidden 过滤器且总数大于 0 时出现“删除全部”按钮；点击确认后 API 请求包含过滤快照与 `expected_count`。

- [ ] **步骤 2：运行测试验证失败**

运行：

```powershell
pnpm test:run src/components/admin/account/__tests__/AccountTableFilters.forbidden.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts
```

预期：FAIL，选项、按钮和 API 方法尚不存在。

- [ ] **步骤 3：实现最小前端行为**

新增 i18n 文案和 API 方法；状态筛选选择 Forbidden 时同步平台为 Grok；操作栏接受 Forbidden 总数和 loading props；页面确认后调用接口，成功或部分失败均刷新列表，409 时提示结果数量已变化。

- [ ] **步骤 4：运行前端测试验证通过**

重复步骤 2 命令，预期 PASS。

- [ ] **步骤 5：提交**

```powershell
git add frontend/src/components/admin/account/AccountTableFilters.vue frontend/src/components/admin/account/AccountBulkActionsBar.vue frontend/src/views/admin/AccountsView.vue frontend/src/api/admin/accounts.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts frontend/src/components/admin/account/__tests__/AccountTableFilters.forbidden.spec.ts frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts
git commit -m "feat: delete all filtered forbidden accounts"
```

### 任务 4：回归、构建与部署

**文件：**
- 修改：`PRIVACYFILTER_WORKFLOW.md`
- 创建：`build-artifacts/v161-forbidden-delete-build.txt`

- [ ] **步骤 1：运行后端受影响包测试**

```powershell
go test ./internal/handler/admin ./internal/repository ./internal/service ./internal/server/routes -count=1
```

预期：全部 PASS；若完整 service 包仅出现已知时序测试失败，单独复跑该测试并记录。

- [ ] **步骤 2：运行前端验证**

```powershell
pnpm lint:check
pnpm typecheck
pnpm test:run
pnpm build
```

预期：全部 exit 0。

- [ ] **步骤 3：更新二开清单并构建 Linux 产物**

记录 Forbidden 筛选、受保护全量删除、测试结果、版本和 SHA256。使用现有 embed 发布流程构建 `linux/amd64` 二进制。

- [ ] **步骤 4：备份并部署 VPS**

备份 `/opt/sub2api/sub2api` 和 PostgreSQL 数据库，上传新产物，原子替换二进制并重启服务。

- [ ] **步骤 5：生产只读验收**

验证服务 active、`/health` 为 200、`status=forbidden` 返回 HTTP 200 且总数与数据库 403 快照数量一致。不得在验收阶段调用删除接口。

- [ ] **步骤 6：提交发布记录**

```powershell
git add PRIVACYFILTER_WORKFLOW.md build-artifacts/v161-forbidden-delete-build.txt
git commit -m "docs: record forbidden deletion build"
```
