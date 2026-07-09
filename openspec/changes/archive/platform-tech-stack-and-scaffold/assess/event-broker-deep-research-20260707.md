# 事件总线 / 消息中间件深度调研存档（D39 事件总线部分深潜）

> 日期：2026-07-07
> 关联：深潜 D39 中"事件总线 Phase 1 进程内 → 后续 NATS"的初步结论，回答"现在要不要建可替换的 pub-sub 端口驱动接口（port driver），还是现在就定一个 broker"。
> 状态：**RESERVE 不锁死** —— 验证 D39 的方向正确，并给出具体的抽象接口契约与升级触发条件。
> 数据来源：GitHub 仓库页（2026-07 实测版本/许可证/发布日期）+ 2025-2026 对比文章 + WebSearch 8 轮。星数为实测或近似（±0.5k），版本/许可证/发布日期为硬数据。

---

## 0. TL;DR（决策级结论）

| 问题 | 结论 |
|---|---|
| **现在要不要锁死一个 broker？** | **不锁。** 你的事件量级（`drift.detected`/`approval.*`/`migration.*`）是 10²–10³ evt/s 量级，所有 broker 都远未到瓶颈，过早选型是负债。 |
| **D39 的方向对吗？** | **对，且证据更强。** Phase 1 进程内 dispatcher（typed registry + channels）足够；升级路径首选 **NATS JetStream**（Go 原生、单二进制、CNCF），次选 RocketMQ（仅当明确部署到强偏好 RocketMQ 的中国企业客户环境）。 |
| **要不要现在就建可替换的 pub-sub port driver 接口？** | **要，但建得薄。** 一个极小的 `EventBus` 接口 + in-process 实现（Phase 1 唯一实现），**不是**一次造 4 个 driver。接口只为"未来换实现时不改业务代码"而存在。有成熟的 Go 先例：`looplab/eventhorizon` 的 EventBus 接口（Local/NATS/Kafka/GCP/Redis 五 driver）。 |
| **PG LISTEN/NOTIFY 够吗？** | **作为进程内 dispatcher 的可选传输层够，但不应承担"事件总线"职责。** 8000 字节 payload 上限 + 长连接 + 无 ack/无重放 + 与 PgBouncer 事务池模式冲突。 |
| **事件总线与 River 关系？** | **解耦。** River = 作业队列（do-once + retry + DLQ，消费即删）；EventBus = pub-sub（多订阅者各自反应同一事件，可重放）。混用会出错（River 是单消费者，不是 fan-out）。 |

---

## 1. 任务一：Broker 全景（2025-2026 实测）

### 1.1 服务端全景表

| Broker | Stars（近似）| 最新版本/日期 | 许可证 | 语言 | 架构亮点 |
|---|---|---|---|---|---|
| **apache/kafka** | ~30k+ | Kafka 4.x（2025-2026，KRaft 已去 ZK）| Apache 2.0 | Java/Scala | 事实标准流平台；4.0 起 KRaft 为唯一模式（ZooKeeper 移除）|
| **apache/rocketmq** | ~21k | **v5.5.0**（实测）| Apache 2.0 | Java | 阿里系，中国企业主导；5.0 gRPC 协议；LiteTopic for AI |
| **apache/pulsar** | ~14k | 4.x | Apache 2.0 | Java | 计算存储分离（Broker 无状态 + BookKeeper）；分层存储原生 |
| **nats-io/nats-server** | ~16k | v2.11.x | Apache 2.0 | **Go** | 单二进制、CNCF、JetStream 持久化、thread-per-core |
| **rabbitmq/rabbitmq-server** | ~13.7k | 4.x | MPL-2.0 | Erlang | 经典 AMQP broker；经典/仲裁/流三种队列类型 |
| **redpanda-data/redpanda** | ~10k+ | **v25.2.7**（实测）| BSL → 后续宽松 | **C++(Seastar)** | Kafka 线协议兼容、无 JVM、thread-per-core、WASM transform |
| **Redis Streams**（Redis/Valkey）| redis ~67k / valkey ~20k | Redis 8.x / Valkey 8.x | RSALv2/SSPL（Redis）/ BSD-3（Valkey）| C | XREADGROUP/XACK/XCLAIM 消费者组；AOF/RDB 持久化 |
| **PostgreSQL LISTEN/NOTIFY** | （PG 内建）| PG 17/18 | PostgreSQL Lic | C | 同库内 fire-and-forget；**8000 字节 payload 上限** |

