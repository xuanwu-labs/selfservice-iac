import { useEffect, useState } from 'react'
import { Button, Card, Col, Descriptions, Row, Space, Steps, Tag, Timeline, Typography } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { useNavigate, useParams } from 'react-router-dom'
import { fetchRequestDetail, type Env, type RequestDetail, type RequestStatus } from '../api'

const { Title, Text } = Typography

const statusColor: Record<RequestStatus, string> = {
  draft: 'default',
  code_generated: 'cyan',
  plan_ready: 'blue',
  pending_approval: 'orange',
  applying: 'blue',
  completed: 'green',
  failed: 'red',
}
const statusLabel: Record<RequestStatus, string> = {
  draft: '草稿',
  code_generated: '已生成代码',
  plan_ready: 'Plan 就绪',
  pending_approval: '待审批',
  applying: 'Apply 中',
  completed: '已完成',
  failed: '失败',
}
const envColor: Record<Env, string> = { dev: 'green', staging: 'orange', prod: 'red' }

export default function RequestDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [detail, setDetail] = useState<RequestDetail | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    fetchRequestDetail(id ?? '').then((data) => {
      setDetail(data)
      setLoading(false)
    })
  }, [id])

  if (loading || !detail) {
    return <div style={{ padding: 24 }}>加载中...</div>
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/requests')}>
        返回工单列表
      </Button>

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
              <Descriptions.Item label="资源">{detail.catalogItem}</Descriptions.Item>
              <Descriptions.Item label="环境">
                <Tag color={envColor[detail.env]}>{detail.env}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="团队">{detail.team}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColor[detail.status]}>{statusLabel[detail.status]}</Tag>
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="状态时间线">
            <Timeline items={detail.timeline.map((t) => ({ color: t.color, children: <Text><strong>{t.label}</strong><br /><Text type="secondary">{t.time}</Text></Text> }))} />
          </Card>
        </Col>
      </Row>

      <Card title="Plan Diff 摘要">
        <pre style={{ background: '#0f1419', color: '#e6e6e6', padding: 16, borderRadius: 6, fontSize: 12, margin: 0, overflow: 'auto' }}>
{detail.planDiff}
        </pre>
      </Card>
    </Space>
  )
}
