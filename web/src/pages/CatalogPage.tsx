import { useEffect, useMemo, useState } from 'react'
import { Card, Col, Empty, Row, Select, Space, Statistic, Table, Tag, Button, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { fetchCatalogItems, type CatalogCategory, type CatalogItem, type CatalogStatus } from '../api'
import RequestFormModal from '../components/RequestFormModal'

const { Text } = Typography

const categoryLabel: Record<CatalogCategory, string> = {
  database: '数据库',
  compute: '计算',
  network: '网络',
  storage: '存储',
  middleware: '中间件',
}

const statusColor: Record<CatalogStatus, string> = {
  published: 'green',
  draft: 'orange',
  deprecated: 'red',
}

const statusLabel: Record<CatalogStatus, string> = {
  published: '已发布',
  draft: '草稿',
  deprecated: '已废弃',
}

export default function CatalogPage() {
  const [items, setItems] = useState<CatalogItem[]>([])
  const [loading, setLoading] = useState(false)
  const [category, setCategory] = useState<CatalogCategory | 'all'>('all')
  const [modalOpen, setModalOpen] = useState(false)
  const [activeItem, setActiveItem] = useState<CatalogItem | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    setLoading(true)
    fetchCatalogItems().then((data) => {
      setItems(data)
      setLoading(false)
    })
  }, [])

  const filtered = useMemo(
    () => (category === 'all' ? items : items.filter((i) => i.category === category)),
    [items, category],
  )

  const stats = useMemo(() => {
    return {
      total: items.length,
      db: items.filter((i) => i.category === 'database').length,
      compute: items.filter((i) => i.category === 'compute').length,
      network: items.filter((i) => i.category === 'network').length,
    }
  }, [items])

  const openRequest = (item: CatalogItem) => {
    setActiveItem(item)
    setModalOpen(true)
  }

  const columns: ColumnsType<CatalogItem> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      render: (c: CatalogCategory) => <Tag>{categoryLabel[c]}</Tag>,
      filters: Object.entries(categoryLabel).map(([value, text]) => ({ text, value })),
      onFilter: (value, record) => record.category === value,
    },
    { title: '分层', dataIndex: 'layer', key: 'layer' },
    { title: '归属团队', dataIndex: 'owner', key: 'owner' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: CatalogStatus) => <Tag color={statusColor[s]}>{statusLabel[s]}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Button type="link" disabled={record.status === 'deprecated'} onClick={() => openRequest(record)}>
          申请
        </Button>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic title="目录总数" value={stats.total} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="数据库类" value={stats.db} valueStyle={{ color: '#1677ff' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="计算类" value={stats.compute} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="网络类" value={stats.network} valueStyle={{ color: '#722ed1' }} />
          </Card>
        </Col>
      </Row>

      <Card
        title="资源目录"
        extra={
          <Space>
            <Text type="secondary">分类筛选：</Text>
            <Select
              value={category}
              onChange={(v) => setCategory(v)}
              style={{ width: 160 }}
              options={[
                { value: 'all', label: '全部' },
                ...Object.entries(categoryLabel).map(([value, label]) => ({ value, label })),
              ]}
            />
          </Space>
        }
      >
        <Table<CatalogItem>
          rowKey="id"
          loading={loading}
          dataSource={filtered}
          columns={columns}
          pagination={{ pageSize: 10 }}
          locale={{
            emptyText: <Empty description="请先注册模块并发布到服务目录" />,
          }}
        />
      </Card>

      <RequestFormModal
        open={modalOpen}
        catalogItem={activeItem}
        onCancel={() => setModalOpen(false)}
        onSubmit={(requestId) => {
          // P1-4: redirect to the requests list so the user can watch the new
          // ticket advance. The detail / list pages poll every 5s, so the row
          // appears within seconds.
          message.success(`工单已提交（${requestId}），正在跳转到我的工单`)
          navigate('/requests')
        }}
      />
    </Space>
  )
}
