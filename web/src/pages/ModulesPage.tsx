import { useEffect, useMemo, useState } from 'react'
import { Button, Card, Collapse, Form, Input, Select, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { fetchModules, type Module, type ModuleStatus } from '../api'

const { Text } = Typography

const statusColor: Partial<Record<ModuleStatus, string>> = {
  ready: 'green',
  extracting: 'blue',
  failed: 'red',
}
const statusLabel: Partial<Record<ModuleStatus, string>> = {
  ready: '可用',
  extracting: '契约提取中',
  failed: '失败',
}

interface RegisterForm {
  gitSource: string
  modulePath: string
  version: string
  provider: string
  displayName: string
  team: string
}

export default function ModulesPage() {
  const [modules, setModules] = useState<Module[]>([])
  const [loading, setLoading] = useState(false)
  const [activeKey, setActiveKey] = useState<string[]>([])
  const [form] = Form.useForm<RegisterForm>()

  useEffect(() => {
    setLoading(true)
    fetchModules().then((data) => {
      setModules(data)
      setLoading(false)
    })
  }, [])

  const handleRegister = async () => {
    try {
      const values = await form.validateFields()
      message.success(`已注册模块 ${values.displayName}，开始提取契约...`)
      form.resetFields()
    } catch {
      // validation handled by form
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
      render: (s: ModuleStatus) => <Tag color={statusColor[s]}>{statusLabel[s]}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            disabled={!record.contractJson}
            onClick={() => setActiveKey((prev) => (prev.includes(record.id) ? prev : [...prev, record.id]))}
          >
            查看契约
          </Button>
          <Button type="link" disabled>发布到目录</Button>
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
            <pre style={{ background: '#0f1419', color: '#e6e6e6', padding: 16, borderRadius: 6, fontSize: 12, margin: 0, overflow: 'auto' }}>
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
              style={{ width: 140 }}
              options={[
                { value: 'alicloud', label: 'alicloud' },
                { value: 'aws', label: 'aws' },
                { value: 'azurerm', label: 'azurerm' },
              ]}
            />
          </Form.Item>
          <Form.Item label="显示名" name="displayName" rules={[{ required: true, message: '请填写显示名' }]}>
            <Input placeholder="rds-mysql" style={{ width: 160 }} />
          </Form.Item>
          <Form.Item label="团队" name="team" rules={[{ required: true, message: '请填写团队' }]}>
            <Input placeholder="DBA 团队" style={{ width: 140 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={handleRegister}>注册</Button>
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
        />
      </Card>

      <Card title="模块契约 (variables_contract_json)">
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          契约由 <Text code>terraform-config-inspect</Text> 从 <Text code>variables.tf</Text> 提取的纯 scalar 信息（name/type/default/description/sensitive）。
        </Text>
        <Collapse activeKey={activeKey} onChange={(keys) => setActiveKey(keys as string[])} items={contractPanels} />
      </Card>
    </Space>
  )
}
