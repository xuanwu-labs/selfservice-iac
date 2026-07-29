import { useEffect, useMemo, useState } from 'react'
import { Alert, Form, Input, Modal, Select, Spin, Typography, message } from 'antd'
import FormRJSF from '@rjsf/antd'
import type { RJSFSchema, RJSFValidationError } from '@rjsf/utils'
import validator from '@rjsf/validator-ajv8'
import {
  createRequest,
  getCatalogItemDetail,
  type CatalogItem,
  type CatalogItemDetail,
  type Env,
} from '../api'

const { Text } = Typography

interface RequestFormModalProps {
  open: boolean
  catalogItem: CatalogItem | null
  onCancel: () => void
  onSubmit?: (requestId: string) => void
}

interface MetaForm {
  env: Env
  teamId: string
}

// Default JSON schema used when the backend has not produced a form_schema_json
// for this catalog item. Lets the operator still submit a request (the values
// become free-form text inputs).
const fallbackSchema: RJSFSchema = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  title: '参数',
  type: 'object',
  properties: {
    name: { type: 'string', title: '资源名称', default: '' },
    notes: { type: 'string', title: '备注', default: '' },
  },
  required: ['name'],
}

export default function RequestFormModal({ open, catalogItem, onCancel, onSubmit }: RequestFormModalProps) {
  const [metaForm] = Form.useForm<MetaForm>()
  const [detail, setDetail] = useState<CatalogItemDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [rjsfFormData, setRjsfFormData] = useState<Record<string, unknown>>({})

  // Load the catalog item detail (with form_schema_json) whenever the target
  // changes / the modal opens.
  useEffect(() => {
    if (!open || !catalogItem?.id) {
      setDetail(null)
      setRjsfFormData({})
      return
    }
    setLoading(true)
    getCatalogItemDetail(catalogItem.id)
      .then((d) => {
        setDetail(d)
        setLoading(false)
      })
      .catch((err: any) => {
        message.error(`加载表单 schema 失败：${err?.message ?? err}`)
        setLoading(false)
      })
  }, [open, catalogItem?.id])

  // Parse form_schema_json into a RJSF schema. Falls back when missing/invalid.
  const schema: RJSFSchema = useMemo(() => {
    const raw = detail?.formSchemaJson
    if (!raw) return fallbackSchema
    try {
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed === 'object') return parsed as RJSFSchema
    } catch {
      // fall through to fallback
    }
    return fallbackSchema
  }, [detail?.formSchemaJson])

  const handleSubmit = async () => {
    if (!catalogItem?.id) return
    let meta: MetaForm
    try {
      meta = await metaForm.validateFields()
    } catch {
      // validation handled by Form
      return
    }
    setSubmitting(true)
    try {
      const result = await createRequest({
        catalogItemId: catalogItem.id,
        envId: meta.env,
        teamId: meta.teamId,
        formData: rjsfFormData,
      })
      message.success(`已提交 ${catalogItem.name} 申请，工单号 ${result.requestId}`)
      onSubmit?.(result.requestId)
      metaForm.resetFields()
      setRjsfFormData({})
      onCancel()
    } catch (err: any) {
      message.error(`提交失败：${err?.message ?? err}`)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      title={`申请资源 · ${catalogItem?.name ?? ''}`}
      open={open}
      onOk={handleSubmit}
      onCancel={() => {
        metaForm.resetFields()
        setRjsfFormData({})
        onCancel()
      }}
      okText="提交申请"
      cancelText="取消"
      width={720}
      destroyOnClose
      confirmLoading={submitting}
    >
      <Spin spinning={loading}>
        <Form form={metaForm} layout="vertical" initialValues={{ env: undefined, teamId: '' }}>
          <Form.Item label="资源" name="resource">
            <Input disabled value={catalogItem?.name} placeholder={catalogItem?.name} />
          </Form.Item>
          <Form.Item label="环境" name="env" rules={[{ required: true, message: '请选择环境' }]}>
            <Select
              placeholder="请选择环境"
              options={[
                { value: 'dev', label: '开发 (dev)' },
                { value: 'staging', label: '预发 (staging)' },
                { value: 'prod', label: '生产 (prod)' },
              ]}
            />
          </Form.Item>
          <Form.Item
            label="团队 ID"
            name="teamId"
            rules={[{ required: true, message: '请填写团队 ID' }]}
            tooltip="owner team 的 snowflake ID"
          >
            <Input placeholder="如 1001" />
          </Form.Item>
        </Form>

        <div style={{ marginTop: 8 }}>
          <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
            本表单由模块契约的 <Text code>form_schema_json</Text> 通过 RJSF 动态渲染，平台按 JSON Schema 校验。
          </Text>
          {detail?.formSchemaJson ? null : (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 8 }}
              message="该目录项未提供 form_schema_json"
              description="使用通用兜底表单提交。请在管理端为该模块发布时配置表单 schema。"
            />
          )}
          <FormRJSF
            schema={schema}
            validator={validator}
            formData={rjsfFormData}
            onChange={(e) => setRjsfFormData(e.formData as Record<string, unknown>)}
            onError={(errors: RJSFValidationError[]) =>
              message.warning(`表单存在 ${errors.length} 个校验错误`)
            }
            showErrorList={false}
          >
            {/* RJSF renders its own submit button by default; we hide it and
                drive submission through the Modal footer's OK button. */}
            <div style={{ display: 'none' }}>
              <button type="submit" />
            </div>
          </FormRJSF>
        </div>
      </Spin>
    </Modal>
  )
}
