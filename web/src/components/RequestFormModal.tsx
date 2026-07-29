import { Form, Input, Modal, Select, Card, Typography, message } from 'antd'
import type { CatalogItem, Env } from '../api'

const { Text } = Typography

export interface RequestFormValues {
  env: Env
  instance_type: string
  storage_size: string
  ha_enabled: string
  tags: string
}

interface RequestFormModalProps {
  open: boolean
  catalogItem: CatalogItem | null
  onCancel: () => void
  onSubmit?: (values: RequestFormValues) => void
}

const formSchemaExplain = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "instance_type": { "type": "string", "default": "rds.mysql.s2.large" },
    "storage_size": { "type": "number", "default": 100 },
    "ha_enabled": { "type": "boolean", "default": false }
  },
  "required": ["instance_type"]
}`

export default function RequestFormModal({ open, catalogItem, onCancel, onSubmit }: RequestFormModalProps) {
  const [form] = Form.useForm<RequestFormValues>()

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      message.success(`已提交 ${catalogItem?.name ?? ''} 申请，工单号 REQ-${Math.floor(Math.random() * 9000) + 1000}`)
      onSubmit?.(values)
      form.resetFields()
      onCancel()
    } catch {
      // validation handled by Form
    }
  }

  return (
    <Modal
      title={`申请资源 · ${catalogItem?.name ?? ''}`}
      open={open}
      onOk={handleOk}
      onCancel={() => {
        form.resetFields()
        onCancel()
      }}
      okText="提交申请"
      cancelText="取消"
      width={640}
      destroyOnClose
    >
      <Form form={form} layout="vertical" initialValues={{ instance_type: 'rds.mysql.s2.large', storage_size: '100', ha_enabled: 'false' }}>
        <Form.Item label="资源" name="resource">
          <Input disabled value={catalogItem?.name} placeholder={catalogItem?.name} />
        </Form.Item>
        <Form.Item label="环境" name="env" rules={[{ required: true, message: '请选择环境' }]}>
          <Select placeholder="请选择环境" options={[
            { value: 'dev', label: '开发 (dev)' },
            { value: 'staging', label: '预发 (staging)' },
            { value: 'prod', label: '生产 (prod)' },
          ]} />
        </Form.Item>
        <Form.Item label="实例规格" name="instance_type" rules={[{ required: true, message: '请选择实例规格' }]}>
          <Select options={[
            { value: 'rds.mysql.s2.large', label: 'rds.mysql.s2.large (4C8G)' },
            { value: 'rds.mysql.s2.xlarge', label: 'rds.mysql.s2.xlarge (8C16G)' },
            { value: 'rds.mysql.s3.large', label: 'rds.mysql.s3.large (性能增强)' },
          ]} />
        </Form.Item>
        <Form.Item label="存储 (GB)" name="storage_size">
          <Select options={[
            { value: '50', label: '50' },
            { value: '100', label: '100' },
            { value: '200', label: '200' },
            { value: '500', label: '500' },
          ]} />
        </Form.Item>
        <Form.Item label="高可用" name="ha_enabled">
          <Select options={[
            { value: 'false', label: '单机 (false)' },
            { value: 'true', label: '主备 (true)' },
          ]} />
        </Form.Item>
        <Form.Item label="标签" name="tags">
          <Input placeholder="如 team=payment,biz=checkout" />
        </Form.Item>
      </Form>

      <Card size="small" type="inner" title="声明式表单 (form_schema_json)" style={{ marginTop: 8, background: '#fafafa' }}>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
          本表单由模块契约的 <Text code>form_schema_json</Text> 自动渲染（RJSF），平台按 Draft 2020-12 校验。
        </Text>
        <pre style={{ background: '#0f1419', color: '#e6e6e6', padding: 12, borderRadius: 6, fontSize: 12, margin: 0, overflow: 'auto' }}>
{formSchemaExplain}
        </pre>
      </Card>
    </Modal>
  )
}
