import { useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Collapse,
  Empty,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  fetchModules,
  publishCatalogItem,
  registerModule,
  resolveGitTokenForURL,
  type CatalogLayer,
  type Module,
  type ModuleStatus,
} from '../api'

const { Text } = Typography

const statusColor: Partial<Record<ModuleStatus, string>> = {
  ready: 'green',
  validated: 'green',
  extracting: 'blue',
  pending_validation: 'blue',
  failed: 'red',
  validation_failed: 'red',
  deprecated: 'default',
}
const statusLabel: Partial<Record<ModuleStatus, string>> = {
  ready: '可用',
  validated: '已验证',
  extracting: '契约提取中',
  pending_validation: '待验证',
  failed: '失败',
  validation_failed: '验证失败',
  deprecated: '已废弃',
}

interface RegisterForm {
  gitSource: string
  modulePath: string
  version: string
  provider: string
  displayName: string
  team: string
  // gitToken: optional Git access token (Phase 1). When empty, the register
  // flow auto-resolves a token from the AdminPage Git 凭证 rules by URL prefix.
  gitToken?: string
}

interface PublishForm {
  displayName: string
  category: string
  layer: CatalogLayer
  ownerTeamId: string
}

const layerOptions: { value: CatalogLayer; label: string }[] = [
  { value: 'global', label: '全局层 (global)' },
  { value: 'middleware', label: '中间件层 (middleware)' },
  { value: 'application', label: '应用层 (application)' },
]

const categoryOptions = [
  { value: 'database', label: '数据库 (database)' },
  { value: 'compute', label: '计算 (compute)' },
  { value: 'network', label: '网络 (network)' },
  { value: 'storage', label: '存储 (storage)' },
  { value: 'middleware', label: '中间件 (middleware)' },
]

// P0-1: Phase 1 hardcoded team list (no TeamsService proto yet). Each value is
// the snowflake ID the backend stores as owner_team_id.
const teamOptions = [
  { label: 'Platform Ops (id=1)', value: '1' },
  { label: 'DBA Team (id=2)', value: '2' },
  { label: 'Middleware Team (id=3)', value: '3' },
]

// P0-2: Provider dropdown — includes built-in (null/local/random) and real cloud
// providers. null = 内置零下载, used for logic-only tests.
const providerOptions = [
  { label: 'null (内置, 零下载)', value: 'null' },
  { label: 'random (内置)', value: 'random' },
  { label: 'local (内置)', value: 'local' },
  { label: 'alicloud', value: 'alicloud' },
  { label: 'aws', value: 'aws' },
  { label: 'azure', value: 'azure' },
]

