// Phase 1 mock API. Returns the same data shape as the HTML prototype.
// Real Connect-ES integration will replace these with generated client calls.

export type CatalogCategory = 'database' | 'compute' | 'network' | 'storage' | 'middleware'
export type CatalogLayer = 'global' | 'middleware' | 'application'
export type CatalogStatus = 'published' | 'draft' | 'deprecated'

export interface CatalogItem {
  id: string
  name: string
  category: CatalogCategory
  layer: CatalogLayer
  owner: string
  status: CatalogStatus
  pathTemplate?: string
  description?: string
}

export type RequestStatus =
  | 'draft'
  | 'code_generated'
  | 'plan_ready'
  | 'pending_approval'
  | 'applying'
  | 'completed'
  | 'failed'

export type Env = 'dev' | 'staging' | 'prod'

export interface IaCRequest {
  id: string
  catalogItem: string
  env: Env
  status: RequestStatus
  team: string
  createdAt: string
}

export interface ApprovalItem extends IaCRequest {
  approver?: string
  reason?: string
}

export type ModuleStatus = 'ready' | 'extracting' | 'failed'

export interface Module {
  id: string
  name: string
  gitSource: string
  modulePath: string
  version: string
  provider: string
  varCount: number
  outputCount: number
  status: ModuleStatus
  contractJson?: string
}

export interface RequestStep {
  title: string
  status: 'finish' | 'process' | 'wait' | 'error'
  description?: string
}

export interface TimelineEntry {
  color: string
  label: string
  time: string
}

export interface RequestDetail {
  id: string
  catalogItem: string
  env: Env
  team: string
  status: RequestStatus
  steps: RequestStep[]
  timeline: TimelineEntry[]
  planDiff: string
}

export type ApprovalDecision = 'approve' | 'reject'

// ---- Mock data ----------------------------------------------------------

const catalogItems: CatalogItem[] = [
  { id: 'ci-001', name: 'MySQL RDS 实例', category: 'database', layer: 'application', owner: 'DBA 团队', status: 'published', pathTemplate: '{team}/{env}/db/{instance_name}', description: '标准 MySQL 托管实例' },
  { id: 'ci-002', name: 'PostgreSQL RDS', category: 'database', layer: 'application', owner: 'DBA 团队', status: 'published', pathTemplate: '{team}/{env}/db/{instance_name}' },
  { id: 'ci-003', name: 'Redis 缓存集群', category: 'database', layer: 'middleware', owner: '中间件团队', status: 'published', pathTemplate: '{env}/middleware/redis/{cluster_name}' },
  { id: 'ci-004', name: 'ECS 计算实例', category: 'compute', layer: 'application', owner: 'SRE 团队', status: 'published', pathTemplate: '{team}/{env}/compute/{instance_name}' },
  { id: 'ci-005', name: 'Kubernetes 集群', category: 'compute', layer: 'application', owner: 'SRE 团队', status: 'published', pathTemplate: '{team}/{env}/compute/{cluster_name}' },
  { id: 'ci-006', name: 'VPC 专有网络', category: 'network', layer: 'global', owner: '网络团队', status: 'published', pathTemplate: '{env}/global/vpc/{vpc_name}' },
  { id: 'ci-007', name: '负载均衡 SLB', category: 'network', layer: 'global', owner: '网络团队', status: 'published', pathTemplate: '{env}/global/slb/{slb_name}' },
  { id: 'ci-008', name: 'NAT 网关', category: 'network', layer: 'global', owner: '网络团队', status: 'published', pathTemplate: '{env}/global/nat/{nat_name}' },
  { id: 'ci-009', name: 'OSS 对象存储', category: 'storage', layer: 'global', owner: '存储团队', status: 'published', pathTemplate: '{env}/global/oss/{bucket_name}' },
  { id: 'ci-010', name: 'Kafka 消息队列', category: 'middleware', layer: 'middleware', owner: '中间件团队', status: 'draft', pathTemplate: '{env}/middleware/kafka/{cluster_name}' },
  { id: 'ci-011', name: 'RabbitMQ', category: 'middleware', layer: 'middleware', owner: '中间件团队', status: 'published', pathTemplate: '{env}/middleware/rabbitmq/{cluster_name}' },
  { id: 'ci-012', name: 'MongoDB', category: 'database', layer: 'application', owner: 'DBA 团队', status: 'deprecated', pathTemplate: '{team}/{env}/db/{instance_name}' },
]

const requests: IaCRequest[] = [
  { id: 'REQ-2026-0001', catalogItem: 'MySQL RDS 实例', env: 'prod', status: 'pending_approval', team: 'payment', createdAt: '2026-07-27 10:23' },
  { id: 'REQ-2026-0002', catalogItem: 'ECS 计算实例', env: 'dev', status: 'completed', team: 'search', createdAt: '2026-07-26 14:05' },
  { id: 'REQ-2026-0003', catalogItem: 'Redis 缓存集群', env: 'staging', status: 'plan_ready', team: 'growth', createdAt: '2026-07-27 09:11' },
  { id: 'REQ-2026-0004', catalogItem: 'VPC 专有网络', env: 'prod', status: 'applying', team: 'infra', createdAt: '2026-07-28 08:30' },
  { id: 'REQ-2026-0005', catalogItem: 'Kafka 消息队列', env: 'dev', status: 'pending_approval', team: 'messaging', createdAt: '2026-07-28 09:45' },
  { id: 'REQ-2026-0006', catalogItem: 'OSS 对象存储', env: 'staging', status: 'code_generated', team: 'data', createdAt: '2026-07-28 10:02' },
  { id: 'REQ-2026-0007', catalogItem: '负载均衡 SLB', env: 'prod', status: 'failed', team: 'infra', createdAt: '2026-07-25 16:40' },
  { id: 'REQ-2026-0008', catalogItem: 'PostgreSQL RDS', env: 'prod', status: 'pending_approval', team: 'risk', createdAt: '2026-07-28 11:15' },
]

