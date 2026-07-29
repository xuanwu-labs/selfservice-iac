import { useMemo } from 'react'
import { Layout, Menu, Tag, Space } from 'antd'
import {
  AppstoreOutlined,
  BlockOutlined,
  ClusterOutlined,
  FileTextOutlined,
  AuditOutlined,
  HomeOutlined,
} from '@ant-design/icons'
import { Routes, Route, useNavigate, useLocation, Link } from 'react-router-dom'
import CatalogPage from './pages/CatalogPage'
import LayerCatalogPage from './pages/LayerCatalogPage'
import ModulesPage from './pages/ModulesPage'
import RequestsPage from './pages/RequestsPage'
import ApprovalPage from './pages/ApprovalPage'
import RequestDetailPage from './pages/RequestDetailPage'

const { Sider, Header, Content } = Layout

interface MenuItem {
  key: string
  label: string
  icon: React.ReactNode
  path: string
}

const menuItems: MenuItem[] = [
  { key: 'catalog', label: '资源目录', icon: <AppstoreOutlined />, path: '/catalog' },
  { key: 'layers', label: '分层目录', icon: <BlockOutlined />, path: '/layers' },
  { key: 'modules', label: '模块仓库', icon: <ClusterOutlined />, path: '/modules' },
  { key: 'requests', label: '我的工单', icon: <FileTextOutlined />, path: '/requests' },
  { key: 'approvals', label: '审批中心', icon: <AuditOutlined />, path: '/approvals' },
]

const titleMap: Record<string, string> = {
  '/': '首页',
  '/catalog': '资源目录',
  '/layers': '分层目录',
  '/modules': '模块仓库',
  '/requests': '我的工单',
  '/approvals': '审批中心',
}

export default function App() {
  const navigate = useNavigate()
  const location = useLocation()

  const selectedKey = useMemo(() => {
    const base = '/' + location.pathname.split('/')[1]
    return menuItems.find((m) => m.path === base)?.key ?? 'catalog'
  }, [location.pathname])

  const pageTitle = useMemo(() => {
    if (location.pathname.startsWith('/requests/')) return '工单详情'
    return titleMap[location.pathname] ?? 'Aether IaC 自助平台'
  }, [location.pathname])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible width={220} style={{ background: '#001529' }}>
        <div style={{ height: 56, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 18, fontWeight: 700 }}>
          🏗️ Aether
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => {
            const item = menuItems.find((m) => m.key === key)
            if (item) navigate(item.path)
          }}
          items={[
            ...menuItems.map((m) => ({ key: m.key, icon: m.icon, label: m.label })),
            { key: 'home', icon: <HomeOutlined />, label: <Link to="/">首页</Link> },
          ]}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid #f0f0f0' }}>
          <h2 style={{ margin: 0, fontSize: 18 }}>{pageTitle}</h2>
          <Space>
            <Tag color="red">prod</Tag>
            <Tag color="purple">admin</Tag>
          </Space>
        </Header>
        <Content style={{ margin: 24, padding: 24, background: '#fff', borderRadius: 8, minHeight: 360 }}>
          <Routes>
            <Route path="/" element={<CatalogPage />} />
            <Route path="/catalog" element={<CatalogPage />} />
            <Route path="/layers" element={<LayerCatalogPage />} />
            <Route path="/modules" element={<ModulesPage />} />
            <Route path="/requests" element={<RequestsPage />} />
            <Route path="/requests/:id" element={<RequestDetailPage />} />
            <Route path="/approvals" element={<ApprovalPage />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  )
}
