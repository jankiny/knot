import { useState, useEffect, useMemo } from 'react'
import { ConfigProvider, Layout, Menu, Button, theme } from 'antd'
import {
  MailOutlined,
  FolderOpenOutlined,
  InfoCircleOutlined,
  SettingOutlined,
  MenuUnfoldOutlined,
  MenuFoldOutlined,
  PlusSquareOutlined,
  MinusOutlined,
  BorderOutlined,
  BlockOutlined,
  CloseOutlined
} from '@ant-design/icons'
import zhCN from 'antd/locale/zh_CN'
import MailList from './components/MailList'
import AutoArchive from './components/AutoArchive'
import Settings from './components/Settings'
import About from './components/About'
import QuickCreate from './components/QuickCreate'
import { getSettings } from './services/settings'
import { USE_MOCK } from './services/api'
import './App.css'

const { Header, Sider, Content } = Layout

function App() {
  const [collapsed, setCollapsed] = useState(false)
  const [activeKey, setActiveKey] = useState('mail')
  const [isMaximized, setIsMaximized] = useState(false)
  const [settings, setSettings] = useState(getSettings())

  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken()

  const isIntegratedStyle = settings.windowStyle === 'integrated'

  // 监听打开设置的事件
  useEffect(() => {
    const handleOpenSettings = () => setActiveKey('settings')
    window.addEventListener('openSettings', handleOpenSettings)

    // 我们也可能需要监听设置的更改以便实时更新 windowStyle，
    // 这里简单处理为每次重新渲染或切回前台时重新获取
    const handleFocus = () => setSettings(getSettings())
    window.addEventListener('focus', handleFocus)

    return () => {
      window.removeEventListener('openSettings', handleOpenSettings)
      window.removeEventListener('focus', handleFocus)
    }
  }, [])

  // 监听窗口最大化状态
  useEffect(() => {
    if (window.electronAPI) {
      window.electronAPI.isWindowMaximized().then(status => setIsMaximized(status))
      window.electronAPI.onMaximizedStateChange((isMax) => {
        setIsMaximized(isMax)
      })
      return () => {
        window.electronAPI.removeMaximizedStateListener()
      }
    }
  }, [])

  const handleMinimize = () => {
    if (window.electronAPI) window.electronAPI.minimizeWindow()
  }

  const handleMaximize = () => {
    if (window.electronAPI) window.electronAPI.maximizeWindow()
  }

  const handleClose = () => {
    if (window.electronAPI) window.electronAPI.closeWindow()
  }

  const menuItems = [
    {
      key: 'mail',
      icon: <MailOutlined />,
      label: '邮件列表',
    },
    {
      key: 'quick',
      icon: <PlusSquareOutlined />,
      label: '快速创建',
    },
    {
      key: 'archive',
      icon: <FolderOpenOutlined />,
      label: '自动归档',
    },
  ]

  const renderContent = () => {
    switch (activeKey) {
      case 'mail':
        return <MailList />
      case 'quick':
        return <QuickCreate />
      case 'archive':
        return <AutoArchive />
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
      case 'mail': return '邮件列表'
      case 'quick': return '快速创建'
      case 'archive': return '自动归档'
      case 'about': return '关于'
      case 'settings': return '设置'
      default: return 'Knot 绳结'
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
          <Header style={{ padding: 0, background: colorBgContainer }} className={`app-header ${isIntegratedStyle ? 'integrated-drag' : ''}`}>
            <div className={`header-left ${isIntegratedStyle ? 'integrated-no-drag' : ''}`}>
              <Button
                type="text"
                icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={() => setCollapsed(!collapsed)}
                style={{
                  fontSize: '16px',
                  width: 64,
                  height: 64,
                }}
              />
              <h2 className={`page-title ${isIntegratedStyle ? 'integrated-drag' : ''}`}>{getHeaderTitle()}</h2>
            </div>
            <div className={`header-right ${isIntegratedStyle ? 'integrated-no-drag' : ''}`}>
              {USE_MOCK && <span className={`mock-badge ${isIntegratedStyle ? 'integrated-no-drag' : ''}`}>Mock 模式</span>}

              {isIntegratedStyle && (
                <div className="window-controls integrated-no-drag">
                  <div className="window-btn" onClick={handleMinimize} title="最小化">
                    <MinusOutlined style={{ fontSize: '12px' }} />
                  </div>
                  <div className="window-btn" onClick={handleMaximize} title={isMaximized ? "向下还原" : "最大化"}>
                    {isMaximized ? (
                      <BlockOutlined style={{ fontSize: '11px' }} />
                    ) : (
                      <BorderOutlined style={{ fontSize: '11px' }} />
                    )}
                  </div>
                  <div className="window-btn close" onClick={handleClose} title="关闭">
                    <CloseOutlined style={{ fontSize: '12px' }} />
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
    </ConfigProvider >
  )
}

export default App
