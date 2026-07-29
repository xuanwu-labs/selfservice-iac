import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Card, Col, Descriptions, Empty, Row, Space, Steps, Tag, Timeline, Typography } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { useNavigate, useParams } from 'react-router-dom'
import { fetchRequestDetail, type Env, type RequestDetail, type RequestStatus } from '../api'

const { Title, Text } = Typography

const statusColor: Partial<Record<RequestStatus, string>> = {
  draft: 'default',
  code_generated: 'cyan',
  plan_ready: 'blue',
  pending_approval: 'orange',
  applying: 'blue',
  completed: 'green',
  failed: 'red',
}
const statusLabel: Partial<Record<RequestStatus, string>> = {
  draft: '草稿',
  code_generated: '已生成代码',
  plan_ready: 'Plan 就绪',
  pending_approval: '待审批',
  applying: 'Apply 中',
  completed: '已完成',
  failed: '失败',
}
const envColor: Partial<Record<Env, string>> = { dev: 'green', staging: 'orange', prod: 'red' }

export default function RequestDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [detail, setDetail] = useState<RequestDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // load: refetch the detail (real getRequest + listRequestEvents + getArtifact
  // in api.ts). Swallows errors into a banner so the 5s polling interval can
  // keep retrying on transient failures without unmounting the page.
  const load = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const data = await fetchRequestDetail(id)
      setDetail(data)
      setError(null)
    } catch (err: any) {
      setError(err?.message ?? String(err))
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    load()
    // Poll every 5s so the steps / timeline advance as the backend transitions
    // status (mirrors the list page). Detail is a long-lived view while waiting
    // on plan/approval, so auto-refresh is the expected UX.
    const timer = setInterval(load, 5000)
    return () => clearInterval(timer)
  }, [load])

  if (error && !detail) {
    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/requests')}>
          返回工单列表
        </Button>
        <Alert type="error" showIcon message="加载工单详情失败" description={error} />
        <Button onClick={load}>重试</Button>
      </Space>
    )
  }

  if ((loading && !detail) || !detail) {
    return <div style={{ padding: 24 }}>加载中...</div>
  }

  // The detail payload exposes resource/env/team as label/value pairs in `info`;
  // status is surfaced as the structured statusKey / statusName fields. Build a
  // lookup for the info-pair labels (Chinese).
  const infoMap: Record<string, string> = {}
  for (const { label, value } of detail.info) {
    infoMap[label] = value
  }
  const catalogItem = infoMap['资源'] ?? ''
  const env = infoMap['环境'] as Env | undefined
  const team = infoMap['团队'] ?? ''
  const statusKey = detail.statusKey

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/requests')}>
        返回工单列表
      </Button>

      {error && (
        <Alert type="warning" showIcon message="刷新失败（保留上次快照）" description={error} />
      )}

      <Card loading={loading}>
        <Steps
          current={detail.steps.findIndex((s) => s.status === 'process' || s.status === 'error')}
          items={detail.steps.map((s) => ({ title: s.title, status: s.status, description: s.description }))}
        />
      </Card>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="工单信息">
            <Descriptions column={1}>
              <Descriptions.Item label="工单号">{detail.id}</Descriptions.Item>
              <Descriptions.Item label="资源">{catalogItem}</Descriptions.Item>
              <Descriptions.Item label="环境">
                <Tag color={env ? envColor[env] : undefined}>{env}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="团队">{team}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusKey ? statusColor[statusKey] : undefined}>
                  {statusKey ? (statusLabel[statusKey] ?? detail.statusName) : detail.statusName}
                </Tag>
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="状态时间线">
            <Timeline items={detail.timeline.map((t) => ({ color: t.color, children: <Text><strong>{t.content}</strong><br /><Text type="secondary">{t.time}</Text></Text> }))} />
          </Card>
        </Col>
      </Row>

      <Card title="Plan Diff 摘要">
        {detail.planDiff ? (
          <pre style={{ background: '#0f1419', color: '#e6e6e6', padding: 16, borderRadius: 6, fontSize: 12, margin: 0, overflow: 'auto' }}>
{detail.planDiff}
          </pre>
        ) : (
          <Empty description="暂无 Plan 摘要（Plan 生成后将显示）" />
        )}
      </Card>
    </Space>
  )
}
