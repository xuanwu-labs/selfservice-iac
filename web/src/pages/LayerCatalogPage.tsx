import { useEffect, useState } from 'react'
import { Card, Col, Row, Table, Tag, Typography, Space } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { fetchCatalogItems, type CatalogItem, type CatalogLayer } from '../api'

const { Text, Title, Paragraph } = Typography

interface LayerConfig {
  key: CatalogLayer
  title: string
  color: string
  pathTemplate: string
  desc: string
}

const layerConfigs: LayerConfig[] = [
  { key: 'global', title: '全局层 (Global)', color: '#1677ff', pathTemplate: '{env}/global/{type}/{name}', desc: 'VPC、SLB、NAT 等环境级共享资源，团队无关，由网络/存储团队维护。' },
  { key: 'middleware', title: '中间件层 (Middleware)', color: '#722ed1', pathTemplate: '{env}/middleware/{type}/{name}', desc: 'Redis、Kafka 等共享中间件集群，按环境统一部署。' },
  { key: 'application', title: '应用层 (Application)', color: '#52c41a', pathTemplate: '{team}/{env}/{type}/{name}', desc: '业务自有资源（RDS/ECS），按团队 + 环境隔离。' },
]

const dagCode = `# DAG 依赖方向（底层 → 上层）
#
#   application ──┐
#                 ├──► middleware ──► global
#                 │     (Redis/Kafka)   (VPC/SLB)
#                 │
#   application ──┘
#
# 路径模板自动推导：
#   global:        prod/global/vpc/payment-vpc
#   middleware:    prod/middleware/redis/growth-cache
#   application:   payment/prod/db/checkout-mysql-001`

export default function LayerCatalogPage() {
  const [items, setItems] = useState<CatalogItem[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    fetchCatalogItems().then((data) => {
      setItems(data)
      setLoading(false)
    })
  }, [])

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
                />
              </Card>
            </Col>
          )
        })}
      </Row>
    </Space>
  )
}
