# 技术栈深度调研存档（D39 任务队列 / D40 JSON Schema / D41 OTel）

> 日期：2026-07-07
> 关联：回应 D39/D40/D41 决策的"是否有更好的解决方案"复核
> 状态：三决策均**保持不变**，但补充了实现细节与生态全景，备未来回溯

## 0. TL;DR

| 决策 | 复核结论 | 补充 |
|---|---|---|
| D39 任务队列 | **保持 River** | 补：作业队列 vs 事件总线区分；asynq 维护放缓实测；事件总线 Phase 1 进程内 → 后续 NATS |
| D40 JSON Schema | **保持 santhosh-tekuri/v6** | 补：4 条实现硬约定；Google 2026-01 新包不换（为 LLM 优化）；vearutop benchmark 实证 |
| D41 OTel | **保持 OTel + otelzap** | 补：明确 Mode 1（trace_id 字段注入，不依赖 Beta Logs SDK）；metrics 用 OTel SDK + Prometheus exporter |

---

## 1. 任务队列全景（D39 复核）

### 1.1 全景表（2026-07 实测 GitHub 数据）

| 库 | 类型 | 后端 | Stars | 最后发布 | 状态 |
|---|---|---|---|---|---|
| **River**（选定）| 完整作业队列 | PostgreSQL | 5,406 | 2026-07-02（v0.40）| 🟢 月度发布 |
| asynq | 完整任务队列 | Redis | 13,492 | 2026-02（v0.26）| 🟡 14-18 月间隔 |
| machinery | Celery 式 | Redis/RabbitMQ/SQS | 7,962 | 2025-11 | 🟡 245 open issues |
| PGMQ | 队列表（无 worker）| PostgreSQL | 5,004 | 2026-07 | 🟢 但要自建 worker |
| gocraft/work | Sidekiq 式 | Redis | 2,524 | 2024-06 | 🔴 停更 |
| robfig/cron | cron 调度器 | 进程内 | 14,151 | 2024-07 | 🟡 非队列 |
| go-co-op/gocron | cron 调度器 | 进程内 | 7,091 | 2026-07 | 🟢 非队列 |
| NATS JetStream | 流/broker | 自身 | 6,682（Go client）| 2026-07 | 🟢 事件总线候选 |

### 1.2 River vs asynq 决定性对比

| 维度 | River | asynq |
|---|---|---|
| **事务原子性** | ✅ `river.InsertTx(tx, job)` 入队与 DB 写原子 | ❌ Redis 在 PG 事务外，需 outbox 模式 |
| 新基础设施 | 无（用现有 PG）| 要部署 Redis |
| 维护活跃（2026）| ✅ 月度（v0.40, 2026-07）| 🟡 14-18 月间隔（v0.26, 2026-02）|
| 类型安全 | ✅ generics `river.Job[MyArgs]` | ❌ 手动 payload |
| Web UI | ✅ riverui 活跃 | ❌ asynqmon 2024-05 停更 |

### 1.3 作业队列 vs 事件总线（关键区分，补入 D39）

| 问题 | 特征 | 工具 |
|---|---|---|
| **作业队列** | do once + retry + DLQ，消费即删除 | River（terraform plan/apply/codegen）|
| **事件总线** | 多订阅者各反应同一事件，可重放 | Phase 1 进程内 dispatcher → 后续 NATS JetStream |

混用会出错：River 是单消费者作业，不是 fan-out pub-sub。

### 1.4 参考

