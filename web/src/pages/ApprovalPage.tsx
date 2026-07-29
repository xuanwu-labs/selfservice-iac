import { useEffect, useState } from 'react'
import { Badge, Button, Card, Popconfirm, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { decideApproval, fetchPendingApprovals, type ApprovalItem, type Env } from '../api'

const envColor: Partial<Record<Env, string>> = { dev: 'green', staging: 'orange', prod: 'red' }

export default function ApprovalPage() {
  const [items, setItems] = useState<ApprovalItem[]>([])
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const load = () => {
    setLoading(true)
    fetchPendingApprovals().then((data) => {
      setItems(data)
      setLoading(false)
    })
  }

  useEffect(() => {
    load()
  }, [])

  const handleDecide = async (id: string, decision: 'approve' | 'reject') => {
    await decideApproval(id, decision === 'approve' ? 'approved' : 'rejected')
    message.success(`已${decision === 'approve' ? '批准' : '拒绝'}工单 ${id}`)
    load()
  }

  const columns: ColumnsType<ApprovalItem> = [
    { title: '工单号', dataIndex: 'id', key: 'id' },
    { title: '资源', dataIndex: 'catalogItem', key: 'catalogItem' },
    { title: '环境', dataIndex: 'env', key: 'env', render: (e: Env) => <Tag color={envColor[e]}>{e}</Tag> },
    { title: '团队', dataIndex: 'team', key: 'team' },
    { title: '申请人', dataIndex: 'approver', key: 'approver' },
    {
      title: '操作',
      key: 'action',
      width: 280,
      render: (_, record) => (
        <Space>
          <Popconfirm title="确认批准该工单？" onConfirm={() => handleDecide(record.id, 'approve')} okText="批准" cancelText="取消">
            <Button type="primary" size="small">批准</Button>
          </Popconfirm>
          <Popconfirm title="确认拒绝该工单？" onConfirm={() => handleDecide(record.id, 'reject')} okText="拒绝" cancelText="取消" okButtonProps={{ danger: true }}>
            <Button danger size="small">拒绝</Button>
          </Popconfirm>
          <Button type="link" size="small" onClick={() => navigate(`/requests/${record.id}`)}>查看 Plan</Button>
        </Space>
      ),
    },
  ]

  return (
    <Card
      title={
        <Badge count={items.length} offset={[12, 0]}>
          <span>待审批工单</span>
        </Badge>
      }
    >
      <Table<ApprovalItem>
        rowKey="id"
        loading={loading}
        dataSource={items}
        columns={columns}
        pagination={{ pageSize: 10 }}
      />
    </Card>
  )
}
