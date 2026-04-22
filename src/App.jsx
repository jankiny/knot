import { useEffect, useState } from 'react'
import { Button, ConfigProvider, Layout, Menu, theme } from 'antd'
import {
  BlockOutlined,
  BorderOutlined,
  CloseOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  InfoCircleOutlined,
  MailOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MinusOutlined,
  PlusSquareOutlined,
  SettingOutlined
} from '@ant-design/icons'
import zhCN from 'antd/locale/zh_CN'
import About from './components/About'
import AutoArchive from './components/AutoArchive'
import DailyReport from './components/DailyReport'
import MailList from './components/MailList'
import QuickCreate from './components/QuickCreate'
import Settings from './components/Settings'
import { USE_MOCK } from './services/api'
import { getSettings } from './services/settings'
import './App.css'

const { Header, Sider, Content } = Layout

function App() {
  const [collapsed, setCollapsed] = useState(false)
  const [activeKey, setActiveKey] = useState('mail')
  const [isMaximized, setIsMaximized] = useState(false)
  const [settings, setSettings] = useState(getSettings())

  const {
    token: { colorBgContainer, borderRadiusLG }
  } = theme.useToken()

  const isIntegratedStyle = settings.windowStyle === 'integrated'

  useEffect(() => {
    const handleOpenSettings = () => setActiveKey('settings')
    const handleFocus = () => setSettings(getSettings())
    window.addEventListener('openSettings', handleOpenSettings)
    window.addEventListener('focus', handleFocus)
    return () => {
      window.removeEventListener('openSettings', handleOpenSettings)
      window.removeEventListener('focus', handleFocus)
    }
  }, [])

  useEffect(() => {
    if (!window.electronAPI) return undefined

    window.electronAPI.isWindowMaximized().then((status) => setIsMaximized(status))
    window.electronAPI.onMaximizedStateChange((isMax) => setIsMaximized(isMax))
    return () => window.electronAPI.removeMaximizedStateListener()
  }, [])

  const menuItems = [
    { key: 'mail', icon: <MailOutlined />, label: '邮件列表' },
    { key: 'quick', icon: <PlusSquareOutlined />, label: '快速创建' },
    { key: 'archive', icon: <FolderOpenOutlined />, label: '自动归档' },
    { key: 'daily', icon: <FileTextOutlined />, label: '日报生成' }
  ]

  const renderContent = () => {
    switch (activeKey) {
      case 'mail':
        return <MailList />
      case 'quick':
        return <QuickCreate />
      case 'archive':
        return <AutoArchive />
      case 'daily':
        return <DailyReport />
      case 'about':
        return <About />
      case 'settings':
        return <Settings />
      default:
        return <MailList />
    }
  }

  const getHeaderTitle = () => {
    switch (activeKey) {
      case 'mail':
        return '邮件列表'
      case 'quick':
        return '快速创建'
      case 'archive':
        return '自动归档'
      case 'daily':
        return '日报生成'
      case 'about':
        return '关于'
      case 'settings':
        return '设置'
      default:
        return 'Knot 绳结'
    }
  }

  return (
    <ConfigProvider locale={zhCN}>
      <Layout className="app-layout">
        <Sider trigger={null} collapsible collapsed={collapsed} theme="light" className="app-sider">
          <div className="logo-container">
            <div className="logo-text">{collapsed ? 'Knot' : 'Knot 绳结'}</div>
          </div>
          <Menu
            theme="light"
            mode="inline"
            defaultSelectedKeys={['mail']}
            selectedKeys={[activeKey]}
            items={menuItems}
            onClick={({ key }) => setActiveKey(key)}
          />
          <div className="sider-footer">
            <Button
              type={activeKey === 'about' ? 'primary' : 'text'}
              icon={<InfoCircleOutlined />}
              block
              onClick={() => setActiveKey('about')}
              style={{ textAlign: collapsed ? 'center' : 'left', marginBottom: 8 }}
            >
              {!collapsed && '关于'}
            </Button>
            <Button
              type={activeKey === 'settings' ? 'primary' : 'text'}
              icon={<SettingOutlined />}
              block
              onClick={() => setActiveKey('settings')}
              style={{ textAlign: collapsed ? 'center' : 'left' }}
            >
              {!collapsed && '设置'}
            </Button>
          </div>
        </Sider>

        <Layout>
          <Header
            style={{ padding: 0, background: colorBgContainer }}
            className={`app-header ${isIntegratedStyle ? 'integrated-drag' : ''}`}
          >
            <div className={`header-left ${isIntegratedStyle ? 'integrated-no-drag' : ''}`}>
              <Button
                type="text"
                icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={() => setCollapsed(!collapsed)}
                style={{ fontSize: 16, width: 64, height: 64 }}
              />
              <h2 className={`page-title ${isIntegratedStyle ? 'integrated-drag' : ''}`}>{getHeaderTitle()}</h2>
            </div>

            <div className={`header-right ${isIntegratedStyle ? 'integrated-no-drag' : ''}`}>
              {USE_MOCK && <span className={`mock-badge ${isIntegratedStyle ? 'integrated-no-drag' : ''}`}>Mock 模式</span>}

              {isIntegratedStyle && (
                <div className="window-controls integrated-no-drag">
                  <div className="window-btn" onClick={() => window.electronAPI?.minimizeWindow()} title="最小化">
                    <MinusOutlined style={{ fontSize: 12 }} />
                  </div>
                  <div className="window-btn" onClick={() => window.electronAPI?.maximizeWindow()} title={isMaximized ? '向下还原' : '最大化'}>
                    {isMaximized ? <BlockOutlined style={{ fontSize: 11 }} /> : <BorderOutlined style={{ fontSize: 11 }} />}
                  </div>
                  <div className="window-btn close" onClick={() => window.electronAPI?.closeWindow()} title="关闭">
                    <CloseOutlined style={{ fontSize: 12 }} />
                  </div>
                </div>
              )}
            </div>
          </Header>

          <Content
            style={{
              margin: '24px 16px',
              padding: 24,
              minHeight: 280,
              background: colorBgContainer,
              borderRadius: borderRadiusLG
            }}
          >
            {renderContent()}
          </Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  )
}

export default App