- [riverqueue/river](https://github.com/riverqueue/river) · [Brandur River 写作](https://brandur.org/river)
- [hibiken/asynq](https://github.com/hibiken/asynq) · [asynqmon 停更](https://github.com/hibiken/asynqmon)
- [transactional outbox 模式](https://packagemain.tech/p/how-to-implement-the-outbox-pattern-in-golang)
- [NATS JetStream work-queue](https://docs.nats.io/nats-concepts/jetstream/consumers)

---

## 2. JSON Schema 校验全景（D40 复核）

### 2.1 全景表

| 库 | Draft 支持 | 维护 | 性能 | 正确性 |
|---|---|---|---|---|
| **santhosh-tekuri/v6**（选定）| 4/6/7/2019-09/**2020-12** | ✅ v6.0.2 | 快 | **最高**（Bowtie 通过）|
| google/jsonschema-go 🆕 | 7/2020-12 | ✅（2026-01 发布）| TBD | TBD（6 月新）|
| kaptinlin/jsonschema | 4/6/7/2019-09/2020-12 | ✅ 新 | 声称高 | 未独立基准 |
| swaggest/jsonschema-go | 2019-09/2020-12 | ✅ | 快 | 良 |
| qri-io/jsonschema | 7/2019-09 **only** | 低 | 快 | 测试失败多 |
| xeipuuv/gojsonschema | 4/6/7 | ❌ 废弃 | 慢 3-4× | Draft-07 止 |

### 2.2 vearutop benchmark 关键结论

> "要快的选 santhosh-tekuri 或 qri-io；要准的选 santhosh-tekuri。"

santhosh-tekuri 是**唯一同时最快且最准**的。qri-io 快但不支持 2020-12 + 测试失败。

### 2.3 Google jsonschema-go 为何不换

- 定位：**LLM/Gemini schema 推断**（从 Go struct 生成 schema），非通用校验
- 6 个月新（2026-01），无 Bowtie 记录
- 你的场景是运行时校验用户 schema，非 LLM 生成 → 12 个月后观望

### 2.4 实现硬约定（补入 D40）

1. 编译一次复用：`jsonschema.NewCompiler()` 启动时编译 meta-schema
2. 注册自定义 format：module path/ARN/semver
3. 禁用远程 `$ref`：安全 + 确定性
4. 两级校验：meta-schema → 用户 schema → 实例

### 2.5 参考

- [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema)
- [vearutop benchmark](https://dev.to/vearutop/benchmarking-correctness-and-performance-of-go-json-schema-validators-3247)
- [google/jsonschema-go 公告](https://opensource.googleblog.com/2026/01/a-json-schema-package-for-go.html)
- [Bowtie 合规测试](https://bowtie.report)

---

## 3. OpenTelemetry 复核（D41）

### 3.1 OTel 是 2026 标准吗

**是。** CNCF 项目，已替代 Jaeger client/Zipkin/Datadog tracer 等厂商 SDK。仪器化一次，后端可换。

### 3.2 otelzap 两种模式（补入 D41，明确选 Mode 1）

| 模式 | 做什么 | 依赖 | 风险 | 选定？ |
|---|---|---|---|---|
| **Mode 1：trace_id 字段注入** | zap 正常输出，otelzap 注入 trace_id 为字段 | 稳定 Traces SDK | **低** | ✅ |
| Mode 2：OTLP 日志导出 | zap 日志转 OTel log record 发 Collector | **Beta** Logs SDK | 中（#6114 坑）| 推迟 |

Mode 1 下"OTel Logs 是 Beta"与你无关 —— 只依赖稳定 Traces SDK + zap。

### 3.3 metrics：OTel SDK vs prometheus/client 直连

| 方式 | 优点 | 缺点 |
|---|---|---|
| **OTel metrics SDK + Prometheus exporter**（选定）| 统一一个 SDK 管 trace+metrics | 略复杂 |
| prometheus/client_golang 直连 | 极简 | 与 trace 两套仪表盘 |

选定 OTel SDK：统一三支柱，未来 +logs 一致。

### 3.4 后端（与仪表盘无关，OTel 是中间层）

- trace：Jaeger / Tempo
- metrics：Prometheus / VictoriaMetrics
- logs：Loki / Datadog

### 3.5 参考

- [opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go)
- [otelzap](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelzap)
- [opentelemetry-go#6114（Beta Logs 坑）](https://github.com/open-telemetry/opentelemetry-go/issues/6114)
- [CNCF landscape](https://landscape.cncf.io)

---

## 4. 结论

三决策经 2026-07 生态复核**全部保持不变**，但补充了：
- D39：作业队列 vs 事件总线区分（避免 River 误用做 pub-sub）
- D40：4 条实现硬约定 + Google 新包观望
- D41：otelzap Mode 1 明确（不依赖 Beta）+ metrics 用 OTel SDK

未来重评触发条件：
- River：超 50-100k jobs/s 或引入 Redis（其他用途）→ 重评 asynq/NATS
- santhosh-tekuri：12 个月后 Google jsonschema-go 出 Bowtie 数据 → 重评
- OTel：Go Logs SDK Stable 后 → 评估 Mode 2
