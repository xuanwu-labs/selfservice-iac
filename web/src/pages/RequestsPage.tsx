import { useCallback, useEffect, useMemo, useState } from 'react'
import { Button, Empty, Popconfirm, Space, Table, Tabs, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { cancelRequest, fetchRequests, type Env, type IaCRequest, type RequestStatus } from '../api'

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

type TabKey = 'all' | 'pending' | 'in_progress' | 'done'

export default function RequestsPage() {
  const [requests, setRequests] = useState<IaCRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [tab, setTab] = useState<TabKey>('all')
  const navigate = useNavigate()

  // load: fetch the request list. Wrapped in useCallback so the polling effect
  // can keep a stable reference. Swallows errors so the 5s interval never
  // surfaces an uncaught rejection in the console on transient backend hiccups.
  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchRequests()
      setRequests(data)
    } catch {
      // keep stale data; the table stays on the last successful snapshot.
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    // P1-1: poll every 5s so the list reflects backend status transitions
    // (code_generated → plan_ready → pending_approval → applying → completed)
    // without a manual refresh.
    const timer = setInterval(load, 5000)
    return () => clearInterval(timer)
  }, [load])

  // Terminal statuses — cancel is only valid for non-terminal requests.
  const terminalStatuses = ['completed', 'failed', 'cancelled', 'rejected']

  const handleCancel = async (id: string) => {
    try {
      await cancelRequest(id, 'cancelled by user from requests list')
      message.success('工单已取消')
      load()
    } catch (err: any) {
      message.error(`取消失败：${err?.message ?? err}`)
    }
  }

  const counts = useMemo(
    () => ({
      all: requests.length,
      pending: requests.filter((r) => r.status === 'pending_approval').length,
      in_progress: requests.filter((r) => ['code_generated', 'plan_ready', 'applying'].includes(r.status)).length,
      done: requests.filter((r) => ['completed', 'failed'].includes(r.status)).length,
    }),
    [requests],
  )

  const filtered = useMemo(() => {
    switch (tab) {
      case 'pending':
        return requests.filter((r) => r.status === 'pending_approval')
      case 'in_progress':
        return requests.filter((r) => ['code_generated', 'plan_ready', 'applying'].includes(r.status))
      case 'done':
        return requests.filter((r) => ['completed', 'failed'].includes(r.status))
      default:
        return requests
    }
  }, [requests, tab])

  const columns: ColumnsType<IaCRequest> = [
    { title: '工单号', dataIndex: 'id', key: 'id' },
    { title: '资源', dataIndex: 'catalogItem', key: 'catalogItem' },
    { title: '环境', dataIndex: 'env', key: 'env', render: (e: Env) => <Tag color={envColor[e]}>{e}</Tag> },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: RequestStatus) => <Tag color={statusColor[s]}>{statusLabel[s]}</Tag>,
    },
    { title: '团队', dataIndex: 'team', key: 'team' },
    { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt' },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => navigate(`/requests/${record.id}`)}>详情</Button>
          {record.status === 'pending_approval' && (
            <Button type="link" onClick={() => navigate('/approvals')}>审批</Button>
          )}
          {record.status === 'plan_ready' && (
            <Button type="link" onClick={() => navigate(`/requests/${record.id}`)}>Plan</Button>
          )}
          {!terminalStatuses.includes(record.status) && (
            <Popconfirm
              title="确认取消该工单？"
              description="取消后工单进入终态，无法继续推进。"
              okText="取消工单"
              okButtonProps={{ danger: true }}
              cancelText="作罢"
              onConfirm={() => handleCancel(record.id)}
            >
              <Button type="link" danger>取消</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <>
      <Tabs
        activeKey={tab}
        onChange={(k) => setTab(k as TabKey)}
        items={[
          { key: 'all', label: `全部 (${counts.all})` },
          { key: 'pending', label: `待审批 (${counts.pending})` },
          { key: 'in_progress', label: `进行中 (${counts.in_progress})` },
          { key: 'done', label: `已完成 (${counts.done})` },
        ]}
      />
      <Table<IaCRequest>
        rowKey="id"
        loading={loading}
        dataSource={filtered}
        columns={columns}
        pagination={{ pageSize: 10 }}
        locale={{
          emptyText: <Empty description="暂无工单，请从服务目录申请资源" />,
        }}
      />
    </>
  )
}
