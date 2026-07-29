import { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Popconfirm,
  Radio,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { saveStateBackend, saveWorkspace } from '../api'

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

// GitCredentialRow is the Phase 1 localStorage-backed git credential config
// (Fix 6). The api.ts transport interceptor matches git_source URL prefixes
// against urlPrefix to inject the matching token as x-git-token.
interface GitCredentialRow {
  key: string
  urlPrefix: string
  username: string
  token: string
}

// localStorage key for the Phase 1 git credentials list.
const GIT_CREDENTIALS_KEY = 'aether_git_credentials'

function loadGitCredentials(): GitCredentialRow[] {
  try {
    const raw = localStorage.getItem(GIT_CREDENTIALS_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed
  } catch {
    return []
  }
}

function persistGitCredentials(rows: GitCredentialRow[]) {
  localStorage.setItem(GIT_CREDENTIALS_KEY, JSON.stringify(rows))
}

export default function AdminPage() {
  const [provider, setProvider] = useState<ProviderOption>('alicloud')
  const [executorMode, setExecutorMode] = useState<ExecutorMode>('process')
  const [worktreePath, setWorktreePath] = useState('/var/lib/aether/worktrees')
  // infra-repo: global workspace row (workspaces table) — codegen output target
  // (D4 workspace). The platform clones this repo and creates a worktree per
  // request, committing generated HCL back to the default branch.
  const [giteaUrl, setGiteaUrl] = useState('http://192.168.31.33:3180/aether/aether-infra.git')
  const [infraDefaultBranch, setInfraDefaultBranch] = useState('main')
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
  const [savingBackend, setSavingBackend] = useState(false)
  const [savingWorkspace, setSavingWorkspace] = useState(false)
  // state backend form fields mirrored into component state so save can read
  // them without re-querying the antd Form.
  const [backendBucket, setBackendBucket] = useState('tm-state')
  const [backendRegion, setBackendRegion] = useState('')
  const [backendEndpoint, setBackendEndpoint] = useState('http://192.168.31.33:9900')
  const [backends, setBackends] = useState<StateBackendRow[]>(defaultBackends)
  const [teams, setTeams] = useState<TeamRow[]>(seedTeams)
  const [backendForm] = Form.useForm<NewBackendForm>()
  const [teamForm] = Form.useForm<NewTeamForm>()
  // Fix 6: git credentials stored in localStorage; the api.ts transport
  // interceptor matches git_source URL prefixes to inject x-git-token.
  const [gitCreds, setGitCreds] = useState<GitCredentialRow[]>([])
  const [credForm] = Form.useForm<GitCredentialRow>()

  useEffect(() => {
    setGitCreds(loadGitCredentials())
  }, [])

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

  const gitCredColumns: ColumnsType<GitCredentialRow> = [
    { title: 'URL 前缀', dataIndex: 'urlPrefix', key: 'urlPrefix' },
    { title: '用户名', dataIndex: 'username', key: 'username' },
    {
      title: '令牌/密码',
      dataIndex: 'token',
      key: 'token',
      render: () => <Tag>已配置</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Popconfirm
          title="删除该凭证规则？"
          onConfirm={() => handleDeleteGitCred(record.key)}
          okText="删除"
          cancelText="取消"
        >
          <Button type="link" danger>删除</Button>
        </Popconfirm>
      ),
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

  // Fix 1: save the default state backend credentials + bucket config to the DB
  // via POST /admin/state-backends. Phase 1 also re-validates fields non-empty.
  const handleSaveBackend = async () => {
    if (!accessKey || !secretKey) {
      message.warning('请先填写 Access Key 与 Secret Key')
      return
    }
    if (!backendBucket) {
      message.warning('请填写 Bucket')
      return
    }
    setSavingBackend(true)
    try {
      await saveStateBackend({
        name: 'default',
        kind: 's3',
        bucket: backendBucket,
        region: backendRegion,
        endpoint: backendEndpoint,
        accessKey,
        secretKey,
      })
      message.success('State 后端配置已保存到数据库')
    } catch (err: any) {
      message.error(`保存失败：${err?.message ?? err}`)
    } finally {
      setSavingBackend(false)
    }
  }

  // P0-4: test the MinIO/S3 connection. Phase 1 just validates the fields are
  // present and reports success — real bucket reachability lands in Phase 2.
  const handleTestConnection = async () => {
    if (!accessKey || !secretKey) {
      message.warning('请先填写 Access Key 与 Secret Key')
      return
    }
    if (!backendBucket) {
      message.warning('请填写 Bucket')
      return
    }
    setTesting(true)
    // Simulate a quick connectivity check; Phase 2 will hit a real health RPC.
    setTimeout(() => {
      setTesting(false)
      message.success(`连接测试通过（Phase 1 仅校验输入；Phase 2 将真实访问 MinIO/S3）`)
    }, 500)
  }

  // Fix 2: save the global infra-repo workspace (remote_url + default_branch)
  // to the DB via POST /admin/workspaces (id=1 row upsert).
  const handleSaveWorkspace = async () => {
    if (!giteaUrl) {
      message.warning('请填写 Gitea URL')
      return
    }
    setSavingWorkspace(true)
    try {
      await saveWorkspace({ remoteUrl: giteaUrl, defaultBranch: infraDefaultBranch || 'main' })
      message.success('执行仓库配置已保存到数据库')
    } catch (err: any) {
      message.error(`保存失败：${err?.message ?? err}`)
    } finally {
      setSavingWorkspace(false)
    }
  }

  // Fix 6: add a git credential row (localStorage in Phase 1). The api.ts
  // transport interceptor matches git_source URL prefixes to inject x-git-token.
  const handleAddGitCred = async () => {
    try {
      const values = await credForm.validateFields()
      const row: GitCredentialRow = {
        key: String(Date.now()),
        urlPrefix: values.urlPrefix,
        username: values.username || '',
        token: values.token,
      }
      const next = [...gitCreds, row]
      setGitCreds(next)
      persistGitCredentials(next)
      message.success(`已添加凭证规则 ${values.urlPrefix}（Phase 1 存于本地，Phase 2 落库）`)
      credForm.resetFields()
    } catch {
      // validation handled by form
    }
  }

  const handleDeleteGitCred = (key: string) => {
    const next = gitCreds.filter((r) => r.key !== key)
    setGitCreds(next)
    persistGitCredentials(next)
    message.success('已删除凭证规则')
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
              MinIO / S3 访问凭证。平台用此凭证对默认 state 后端进行 terraform state 读写。点击保存将凭证与 bucket 配置写入数据库（state_backends 默认行；Phase 1 不持久化明文 AK/SK，Phase 2 接入 Vault/KMS）。
            </Paragraph>
            <Form layout="vertical" style={{ maxWidth: 640 }}>
              <Space direction="vertical" size="small" style={{ width: '100%' }}>
                <Form.Item label="Bucket" required>
                  <Input
                    placeholder="tm-state"
                    value={backendBucket}
                    onChange={(e) => setBackendBucket(e.target.value)}
                    style={{ width: 320 }}
                  />
                </Form.Item>
                <Form.Item label="Region">
                  <Input
                    placeholder="cn-hangzhou（可空）"
                    value={backendRegion}
                    onChange={(e) => setBackendRegion(e.target.value)}
                    style={{ width: 320 }}
                  />
                </Form.Item>
                <Form.Item label="Endpoint">
                  <Input
                    placeholder="http://192.168.31.33:9900"
                    value={backendEndpoint}
                    onChange={(e) => setBackendEndpoint(e.target.value)}
                    style={{ width: 360 }}
                  />
                </Form.Item>
                <Form.Item label="Access Key" required>
                  <Input.Password
                    placeholder="minio-access-key"
                    value={accessKey}
                    onChange={(e) => setAccessKey(e.target.value)}
                    style={{ width: 320 }}
                  />
                </Form.Item>
                <Form.Item label="Secret Key" required>
                  <Input.Password
                    placeholder="minio-secret-key"
                    value={secretKey}
                    onChange={(e) => setSecretKey(e.target.value)}
                    style={{ width: 360 }}
                  />
                </Form.Item>
              </Space>
            </Form>
            <Space style={{ marginTop: 12 }}>
              <Button type="primary" loading={savingBackend} onClick={handleSaveBackend}>
                保存
              </Button>
              <Button loading={testing} onClick={handleTestConnection}>
                测试连接
              </Button>
            </Space>
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
                <Input
                  value={infraDefaultBranch}
                  onChange={(e) => setInfraDefaultBranch(e.target.value)}
                  style={{ width: 200 }}
                />
              </Form.Item>
              <Form.Item label="访问凭证">
                <Input.Password placeholder="通过环境变量 AETHER_GITEA_TOKEN 注入" disabled />
              </Form.Item>
            </Form>
            <Paragraph type="secondary" style={{ marginTop: 8 }}>
              执行仓库是 codegen 产物的提交目标（D4 workspace）。平台用 go-git 操作这个仓库。
            </Paragraph>
            <Button type="primary" loading={savingWorkspace} onClick={handleSaveWorkspace} style={{ marginTop: 8 }}>
              保存
            </Button>
          </Card>
        </Space>
      ),
    },
    {
      key: 'git-credentials',
      label: 'Git 凭证',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="Git 访问凭证"
            description="配置 URL 前缀匹配规则，注册私有仓库模块时前端会按 git_source 匹配并注入 x-git-token 请求头。Phase 1 存储于浏览器 localStorage，Phase 2 落库到 credentials 表。"
          />
          <Table<GitCredentialRow>
            rowKey="key"
            columns={gitCredColumns}
            dataSource={gitCreds}
            pagination={false}
            locale={{ emptyText: '暂无凭证规则' }}
          />
          <Card title="添加凭证规则" size="small">
            <Form form={credForm} layout="inline" style={{ rowGap: 12 }}>
              <Form.Item
                label="URL 前缀"
                name="urlPrefix"
                rules={[{ required: true, message: '请填写 URL 前缀' }]}
              >
                <Input placeholder="http://192.168.31.33:3180/*" style={{ width: 280 }} />
              </Form.Item>
              <Form.Item label="用户名" name="username">
                <Input placeholder="（可选）" style={{ width: 160 }} />
              </Form.Item>
              <Form.Item
                label="令牌/密码"
                name="token"
                rules={[{ required: true, message: '请填写令牌' }]}
              >
                <Input.Password placeholder="git access token" style={{ width: 220 }} />
              </Form.Item>
              <Form.Item>
                <Button type="primary" onClick={handleAddGitCred}>添加</Button>
              </Form.Item>
            </Form>
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