export default function ModulesPage() {
  const [modules, setModules] = useState<Module[]>([])
  const [loading, setLoading] = useState(false)
  const [activeKey, setActiveKey] = useState<string[]>([])
  const [form] = Form.useForm<RegisterForm>()
  const [publishForm] = Form.useForm<PublishForm>()
  const [publishOpen, setPublishOpen] = useState(false)
  const [publishTarget, setPublishTarget] = useState<Module | null>(null)
  const [publishing, setPublishing] = useState(false)

  const load = () => {
    setLoading(true)
    fetchModules()
      .then((data) => {
        setModules(data)
        setLoading(false)
      })
      .catch(() => {
        setLoading(false)
      })
  }

  useEffect(() => {
    load()
  }, [])

  const openPublish = (mod: Module) => {
    setPublishTarget(mod)
    publishForm.setFieldsValue({
      displayName: mod.name,
      category: 'database',
      layer: 'application',
      ownerTeamId: '',
    })
    setPublishOpen(true)
  }

  const handleRegister = async () => {
    try {
      const values = await form.validateFields()
      // Fix 3: resolve git token. Explicit field value wins; otherwise fall
      // back to the AdminPage Git 凭证 localStorage rules matched by URL prefix.
      const resolvedToken = values.gitToken || resolveGitTokenForURL(values.gitSource)
      const result = await registerModule({
        gitSource: values.gitSource,
        modulePath: values.modulePath,
        version: values.version,
        provider: values.provider,
        name: values.displayName,
        ownerTeamId: values.team,
        gitToken: resolvedToken,
      })
      message.success(`已注册模块 ${values.displayName}@${values.version}`)
      form.resetFields()
      load()

      // Offer to publish to the catalog immediately.
      Modal.confirm({
        title: '是否发布到服务目录?',
        content: `模块 ${values.displayName} 已成功注册（version_id=${result.moduleVersionId}）。是否立即将其发布为服务目录项？`,
        okText: '发布',
        cancelText: '稍后',
        onOk: () => {
          // Seed the publish modal with the freshly registered module.
          openPublish({
            id: result.moduleId,
            name: values.displayName,
            gitSource: values.gitSource,
            modulePath: values.modulePath,
            version: values.version,
            provider: values.provider,
            varCount: 0,
            outputCount: 0,
            status: result.status,
            moduleVersionId: result.moduleVersionId,
          })
        },
      })
    } catch (err: any) {
      // Connect errors carry a `code` / `message`; surface them.
      if (err?.message) {
        message.error(`注册失败：${err.message}`)
      }
      // field validation handled by Form
    }
  }

  const handlePublish = async () => {
    if (!publishTarget?.moduleVersionId) {
      message.error('缺少 module_version_id，无法发布。请重新注册或联系管理员。')
      return
    }
    try {
      const values = await publishForm.validateFields()
      setPublishing(true)
      const item = await publishCatalogItem({
        moduleVersionId: publishTarget.moduleVersionId,
        displayName: values.displayName,
        category: values.category,
        layerLogicalId: values.layer,
        ownerTeamId: values.ownerTeamId,
      })
      message.success(`已发布到服务目录（id=${item.id}）`)
      setPublishOpen(false)
      publishForm.resetFields()
    } catch (err: any) {
      if (err?.message) {
        message.error(`发布失败：${err.message}`)
      }
    } finally {
      setPublishing(false)
    }
  }

  const columns: ColumnsType<Module> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: 'Git 源', dataIndex: 'gitSource', key: 'gitSource', ellipsis: true },
    { title: '模块路径', dataIndex: 'modulePath', key: 'modulePath', ellipsis: true },
    { title: '版本', dataIndex: 'version', key: 'version' },
    { title: 'Provider', dataIndex: 'provider', key: 'provider' },
    { title: '变量数', dataIndex: 'varCount', key: 'varCount' },
    { title: '输出数', dataIndex: 'outputCount', key: 'outputCount' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: ModuleStatus) => <Tag color={statusColor[s]}>{statusLabel[s] ?? s}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            disabled={!record.contractJson}
            onClick={() =>
              setActiveKey((prev) => (prev.includes(record.id) ? prev : [...prev, record.id]))
            }
          >
            查看契约
          </Button>
          <Button
            type="link"
            disabled={record.status === 'deprecated' || record.status === 'validation_failed'}
            onClick={() => openPublish(record)}
          >
            发布目录
          </Button>
        </Space>
      ),
    },
  ]

  const contractPanels = useMemo(
    () =>
      modules
        .filter((m) => m.contractJson)
        .map((m) => ({
          key: m.id,
          label: `${m.name} @ ${m.version} · variables_contract_json`,
          children: (
            <pre
              style={{
                background: '#0f1419',
                color: '#e6e6e6',
                padding: 16,
                borderRadius: 6,
                fontSize: 12,
                margin: 0,
                overflow: 'auto',
              }}
            >
              {m.contractJson ?? ''}
            </pre>
          ),
        })),
    [modules],
  )

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Card title="注册新模块">
        <Form form={form} layout="inline" style={{ rowGap: 12 }}>
          <Form.Item label="Git 源" name="gitSource" rules={[{ required: true, message: '请填写 Git 源' }]}>
            <Input placeholder="git@github.com:org/tf-modules.git" style={{ width: 260 }} />
          </Form.Item>
          <Form.Item label="模块路径" name="modulePath" rules={[{ required: true, message: '请填写模块路径' }]}>
            <Input placeholder="modules/rds/mysql" style={{ width: 200 }} />
          </Form.Item>
          <Form.Item label="版本" name="version" rules={[{ required: true, message: '请填写版本' }]}>
            <Input placeholder="v1.0.0" style={{ width: 120 }} />
          </Form.Item>
          <Form.Item label="Provider" name="provider" rules={[{ required: true, message: '请选择 provider' }]}>
            <Select
              style={{ width: 200 }}
              options={providerOptions}
              placeholder="请选择 provider"
            />
          </Form.Item>
          <Form.Item label="显示名" name="displayName" rules={[{ required: true, message: '请填写显示名' }]}>
            <Input placeholder="rds-mysql" style={{ width: 160 }} />
          </Form.Item>
          <Form.Item
            label="团队"
            name="team"
            rules={[{ required: true, message: '请选择团队' }]}
            tooltip="owner_team_id 必须是已存在团队的数字 snowflake ID"
          >
            <Select
              style={{ width: 200 }}
              options={teamOptions}
              placeholder="请选择团队"
            />
          </Form.Item>
          <Form.Item
            label="Git 访问令牌 (可选)"
            name="gitToken"
            tooltip="私有仓库的访问令牌；留空时按 Git 凭证规则（管理设置 → Git 凭证）匹配 URL 前缀自动注入"
          >
            <Input.Password placeholder="私有仓库 token，可空" style={{ width: 220 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={handleRegister}>
              注册
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card title="已注册模块">
        <Table<Module>
          rowKey="id"
          loading={loading}
          dataSource={modules}
          columns={columns}
          pagination={{ pageSize: 10 }}
          locale={{
            emptyText: <Empty description="暂无注册模块" />,
          }}
        />
      </Card>

      <Card title="模块契约 (variables_contract_json)">
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          契约由 <Text code>terraform-config-inspect</Text> 从 <Text code>variables.tf</Text> 提取的纯 scalar 信息（name/type/default/description/sensitive）。
        </Text>
        <Collapse
          activeKey={activeKey}
          onChange={(keys) => setActiveKey(keys as string[])}
          items={contractPanels}
        />
      </Card>

      <Modal
        title={`发布到服务目录 · ${publishTarget?.name ?? ''}`}
        open={publishOpen}
        onOk={handlePublish}
        onCancel={() => {
          setPublishOpen(false)
          publishForm.resetFields()
        }}
        okText="发布"
        cancelText="取消"
        confirmLoading={publishing}
        destroyOnClose
      >
        <Form form={publishForm} layout="vertical">
          <Form.Item label="module_version_id">
            <Input value={publishTarget?.moduleVersionId || ''} disabled />
          </Form.Item>
          <Form.Item
            label="目录显示名"
            name="displayName"
            rules={[{ required: true, message: '请填写显示名' }]}
          >
            <Input placeholder="rds-mysql" />
          </Form.Item>
          <Form.Item label="分类" name="category" rules={[{ required: true }]}>
            <Select options={categoryOptions} />
          </Form.Item>
          <Form.Item label="分层" name="layer" rules={[{ required: true }]}>
            <Select options={layerOptions} />
          </Form.Item>
          <Form.Item
            label="归属团队"
            name="ownerTeamId"
            rules={[{ required: true, message: '请选择团队' }]}
          >
            <Select options={teamOptions} placeholder="请选择团队" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
