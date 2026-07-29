import { useEffect, useState } from 'react'
import { Card, Col, Row, Table, Tag, Typography, Space, Button } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { fetchCatalogItems, type CatalogItem, type CatalogLayer } from '../api'
import RequestFormModal from '../components/RequestFormModal'

const { Text, Title, Paragraph } = Typography

interface LayerConfig {
  key: CatalogLayer
  title: string
  color: string
  pathTemplate: string
  desc: string
}

const layerConfigs: LayerConfig[] = [
  { key: 'global', title: '全局层 (Global)', color: '#f5222d', pathTemplate: 'global/{{.component}}-{{.tenant}}-{{.env}}', desc: 'VPC、ACK、CEN 等全局共享基础设施。平台运维团队维护。跨租户共享。' },
  { key: 'middleware', title: '中间件层 (Middleware)', color: '#fa8c16', pathTemplate: 'middleware/{{.tenant}}/{{.component}}-{{.env}}', desc: 'RDS、Redis、Kafka 等共享中间件。DBA + 中间件团队维护。上层通过 remote_state 引用。' },
  { key: 'application', title: '应用层 (Application)', color: '#52c41a', pathTemplate: 'application/{{.tenant}}/{{.team}}/{{if .space}}{{.space}}/{{end}}{{.component}}-{{.env}}', desc: '业务 ECS、SLB 等。各业务团队独立维护。按团队+环境隔离。' },
]

const dagCode = `# 跨层依赖方向（DAG，单向无环）
#
#   Global (VPC/ACK) ──→ Middleware (RDS/Redis) ──→ Application (ECS/SLB)
#        │                      │                         │
#        └── CEN ───────────────┘                         │
#                               └── ACK ─────────────────┘
#
# 路径模板（D29 layer-first Path Contract）：
#   global:       global/vpc-platform-default-prod
#   middleware:   middleware/platform-default/rds-prod
#   application:  application/platform-default/team-a/orders/ecs-prod
#
# 跨层引用：上层通过 terraform_remote_state 读下层 outputs`

export default function LayerCatalogPage() {
  const [items, setItems] = useState<CatalogItem[]>([])
  const [loading, setLoading] = useState(false)
  // P1-5: per-row apply button opens the same RequestFormModal the catalog page
  // uses, so a user can request a resource directly from the layer view.
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

  const openRequest = (item: CatalogItem) => {
    setActiveItem(item)
    setModalOpen(true)
  }

  const columns: ColumnsType<CatalogItem> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '分类', dataIndex: 'category', key: 'category' },
    { title: '归属团队', dataIndex: 'owner', key: 'owner' },
    {
      title: '路径模板',
      dataIndex: 'pathTemplate',
      key: 'pathTemplate',
      render: (t: string) => (t ? <Text code copyable>{t}</Text> : '-'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s) => <Tag color={s === 'published' ? 'green' : s === 'draft' ? 'orange' : 'red'}>{s}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <Button
          type="link"
          size="small"
          disabled={record.status === 'deprecated'}
          onClick={() => openRequest(record)}
        >
          申请
        </Button>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Card title="依赖图 (DAG)" size="small">
        <pre style={{ background: '#0f1419', color: '#e6e6e6', padding: 16, borderRadius: 6, fontSize: 12, margin: 0, overflow: 'auto' }}>
{dagCode}
        </pre>
      </Card>

      <Row gutter={16}>
        {layerConfigs.map((cfg) => {
          const layerItems = items.filter((i) => i.layer === cfg.key)
          return (
            <Col span={8} key={cfg.key}>
              <Card
                title={
                  <Space>
                    <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: 2, background: cfg.color }} />
                    {cfg.title}
                    <Tag>{layerItems.length}</Tag>
                  </Space>
                }
              >
                <Paragraph type="secondary" style={{ fontSize: 12, minHeight: 44 }}>
                  {cfg.desc}
                </Paragraph>
                <Title level={5} style={{ marginTop: 0 }}>路径模板</Title>
                <Paragraph><Text code copyable>{cfg.pathTemplate}</Text></Paragraph>
                <Table<CatalogItem>
                  rowKey="id"
                  size="small"
                  loading={loading}
                  dataSource={layerItems}
                  columns={columns}
                  pagination={false}
                  scroll={{ y: 320 }}
                  locale={{ emptyText: <Text type="secondary" style={{ fontSize: 12 }}>本层暂无上架资源</Text> }}
                />
              </Card>
            </Col>
          )
        })}
      </Row>

      <RequestFormModal
        open={modalOpen}
        catalogItem={activeItem}
        onCancel={() => setModalOpen(false)}
        onSubmit={(requestId) => {
          // Match CatalogPage: after submit, jump to the requests list so the
          // user can watch the new ticket progress.
          navigate('/requests')
          // navigate happens on next tick; brief inline confirmation is omitted
          // to avoid a dangling toast after unmount — the requests page is the
          // source of truth for the new ticket (requestId available here).
          void requestId
        }}
      />
    </Space>
  )
}
