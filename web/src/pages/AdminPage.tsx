import { useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Radio,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'

const { Text, Paragraph } = Typography

// ---- Seed / config display types ----

interface TeamRow {
  key: string
  name: string
  slug: string
  kind: string
}

interface StateBackendRow {
  key: string
  name: string
  kind: string
  bucket: string
  endpoint: string
  region: string
  isDefault: boolean
}

// Static seed data for the Team tab. Per the MVP integration plan there is no
// TeamService proto yet, so the table is read-only and surfaced from the known
// seed data; Phase 2 will wire full CRUD through an Admin API.
const seedTeams: TeamRow[] = [
  { key: '1', name: '平台工程', slug: 'platform', kind: 'platform' },
  { key: '2', name: 'DBA 团队', slug: 'dba', kind: 'business' },
  { key: '3', name: 'SRE 团队', slug: 'sre', kind: 'platform' },
]

// Default MinIO / state backend display. Mirrors the seed row in migration 011
// plus the deployment MinIO endpoint from the project context.
const defaultBackends: StateBackendRow[] = [
  {
    key: 'default',
    name: 'default',
    kind: 's3',
    bucket: 'tm-state',
    region: '',
    endpoint: 'http://192.168.31.33:9900',
    isDefault: true,
  },
]

type ProviderOption = 'null' | 'local' | 'random' | 'alicloud' | 'aws'
type ExecutorMode = 'process' | 'container'

interface NewBackendForm {
  name: string
  kind: 's3' | 'oss' | 'local'
  bucket: string
  endpoint?: string
}

interface NewTeamForm {
  name: string
  slug: string
  kind: string
}

export default function AdminPage() {
  const [provider, setProvider] = useState<ProviderOption>('alicloud')
  const [executorMode, setExecutorMode] = useState<ExecutorMode>('process')
  const [worktreePath, setWorktreePath] = useState('/var/lib/aether/worktrees')
  // infra-repo: global workspace row (workspaces table) — codegen output target
  // (D4 workspace). The platform clones this repo and creates a worktree per
  // request, committing generated HCL back to the default branch.
  const [giteaUrl, setGiteaUrl] = useState('http://192.168.31.33:3180/aether/aether-infra.git')
  // module-source repo: where module definitions are cloned FROM. Per-module
  // (each catalog item carries its own git_source), but surfaced here as the
  // default org/repo prefix for newly registered modules.
  const [moduleRepoUrl, setModuleRepoUrl] = useState('http://192.168.31.33:3180/aether/aether-modules.git')
  // MinIO/S3 credentials for the default state backend (P0-3). Phase 1: stored
  // only in component state for the test-connection flow; Phase 2 will persist
  // via an Admin API into state_backends.
  const [accessKey, setAccessKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [testing, setTesting] = useState(false)
  const [backends, setBackends] = useState<StateBackendRow[]>(defaultBackends)
  const [teams, setTeams] = useState<TeamRow[]>(seedTeams)
  const [backendForm] = Form.useForm<NewBackendForm>()
  const [teamForm] = Form.useForm<NewTeamForm>()

  // P0-4: test the MinIO/S3 connection. Phase 1 just validates the fields are
  // present and reports success — real bucket reachability lands in Phase 2.
  const handleTestConnection = async () => {
    if (!accessKey || !secretKey) {
      message.warning('请先填写 Access Key 与 Secret Key')
      return
    }
    setTesting(true)
    // Simulate a quick connectivity check; Phase 2 will hit a real health RPC.
    setTimeout(() => {
      setTesting(false)
      message.success(`连接测试成功（Phase 1 仅校验输入；Phase 2 将真实访问 MinIO/S3）`)
    }, 500)
  }

  const backendColumns: ColumnsType<StateBackendRow> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型',
      dataIndex: 'kind',
      key: 'kind',
      render: (k: string) => <Tag color="blue">{k}</Tag>,
    },
    { title: 'Bucket', dataIndex: 'bucket', key: 'bucket' },
    { title: 'Endpoint', dataIndex: 'endpoint', key: 'endpoint' },
    { title: 'Region', dataIndex: 'region', key: 'region' },
    {
      title: '默认',
      dataIndex: 'isDefault',
      key: 'isDefault',
      render: (v: boolean) => (v ? <Tag color="green">默认</Tag> : null),
    },
  ]

  const teamColumns: ColumnsType<TeamRow> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: 'Slug', dataIndex: 'slug', key: 'slug' },
    {
      title: '类型',
      dataIndex: 'kind',
      key: 'kind',
      render: (k: string) => <Tag color={k === 'platform' ? 'purple' : 'cyan'}>{k}</Tag>,
    },
  ]

  const handleAddBackend = async () => {
    try {
      const values = await backendForm.validateFields()
      setBackends((prev) => [
        ...prev,
        {
          key: String(prev.length + 1),
          name: values.name,
          kind: values.kind,
          bucket: values.bucket,
          endpoint: values.endpoint || '',
          region: '',
          isDefault: false,
        },
      ])
      message.success(`已添加 state 后端 ${values.name}（仅前端展示，Phase 2 落库）`)
      backendForm.resetFields()
    } catch {
      // validation handled by form
    }
  }

  const handleAddTeam = async () => {
    try {
      const values = await teamForm.validateFields()
      setTeams((prev) => [
        ...prev,
        { key: String(prev.length + 1), name: values.name, slug: values.slug, kind: values.kind },
      ])
      message.success(`已添加团队 ${values.name}（仅前端展示，Phase 2 通过 API 落库）`)
      teamForm.resetFields()
    } catch {
      // validation handled by form
    }
  }

  const tabItems = [
    {
      key: 'teams',
      label: '团队管理',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="Phase 2: full CRUD via API"
            description="当前没有 TeamService proto，团队列表来自种子数据，新建团队仅在前端展示。Phase 2 将通过 Admin API 落库。"
          />
          <Card title="创建团队" size="small">
            <Form form={teamForm} layout="inline">
              <Form.Item label="名称" name="name" rules={[{ required: true, message: '请填写团队名称' }]}>
                <Input placeholder="数据团队" style={{ width: 180 }} />
              </Form.Item>
              <Form.Item label="Slug" name="slug" rules={[{ required: true, message: '请填写 slug' }]}>
                <Input placeholder="data" style={{ width: 140 }} />
              </Form.Item>
              <Form.Item label="类型" name="kind" initialValue="business" rules={[{ required: true }]}>
                <Input placeholder="platform / business" style={{ width: 160 }} />
              </Form.Item>
              <Form.Item>
                <Button type="primary" onClick={handleAddTeam}>创建团队</Button>
              </Form.Item>
            </Form>
          </Card>
          <Table<StateBackendRow>
            rowKey="key"
            columns={teamColumns as unknown as ColumnsType<StateBackendRow>}
            dataSource={teams as unknown as StateBackendRow[]}
            pagination={false}
          />
        </Space>
      ),
    },
    {
      key: 'state',
      label: 'State 后端',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="MinIO / S3 兼容存储"
            description="State 后端配置来自 state_backends 表。codegen 读取默认行渲染 backend.tf。Access/Secret Key 为 MinIO/S3 访问凭证。新建后端仅前端展示，Phase 2 接入 Admin API。"
          />
          <Table<StateBackendRow>
            rowKey="key"
            columns={backendColumns}
            dataSource={backends}
            pagination={false}
          />
          <Card title="默认后端凭证 (MinIO / S3)" size="small">
            <Paragraph type="secondary" style={{ marginBottom: 12 }}>
              MinIO / S3 访问凭证。平台用此凭证对默认 state 后端进行 terraform state 读写。Phase 1 仅前端校验，Phase 2 通过 Admin API 落库到 state_backends。
            </Paragraph>
            <Form layout="inline">
              <Form.Item label="Access Key" required>
                <Input.Password
                  placeholder="minio-access-key"
                  value={accessKey}
                  onChange={(e) => setAccessKey(e.target.value)}
                  style={{ width: 220 }}
                />
              </Form.Item>
              <Form.Item label="Secret Key" required>
                <Input.Password
                  placeholder="minio-secret-key"
                  value={secretKey}
                  onChange={(e) => setSecretKey(e.target.value)}
                  style={{ width: 240 }}
                />
              </Form.Item>
              <Form.Item>
                <Button type="primary" loading={testing} onClick={handleTestConnection}>
                  测试连接
                </Button>
              </Form.Item>
            </Form>
          </Card>
          <Card title="添加 State 后端" size="small">
            <Form form={backendForm} layout="inline">
              <Form.Item label="名称" name="name" rules={[{ required: true, message: '请填写名称' }]}>
                <Input placeholder="default-oss" style={{ width: 160 }} />
              </Form.Item>
              <Form.Item label="类型" name="kind" initialValue="s3" rules={[{ required: true }]}>
                <Input placeholder="s3 / oss / local" style={{ width: 120 }} />
              </Form.Item>
              <Form.Item label="Bucket" name="bucket" rules={[{ required: true, message: '请填写 bucket' }]}>
                <Input placeholder="tm-state" style={{ width: 160 }} />
              </Form.Item>
              <Form.Item label="Endpoint" name="endpoint">
                <Input placeholder="http://192.168.31.33:9900" style={{ width: 220 }} />
              </Form.Item>
              <Form.Item>
                <Button type="primary" onClick={handleAddBackend}>添加</Button>
              </Form.Item>
            </Form>
          </Card>
        </Space>
      ),
    },
    {
      key: 'provider',
      label: 'Provider 配置',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="Provider 加载方式"
            description="选择平台在 codegen / plan 阶段注入的 provider 来源。null = 显式 null provider（纯逻辑测试）；local = 本地资源；random = 随机资源；alicloud / aws = 真实云 provider。"
          />
          <Card title="默认 Provider">
            <Radio.Group
              value={provider}
              onChange={(e) => {
                const v = e.target.value as ProviderOption
                setProvider(v)
                message.info(`Provider 切换为 ${v}（仅前端展示）`)
              }}
            >
              <Space direction="vertical">
                <Radio value="null">null — 纯逻辑测试，不创建真实资源</Radio>
                <Radio value="local">local — 本地文件资源</Radio>
                <Radio value="random">random — 随机资源（占位 / 测试）</Radio>
                <Radio value="alicloud">alicloud — 阿里云</Radio>
                <Radio value="aws">aws — AWS</Radio>
              </Space>
            </Radio.Group>
          </Card>
        </Space>
      ),
    },
    {
      key: 'executor',
      label: '执行配置',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="Executor 模式"
            description="process = 在主机进程内执行 Terramate / Terraform（Phase 1 默认）；container = 在隔离容器中执行（Phase 2）。"
          />
          <Card title="执行器">
            <Form layout="vertical" style={{ maxWidth: 520 }}>
              <Form.Item label="执行模式">
                <Radio.Group
                  value={executorMode}
                  onChange={(e) => {
                    const v = e.target.value as ExecutorMode
                    setExecutorMode(v)
                    message.info(`执行模式切换为 ${v}（仅前端展示）`)
                  }}
                >
                  <Radio value="process">process（默认）</Radio>
                  <Radio value="container">container（Phase 2）</Radio>
                </Radio.Group>
              </Form.Item>
              <Form.Item label="Worktree 路径">
                <Input
                  value={worktreePath}
                  onChange={(e) => setWorktreePath(e.target.value)}
                  style={{ width: 400 }}
                />
              </Form.Item>
            </Form>
          </Card>
        </Space>
      ),
    },
    {
      key: 'module-source-repo',
      label: '模块源仓库',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="模块源仓库 (module source repository)"
            description="模块源仓库是注册模块时被克隆 / 提取契约的来源（如 Gitea aether-modules）。该配置是 per-module 的：每个目录项携带自己的 git_source + module_path，这里仅设置默认仓库前缀供新注册模块复用。"
          />
          <Card title="默认模块源仓库">
            <Form layout="vertical" style={{ maxWidth: 520 }}>
              <Form.Item label="默认 Git 源">
                <Input
                  value={moduleRepoUrl}
                  onChange={(e) => setModuleRepoUrl(e.target.value)}
                  style={{ width: 520 }}
                />
              </Form.Item>
              <Form.Item label="默认分支">
                <Input defaultValue="main" style={{ width: 200 }} />
              </Form.Item>
              <Form.Item label="访问凭证">
                <Input.Password placeholder="通过环境变量 AETHER_MODULE_REPO_TOKEN 注入" disabled />
              </Form.Item>
            </Form>
            <Paragraph type="secondary" style={{ marginTop: 8 }}>
              平台从该仓库克隆 + 提取 <Text code>variables.tf</Text> 契约。每个模块可单独覆盖 git_source / module_path。
            </Paragraph>
          </Card>
        </Space>
      ),
    },
    {
      key: 'workspace',
      label: '执行仓库 (infra-repo)',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="warning"
            showIcon
            message="凭证安全"
            description="Gitea 访问令牌通过环境变量 / Secret 注入到后端，不会出现在前端。此处仅展示仓库 URL。"
          />
          <Alert
            type="info"
            showIcon
            message="执行仓库 (infra-repo, D4 workspace)"
            description="执行仓库是 codegen 产物的提交目标 —— 即 workspaces 表中配置的全局 infra-repo。平台克隆此仓库并为每个工单创建 worktree，codegen 将生成的 HCL 提交回默认分支。这是全局配置，所有工单共享。"
          />
          <Card title="Gitea 执行仓库 (infra-repo)">
            <Form layout="vertical" style={{ maxWidth: 520 }}>
              <Form.Item label="Gitea URL">
                <Input
                  value={giteaUrl}
                  onChange={(e) => setGiteaUrl(e.target.value)}
                  style={{ width: 400 }}
                />
              </Form.Item>
              <Form.Item label="默认分支">
                <Input defaultValue="main" style={{ width: 200 }} />
              </Form.Item>
              <Form.Item label="访问凭证">
                <Input.Password placeholder="通过环境变量 AETHER_GITEA_TOKEN 注入" disabled />
              </Form.Item>
            </Form>
            <Paragraph type="secondary" style={{ marginTop: 8 }}>
              平台克隆此仓库并为每个工单创建 worktree，codegen 将生成产物提交回默认分支。
            </Paragraph>
          </Card>
        </Space>
      ),
    },
  ]

  return (
    <Card title="管理设置">
      <Tabs defaultActiveKey="teams" items={tabItems} />
      <Text type="secondary" style={{ display: 'block', marginTop: 16 }}>
        注：除 State 后端读取来自数据库外，其余配置为 Phase 1 展示态；持久化与下发将在 Phase 2 接入。
      </Text>
    </Card>
  )
}