const modules: Module[] = [
  {
    id: 'mod-001',
    name: 'rds-mysql',
    gitSource: 'git@github.com:org/tf-modules.git',
    modulePath: 'modules/rds/mysql',
    version: 'v1.2.0',
    provider: 'alicloud',
    varCount: 8,
    outputCount: 5,
    status: 'ready',
    contractJson: JSON.stringify(
      {
        instance_name: { type: 'string', required: true, description: '实例名称' },
        instance_type: { type: 'string', required: true, description: '实例规格', default: 'rds.mysql.s2.large' },
        storage_size: { type: 'number', required: false, description: '存储大小 GB', default: 100 },
        ha_enabled: { type: 'bool', required: false, description: '是否高可用', default: false },
        vswitch_id: { type: 'string', required: false, description: '交换机 ID（平台推断）', sensitive: false },
        secret_key: { type: 'string', required: false, description: '密钥', sensitive: true },
      },
      null,
      2,
    ),
  },
  {
    id: 'mod-002',
    name: 'ecs-instance',
    gitSource: 'git@github.com:org/tf-modules.git',
    modulePath: 'modules/ecs/instance',
    version: 'v2.0.1',
    provider: 'alicloud',
    varCount: 12,
    outputCount: 7,
    status: 'ready',
    contractJson: JSON.stringify(
      {
        instance_name: { type: 'string', required: true, description: '实例名称' },
        instance_type: { type: 'string', required: true, description: '实例规格', default: 'ecs.g6.large' },
        image_id: { type: 'string', required: false, description: '镜像 ID', default: 'centos_7_9' },
      },
      null,
      2,
    ),
  },
  {
    id: 'mod-003',
    name: 'vpc-core',
    gitSource: 'git@github.com:org/tf-modules.git',
    modulePath: 'modules/network/vpc',
    version: 'v1.0.0',
    provider: 'alicloud',
    varCount: 6,
    outputCount: 4,
    status: 'extracting',
  },
]

// ---- Mock fetch helpers (simulate async) -------------------------------

function delay<T>(value: T, ms = 120): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ms))
}

export function fetchCatalogItems(): Promise<CatalogItem[]> {
  return delay(catalogItems)
}

export function fetchRequests(): Promise<IaCRequest[]> {
  return delay(requests)
}

export function fetchPendingApprovals(): Promise<ApprovalItem[]> {
  return delay(
    requests
      .filter((r) => r.status === 'pending_approval')
      .map((r) => ({ ...r, approver: 'admin@aether.io' })),
  )
}

export function fetchModules(): Promise<Module[]> {
  return delay(modules)
}

export function fetchRequestDetail(id: string): Promise<RequestDetail> {
  const req = requests.find((r) => r.id === id) ?? requests[0]
  return delay({
    id: req.id,
    catalogItem: req.catalogItem,
    env: req.env,
    team: req.team,
    status: req.status,
    steps: buildSteps(req.status),
    timeline: [
      { color: 'green', label: '工单提交', time: '2026-07-27 10:23' },
      { color: 'green', label: '代码生成完成', time: '2026-07-27 10:24' },
      { color: 'blue', label: 'Terraform Plan 完成', time: '2026-07-27 10:25' },
      { color: 'orange', label: '等待审批', time: '2026-07-27 10:26' },
    ],
    planDiff: [
      'Terraform will perform the following actions:',
      '',
      '  # module.rds_mysql.alicloud_db_instance.this will be created',
      '  + resource "alicloud_db_instance" "this" {',
      '      + engine               = "mysql"',
      '      + instance_type        = "rds.mysql.s2.large"',
      '      + instance_storage     = 100',
      '      + instance_name        = "payment-prod-mysql-001"',
      '      + vswitch_id           = (known after apply)',
      '    }',
      '',
      'Plan: 1 to add, 0 to change, 0 to destroy.',
    ].join('\n'),
  })
}

export function decideApproval(id: string, decision: ApprovalDecision): Promise<{ id: string; decision: ApprovalDecision; success: boolean }> {
  return delay({ id, decision, success: true })
}

function buildSteps(status: RequestStatus): RequestStep[] {
  const all: RequestStep['title'][] = ['提交', '代码生成', 'Plan', '审批', 'Apply', '完成']
  const statusIndex: Record<RequestStatus, number> = {
    draft: 0,
    code_generated: 1,
    plan_ready: 2,
    pending_approval: 3,
    applying: 4,
    completed: 5,
    failed: 4,
  }
  const current = statusIndex[status]
  return all.map((title, idx) => {
    if (status === 'failed' && idx === current) {
      return { title, status: 'error' as const, description: '执行失败' }
    }
    if (idx < current) return { title, status: 'finish' as const }
    if (idx === current) return { title, status: 'process' as const }
    return { title, status: 'wait' as const }
  })
}
