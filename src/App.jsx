import { useEffect, useState } from 'react'
import { Button, ConfigProvider, Layout, Menu, theme } from 'antd'
import {
  BlockOutlined,
  BorderOutlined,
  CloseOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  InboxOutlined,
  InfoCircleOutlined,
  MailOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MinusOutlined,
  PlusSquareOutlined,
  SolutionOutlined,
  SettingOutlined
} from '@ant-design/icons'
import zhCN from 'antd/locale/zh_CN'
import About from './components/About'
import ArchiveAiSearch from './components/ArchiveAiSearch'
import AutoArchive from './components/AutoArchive'
import ArchiveManager from './components/ArchiveManager'
import MailList from './components/MailList'
import PersonalSummary from './components/PersonalSummary'
import QuickCreate from './components/QuickCreate'
import Settings from './components/Settings'
import WorkReport from './components/WorkReport'
import { USE_MOCK } from './services/api'
import { getSettings } from './services/settings'
import { importLegacySourceRoots } from './services/sourceRoots'
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
  const appTitle = settings.appTitleLanguage === 'zh' ? '绳结' : 'Knot'

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
    importLegacySourceRoots(settings).catch((error) => {
      console.warn('资料源 legacy 导入暂未完成:', error?.message || error)
    })
  // Legacy paths are imported once at startup. Settings performs a debounced
  // import while these path settings are being edited.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!window.electronAPI) return undefined

    window.electronAPI.isWindowMaximized().then((status) => setIsMaximized(status))
    const unsubscribe = window.electronAPI.onMaximizedStateChange((isMax) => setIsMaximized(isMax))
    return () => {
      if (typeof unsubscribe === 'function') {
        unsubscribe()
      }
    }
  }, [])

  const menuItems = [
    { key: 'mail', icon: <MailOutlined />, label: '邮件列表' },
    { key: 'quick', icon: <PlusSquareOutlined />, label: '快速创建' },
    { key: 'archive', icon: <FolderOpenOutlined />, label: '当前工作' },
    { key: 'archive-manager', icon: <InboxOutlined />, label: '归档管理' },
    { key: 'archive-ai-search', icon: <FileSearchOutlined />, label: '资料检索' },
    { key: 'work-report', icon: <FileTextOutlined />, label: '工作报告' },
    { key: 'personal-summary', icon: <SolutionOutlined />, label: '个人总结' }
  ]

  const renderContent = () => {
    switch (activeKey) {
      case 'mail':
        return <MailList />
      case 'quick':
        return <QuickCreate />
      case 'archive':
        return <AutoArchive />
      case 'archive-manager':
        return <ArchiveManager />
      case 'archive-ai-search':
        return <ArchiveAiSearch />
      case 'work-report':
        return <WorkReport />
      case 'personal-summary':
        return <PersonalSummary />
      case 'about':
        return <About appTitle={appTitle} developerMode={settings.developerMode === true} />
      case 'settings':
        return <Settings onSettingsChange={setSettings} />
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
        return '当前工作'
      case 'archive-manager':
        return '归档管理'
      case 'archive-ai-search':
        return '资料检索'
      case 'work-report':
        return '工作报告'
      case 'personal-summary':
        return '个人总结'
      case 'about':
        return '关于'
      case 'settings':
        return '设置'
      default:
        return 'Knot'
    }
  }

  return (
    <ConfigProvider locale={zhCN}>
      <Layout className="app-layout">
        <Sider trigger={null} collapsible collapsed={collapsed} theme="light" className="app-sider">
          <div className="logo-container">
            <div className="logo-text">{appTitle}</div>
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
