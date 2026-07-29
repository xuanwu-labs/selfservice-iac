import { useCallback, useEffect, useState } from 'react'
import { Badge, Button, Card, Empty, Popconfirm, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { decideApproval, fetchPendingApprovals, type ApprovalItem, type Env } from '../api'

const envColor: Partial<Record<Env, string>> = { dev: 'green', staging: 'orange', prod: 'red' }

export default function ApprovalPage() {
  const [items, setItems] = useState<ApprovalItem[]>([])
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  // load: refetch the pending queue. Wrapped in useCallback and error-swallowing
  // so re-fetching after a decision is resilient to transient backend failures.
  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchPendingApprovals()
      setItems(data)
    } catch {
      // keep stale list on transient error
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  // handleDecide: optimistically remove the row, call the RPC, then re-fetch.
  // P1-2: the list refreshes so the decided item disappears immediately even
  // before the backend re-evaluates the queue. If the RPC throws we surface the
  // error but still re-fetch to reconcile with server truth.
  const handleDecide = async (id: string, decision: 'approve' | 'reject') => {
    // optimistic: drop the row so the UI reacts instantly
    setItems((prev) => prev.filter((it) => it.id !== id))
    try {
      await decideApproval(id, decision === 'approve' ? 'approved' : 'rejected')
      message.success(`已${decision === 'approve' ? '批准' : '拒绝'}工单 ${id}`)
    } catch (err: any) {
      message.error(`操作失败：${err?.message ?? err}`)
    } finally {
      // P1-2: re-fetch the authoritative queue regardless of outcome.
      load()
    }
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
        locale={{
          emptyText: <Empty description="暂无待审批工单" />,
        }}
      />
    </Card>
  )
}