> Memphis.dev：2025 社区讨论中已基本消失，无官方 EOL 公告但生态停滞 → **排除**。PubSub（GCP）/SQS（AWS）是托管云服务，与"self-hosted"硬约束冲突 → **排除**。

### 1.2 Go 客户端全景表（关键，本项目是 Go）

| 客户端 | Stars | 最新版本/日期 | 许可证 | 纯 Go？ | 质量评估（2025-2026）|
|---|---|---|---|---|---|
| **twmb/franz-go**（Kafka，首选）| ~2k+ | v1.21.x（实测，支持 Kafka 4.2+）| BSD-3 | ✅ 纯 Go | 🟢 **2025 新项目首选**；feature-complete（produce/consume/transact/admin）；kadm 配套；无 CGO |
| **IBM/sarama**（Kafka）| **12.5k**（实测）| **v1.50.2（2026-06-05）**| MIT | ✅ 纯 Go | 🟡 成熟但团队在迁向 franz-go；14 open issues；仍维护 |
| confluentinc/confluent-kafka-go | ~4.5k | 2.x | Apache 2.0 | ❌ **CGO/librdkafka** | 🔴 CGO 是部署/交叉编译痛点，采用度下降 |
| segmentio/kafka-go | ~7.3k | v0.4.x | MIT | ✅ 纯 Go | 🟡 中间选择，活跃度低于 franz-go |
| **nats-io/nats.go**（首选）| ~5.5k | **v1.52.0**（实测）| Apache 2.0 | ✅ 纯 Go | 🟢 **官方一等公民**；与 nats-server 同仓出品；JetStream 一等支持 |
| **redis/go-redis**（Streams）| ~20.5k | v9（实测，支持 Redis-8.8）| BSD-2 | ✅ 纯 Go | 🟢 官方维护；Streams API 完整（XADD/XREADGROUP/XACK/XCLAIM/XAUTOCLAIM）|
| rabbitmq/amqp091-go | ~1.3k | v1.x | BSD-2 | ✅ 纯 Go | 🟡 官方接续 streadway/amqp；AMQP 0-9-1 完整但无流协议 client |
| apache/pulsar-client-go | ~1.3k | 0.15.x | Apache 2.0 | ✅ 纯 Go | 🟡 官方但 Go client 成熟度低于 Java client |
| apache/rocketmq-client-go (v2 Remoting) | ~1.4k | v2.1.x | Apache 2.0 | ✅ 纯 Go | 🟢 4.x 成熟，ACL/事务/定时消息齐全 |
| apache/rocketmq-clients/golang (v5 gRPC) | （monorepo）| v5.1.x | Apache 2.0 | ✅ 纯 Go（grpc-go+protobuf）| 🟡 5.x 正道，但较新；[#1055 gRPC failover](https://github.com/apache/rocketmq-clients/issues/1055) 等开放 issue |
| **riverqueue/river**（作业队列，非 broker）| **5,406**（实测）| **v0.40（2026-07-02）**| MPL-2.0 | ✅ 纯 Go | 🟢 月度发布；见 D39 |
| looplab/eventhorizon（抽象先例）| ~1.7k | v1.x | MIT | ✅ 纯 Go | 🟢 EventBus 接口 + Local/NATS/Kafka/GCP/Redis 五 driver |

---

## 2. 任务二：逐 Broker 特性矩阵

维度图例：队列（do-once + 消费即删）/ pub-sub（fan-out）/ 流式（append-log + 重放 + 分区）。

| Broker | 范式 | 持久化模型 | 运维复杂度 | Go client 质量 | 吞吐量级 | 最适合场景 |
|---|---|---|---|---|---|---|
| **NATS JetStream** | pub-sub + 流 + work-queue | 文件（Raft 复制），durable 消费者 | 🟢 **最低**（单二进制，无 JVM/ZK）| 🟢 官方一等 | 200k–400k msg/s | 中小规模事件总线、Go 原生、边缘/云通吃 |
| **Kafka**（含 Redpanda）| 流为主，pub-sub 次之 | append-log 分区，按 offset 重放 | 🔴 高（JVM+JVM 调优）/ 🟡 Redpanda 降一档 | 🟢 franz-go | 百万 msg/s | 大规模事件溯源、日志聚合、多消费者重放 |
| **Redis Streams** | 队列 + pub-sub | AOF/RDB，PEL 待确认表 | 🟢 低（但 Redis 许可已变 RSALv2/SSPL；**Valkey 是 BSD-3 兜底**）| 🟢 go-redis | 100k msg/s | 已有 Redis 时的小规模队列；**你现在是 PG-only，为此上 Redis 是负债** |
| **RocketMQ** | pub-sub + 流 + 事务消息 | CommitLog + ConsumeQueue，主从同步/异步 | 🟡 中（JVM，但单 broker 模型比 Kafka 轻当量级小）| 🟢 v2 纯 Go 成熟 / 🟡 v5 gRPC 较新 | 百万 msg/s | **中国企业客户环境**；事务消息原生；金融级可靠 |
| **Pulsar** | pub-sub + 流（计算存储分离）| BookKeeper（broker 无状态）+ 分层存储 | 🔴 **最高**（broker + BookKeeper bookies 两套）| 🟡 官方但成熟度次于 Java | 百万+ msg/s | 超大规模、多租户隔离、跨地域复制 |
| **RabbitMQ** | 队列为主，pub-sub（exchange/fanout）| 经典队列内存/磁盘；仲裁队列 Raft | 🟡 中（Erlang，集群调优有学习曲线）| 🟡 amqp091-go | 10k–50k msg/s | 复杂路由（topic/fanout/header exchange）、低延迟 RPC；非重放流 |
| **PG LISTEN/NOTIFY** | pub-sub（fire-and-forget）| **无持久化**（断连丢未投递通知）| 🟢 零（用现有 PG）| ✅ pgx 原生 | 数百–数千/s | 进程内 dispatcher 的可选"跨进程唤醒"传输；**不可承担事件总线全责** |

### 2.1 PG LISTEN/NOTIFY 的硬限制（为什么不能当事件总线）

1. **8000 字节 payload 上限**（`notification_payload`，超出报错）—— 事件 JSON 常超限。
2. **需长连接 + 持久 session** —— 与 PgBouncer **transaction pooling 模式冲突**（LISTEN 必须在 session 级别）。
3. **fire-and-forget，无 ack/无重放** —— 订阅者断连期间的事件永久丢失。
4. **同库 only** —— 不能跨库广播。

**正确定位**：进程内 dispatcher 的"跨进程唤醒"传输（如果未来拆服务）。即：业务写 outbox 行 + `pg_notify(channel, id)`，另一个进程收到通知后 poll outbox 表拉取完整事件。**NOTIFY 只传 id，不传 payload。**

---

## 3. 任务三：本项目决策矩阵

### 3.1 加权评分（本平台权重）

权重依据：self-hosted（30%）+ Go 原生（20%）+ 最小运维（20%）+ 事件量级匹配（10%）+ 中国企业兼容（10%）+ 可重放（10%）。

| Broker | self-hosted 易度（30%）| Go 原生（20%）| 运维轻量（20%）| 量级匹配（10%）| 中国企业（10%）| 可重放（10%）| **加权** |
|---|---|---|---|---|---|---|---|
| **进程内 dispatcher（Phase 1）** | 10 | 10 | 10 | 10 | 5 | 0（outbox 表补）| **8.5** |
| **NATS JetStream** | 10 | 10 | 10 | 10 | 6 | 9 | **9.3** |
| **RocketMQ** | 8 | 8 | 6 | 10 | **10** | 9 | **8.3** |
| **Kafka** | 6 | 9（franz-go）| 4 | 10（过剩）| 8 | 10 | **7.3** |
| **Redpanda** | 9 | 9（franz-go 线协议兼容）| 7 | 10（过剩）| 7 | 10 | **8.5** |
| **Redis Streams** | 8 | 10 | 9 | 10 | 7 | 6 | **8.4**（但**你现在没 Redis，为此引入是负债**）|
| **RabbitMQ** | 9 | 8 | 7 | 10 | 6 | 3（无重放）| **7.4** |
| **Pulsar** | 5 | 7 | 3 | 10（过剩）| 6 | 10 | **6.0** |

**读法**：Phase 1 进程内 dispatcher 是当前最优（8.5，零新依赖）；未来升级时 NATS JetStream（9.3）是全局最优解，Redpanda（8.5）是 Kafka 生态偏好者的备选，RocketMQ（8.3）是中国企业场景的特殊高分项。

### 3.2 关键否决理由（本项目语境）

- **Kafka/Pulsar 直接上**：事件量级 10²–10³/s，用百万级 broker 是杀鸡用牛刀；运维复杂度与本平台"最小依赖"哲学冲突。**仅当 >50–100k evt/s 或真正的事件溯源/跨服务重放需求时重评。**
- **Redis Streams**：你现在 PG-only，**为事件总线专门引入 Redis 是纯负债**（Redis 许可已变 RSALv2/SSPL，Valkey 是 BSD 兜底但仍是新基础设施）。若未来因缓存/会话等正当理由已上 Redis，可重新评估 Streams。
- **RabbitMQ**：pub-sub 能力（fanout exchange）可用但**无重放**，与你"possible replay"需求半冲突；强项（复杂路由）你不需要。
- **Pulsar**：运维最重（broker + BookKeeper 两套），过度工程，否决。

---

## 4. 任务四：架构问题回答

### 4.1 事件总线与 River 统一还是分离？

**分离。** 这是 D39 已确立的核心区分，证据再确认：

| 维度 | 作业队列（River）| 事件总线（EventBus）|
|---|---|---|
| 语义 | **do-once**：消费即删除，重试/死信 | **fan-out**：多订阅者各自独立消费同一事件 |
| 消费模型 | 单 worker 竞争拉取（一个 job 被一个 worker 处理）| 每个订阅者独立进度（topic + subscription）|
| 典型事件 | terraform plan/apply、代码生成、批量漂移检测 | `drift.detected`、`approval.requested/granted/denied`、`migration.started/completed` |
| 重放 | ❌（job 完成就删）| ✅（按 offset/timestamp 重放历史事件）|
| 失败语义 | 重试 + DLQ（作业级）| 订阅者各自 ack/nack（事件级，互不影响）|

**反模式**：用 River 做事件分发。River 的 job 是单消费者模型，做不到"审批通过后，通知服务 A + 服务 B + 审计服务 C 各自反应"。强行用 River 模拟 fan-out 要为每个订阅者各插一条 job，丧失事件总线的单一真相语义。

### 4.2 PG LISTEN/NOTIFY 够不够（Phase 1）？

**够，但只作为进程内 dispatcher 的可选传输层，不承担事件总线全责。** 见 §2.1。Phase 1 推荐：

- **进程内实现**：typed registry + Go channels。同步 publish（注册的 handler 直接在本进程内调用）。
- **可选持久化**：事件先写 `outbox` 表（与业务写同 PG 事务），进程内 dispatcher publish 后标记 `outbox.delivered_at`。这给"重启后重放未投递事件"兜底。
- **LISTEN/NOTIFY 角色**：**Phase 1 单进程时不需要**。若未来拆多进程/多副本，用 NOTIFY 做"跨进程唤醒"（只传 outbox id，不传 payload），收到通知后 poll outbox 表。

### 4.3 什么时候该升级到真正的 broker？

按优先级触发：

| 触发条件 | 升级目标 | 理由 |
|---|---|---|
| **拆成多服务/多副本**，事件需跨进程 fan-out | NATS JetStream | 进程内 dispatcher 无法跨进程；NATS 单二进制最轻 |
| **需要事件重放**（审计/调试/重建投影） | NATS JetStream（durable + replay）| 进程内无历史；outbox 表 poll 是穷版重放但不可扩展 |
| **明确部署到强偏好 RocketMQ 的中国企业客户** | RocketMQ | 客户运维栈一致性、信创合规、阿里云生态 |
| **事件量级 > 50–100k evt/s** 或多服务大规模事件溯源 | Kafka/Redpanda（franz-go）| 真 streaming 语义；Redpanda 免 JVM |
| **需要跨地域复制 / 多租户强隔离** | Pulsar | 计算存储分离 + 原生多租户 |

### 4.4 与 transactional outbox 模式的关系

**outbox 是事件总线的上游，不是替代品。** 正确分层：

```
[业务事务] --原子--> [outbox 表 INSERT]   ← 与业务写同 PG 事务（river.InsertTx 式原子）
                          │
                          ▼
                   [Relay 中继器]          ← poll outbox 或 WAL/CDC log-tail
                          │
                          ▼
                   [EventBus.publish()]    ← 进程内 / NATS / Kafka / RocketMQ
                          │
                ┌─────────┼─────────┐
                ▼         ▼         ▼
            订阅者A    订阅者B    订阅者C
```

- **outbox 保证"业务写与事件产生原子"**（DB 事务内 INSERT，避免"业务成功但事件丢了"）。
- **EventBus 保证"事件被多订阅者消费 + 可重放"**。
- **Relay 中继器**负责把 outbox 行投递到 EventBus。Phase 1 用简单 poll（`SELECT * FROM outbox WHERE delivered_at IS NULL LIMIT 100 FOR UPDATE SKIP LOCKED`）；高吞吐时上 Debezium/Flink CDC 做 WAL log-tailing（更专业但重）。
- **关键**：outbox 与 EventBus 实现解耦 —— 换 EventBus 实现（进程内→NATS→Kafka）**不改 outbox 表与 Relay 中继器的核心逻辑**，只改 `EventBus.publish()` 的底层驱动。这正是可替换 port driver 的价值。

---

## 5. 任务五：最终建议 —— 现在建抽象接口还是现在锁 broker？

### 5.1 结论：**建薄抽象，不锁 broker**

**现在（Phase 1）只做两件事**：

1. **定义极小的 `EventBus` 接口**（port）+ **一个 in-process 实现**（driver）。不一次造 4 个 driver。
2. **配合 outbox 表 + Relay 中继器**，保证业务写与事件原子。

接口只为"未来换实现时不改业务代码"而存在 —— 这是 port-and-adapter（hexagonal）的标准做法，**不是**过度抽象。

### 5.2 接口契约草案（最小可用）

```go
// core/events/eventbus.go  ——  port（接口）
package events

import "context"

// Event 是总线上的一个领域事件。Payload 必须可序列化。
type Event struct {
    ID            string          // UUID，幂等键
    Type          string          // "drift.detected" / "approval.requested" ...
    OccurredAt    time.Time
    Payload       json.RawMessage // 完整事件体（受 PG NOTIFY 8KB 限影响时不走 NOTIFY）
    AggregateID   string          // 聚合根 ID（用于分区/路由）
    CorrelationID string          // 追踪用（与 OTel trace_id 关联）
}

// Handler 是订阅者的处理函数。返回 error 触发重试/DLQ。
type Handler func(ctx context.Context, e Event) error

// EventBus 是事件总线端口（port）。实现可替换：进程内 / NATS / Kafka / RocketMQ。
type EventBus interface {
    // Publish 投递一个事件到指定主题。必须幂等（订阅者按 Event.ID 去重）。
    Publish(ctx context.Context, topic string, e Event) error

    // Subscribe 注册一个订阅者。Phase 1 进程内实现：同步调用 handler。
    // 未来实现：每个 (topic, subscription) 独立 durable 消费者。
    Subscribe(topic string, subscription string, h Handler) (func(), error)
}
```

**Phase 1 唯一实现**（`internal/events/inprocess/bus.go`）：

```go
// inprocess.Bus —— typed registry + 互斥保护的 handler 列表
type Bus struct {
    mu   sync.Mutex
    subs map[string]map[string][]events.Handler // topic -> subscription -> handlers
}

func (b *Bus) Publish(ctx context.Context, topic string, e events.Event) error {
    b.mu.Lock()
    subs := b.subs[topic] // 拷贝快照
    b.mu.Unlock()
    for _, h := range subs {
        if err := h(ctx, e); err != nil {
            // 记日志 + 计数，不中断其他订阅者（fan-out 语义）
            zap.L().Error("event handler failed", ...)
        }
    }
    return nil
}
```

**为什么这样够**：你的订阅者都是本进程内的服务（审批服务、通知服务、审计投影）。进程内调用零网络、零新依赖、零运维。`Subscribe` 的 `subscription` 参数为未来 durable 消费者预留语义（NATS/Kafka 每个 subscription 是独立消费进度）。

### 5.3 成熟先例：looplab/eventhorizon

`looplab/eventhorizon`（~1.7k★，MIT）是 Go 事件溯源库，其 `EventBus` 接口正是这种 port-and-adapter 模式的范本 —— **一个接口，五个 driver**：

- `eventbuslocal`（进程内，默认）
- `eventbusnats`（NATS JetStream）
- `eventbuskafka`（Kafka，用 sarama）
- `eventbusgcp`（Google Cloud Pub/Sub）
- `eventbusredis`（Redis Pub/Sub）

业务代码只依赖 `EventBus` 接口，换 broker 只改 wire 注入的 driver。**这正是 D38（wire DI）天然适配的模式** —— 每个 driver 是一个 `ProviderSet`，`wire.go` 里换一行。

**借鉴点**：接口形状、订阅语义、driver 切换方式。**不必直接依赖 eventhorizon**（它是完整事件溯源库，你只要 EventBus port 的设计思想）。

### 5.4 升级路径（明确触发条件，见 §4.3）

```
Phase 1（现在）：EventBus 接口 + inprocess 实现 + outbox 表 + poll Relay
                          │
                          │ 触发：拆多服务 / 需重放 / 中国企业客户偏好 RocketMQ
                          ▼
Phase 2：新增 nats 实现（eventbusnats.go）或 rocketmq 实现
         wire 注入切换，业务代码零改动
         Relay 从 poll outbox 改为直接 publish 到 broker（outbox 仍保留做事务保证）
                          │
                          │ 触发：>50-100k evt/s / 大规模事件溯源
                          ▼
Phase 3：新增 kafka/redpanda 实现（franz-go）
         同样 wire 切换
```

### 5.5 对 D39 的具体修订建议

D39 现有文本只说"Phase 1 进程内 dispatcher → 后续 NATS"。建议**补充**（不改方向，加深）：

1. **明确接口契约**：定义 `core/events.EventBus` port（§5.2），业务代码只依赖接口。
2. **明确 outbox + Relay**：事件先写 `outbox` 表（与业务写同事务），Relay 中继器投递到 EventBus。这与 River 的 `InsertTx` 原子语义一致，复用同一套"DB 事务内入队"心智。
3. **明确升级触发条件**（§4.3），写进决策文档，避免未来"什么时候上 broker"的争议。
4. **明确 broker 偏好排序**：NATS JetStream（Go 原生、最轻）> RocketMQ（中国企业场景）> Redpanda/Kafka（大规模流场景）。
5. **明确 PG LISTEN/NOTIFY 定位**：仅作未来多进程时的"跨进程唤醒"传输（传 outbox id 不传 payload），不承担事件总线职责。

---

## 6. 结论与可执行的下一步

| 项 | 结论 |
|---|---|
| **D39 方向** | **保持**（进程内 → NATS），证据比初稿更强 |
| **是否锁 broker** | **不锁**（RESERVE）。你的量级所有 broker 都过剩，过早选型是负债 |
| **是否建抽象** | **建薄抽象**：`EventBus` 接口 + inprocess 实现（port-and-adapter，借鉴 looplab/eventhorizon）|
| **PG LISTEN/NOTIFY** | 不当事件总线；仅作未来多进程跨进程唤醒（传 id 不传 payload）|
| **outbox** | 保留，与 River `InsertTx` 同心智，给 Relay 中继器兜底重放 |
| **升级触发** | 拆多服务/需重放 → NATS；中国企业客户 → RocketMQ；大规模 → Kafka/Redpanda（franz-go）|

**未来重评触发条件**：
- 拆成多服务/多副本，事件需跨进程 fan-out → 评估 NATS JetStream
- 明确部署到强偏好 RocketMQ 的中国企业客户 → 评估 RocketMQ
- 事件量级 > 50–100k evt/s 或引入大规模事件溯源 → 评估 Kafka/Redpanda（franz-go）
- 因缓存/会话等正当理由已引入 Redis（非 Valkey）→ 评估 Redis Streams（但需接受 RSALv2/SSPL 许可）

## 7. 参考

**Broker 服务端**
- [apache/kafka](https://github.com/apache/kafka) · [apache/rocketmq](https://github.com/apache/rocketmq) · [apache/pulsar](https://github.com/apache/pulsar)
- [nats-io/nats-server](https://github.com/nats-io/nats-server) · [rabbitmq/rabbitmq-server](https://github.com/rabbitmq/rabbitmq-server) · [redpanda-data/redpanda](https://github.com/redpanda-data/redpanda)
- [redis/go-redis](https://github.com/redis/go-redis) · [valkey-io/valkey](https://github.com/valkey-io/valkey)（Redis 许可变更后的 BSD 兜底）

**Go 客户端**
- [twmb/franz-go](https://github.com/twmb/franz-go)（Kafka 首选，纯 Go）· [IBM/sarama](https://github.com/IBM/sarama)（成熟，迁移中）· [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
- [nats-io/nats.go](https://github.com/nats-io/nats.go)（官方一等）· [rabbitmq/amqp091-go](https://github.com/rabbitmq/amqp091-go) · [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go)
- [apache/rocketmq-client-go](https://github.com/apache/rocketmq-client-go)（v2 Remoting，纯 Go 成熟）· [apache/rocketmq-clients](https://github.com/apache/rocketmq-clients)（v5 gRPC monorepo，较新）

**对比与模式**
- [Kafka is old, Redpanda is fast, Pulsar is weird, NATS is tiny (2025)](https://medium.com/@BuildShift/kafka-is-old-redpanda-is-fast-pulsar-is-weird-nats-is-tiny-which-message-broker-should-you-32ce61d8aa9f)
- [Pulsar vs RabbitMQ vs NATS JetStream — StreamNative](https://streamnative.io/blog/comparison-of-messaging-platforms-apache-pulsar-vs-rabbitmq-vs-nats-jetstream)
- [NATS 官方对比](https://docs.nats.io/nats-concepts/overview/compare-nats)
- [transactional outbox in Golang](https://packagemain.tech/p/how-to-implement-the-outbox-pattern-in-golang)
- [looplab/eventhorizon](https://github.com/looplab/eventhorizon)（EventBus port-and-adapter 先例：Local/NATS/Kafka/GCP/Redis）

**RocketMQ 中国语境**
- [RocketMQ Go SDK 官方文档（5.0 gRPC）](https://rocketmq.apache.org/docs/sdk/05go/)
- [Aliyun ApsaraMQ for RocketMQ Go SDK](https://help.aliyun.com/zh/apsaramq-for-rocketmq/cloud-message-queue-rocketmq-5-x-series/developer-reference/sdk-for-go/)
