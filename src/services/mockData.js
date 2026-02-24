// Mock 数据 - 用于外网开发测试
import { getSettings } from './settings'

export const mockMails = [
  {
    id: '1',
    subject: '关于2025年度预算审批的通知',
    from: '财务部 <caiwu@company.com>',
    date: '2025-01-19 10:30:00',
    body: `各部门：

根据公司年度工作计划，现将2025年度预算审批工作安排通知如下：

一、预算编制要求
1. 各部门需按照统一模板编制2025年度预算
2. 预算编制应遵循"量入为出、统筹兼顾"的原则
3. 重点项目需附详细说明

二、时间安排
1. 1月20日前：各部门提交预算初稿
2. 1月25日前：财务部汇总审核
3. 2月1日前：提交公司审批

请各部门按时完成预算编制工作。

附件：2025年度预算表、预算说明

财务部
2025年1月19日`,
    attachment_count: 2,
    attachments: [
      { filename: '2025年度预算表.xlsx', size: 156000, content_type: 'application/vnd.ms-excel' },
      { filename: '预算说明.docx', size: 45000, content_type: 'application/msword' }
    ]
  },
  {
    id: '2',
    subject: '科数部系统升级方案讨论',
    from: '科数部 张三 <zhangsan@company.com>',
    date: '2025-01-18 14:20:00',
    body: `各位同事：

关于数字化办公平台系统升级方案，现将讨论要点整理如下：

1. 升级目标
   - 提升系统响应速度30%以上
   - 优化用户界面体验
   - 增加移动端适配

2. 技术方案
   - 前端：升级至React 18
   - 后端：优化数据库查询
   - 部署：采用容器化方案

3. 实施计划
   - 第一阶段：环境准备（1周）
   - 第二阶段：开发测试（2周）
   - 第三阶段：上线部署（3天）

请查阅附件中的详细方案，如有问题请及时反馈。

张三
科数部`,
    attachment_count: 1,
    attachments: [
      { filename: '系统升级方案v2.pdf', size: 2340000, content_type: 'application/pdf' }
    ]
  },
  {
    id: '3',
    subject: '本周工作周报模板更新',
    from: '办公室 <office@company.com>',
    date: '2025-01-17 09:00:00',
    body: `各部门：

为规范周报填写，现对周报模板进行更新，主要变更如下：

1. 新增"下周计划"栏目
2. 优化"本周完成"格式
3. 增加项目进度跟踪表

请各部门自本周起使用新模板提交周报。

办公室
2025年1月17日`,
    attachment_count: 1,
    attachments: [
      { filename: '周报模板2025.docx', size: 32000, content_type: 'application/msword' }
    ]
  },
  {
    id: '4',
    subject: '关于组织部门团建活动的通知',
    from: '人事部 <hr@company.com>',
    date: '2025-01-16 16:45:00',
    body: `各位同事：

为增进部门间交流，丰富员工文化生活，公司定于2025年2月举办团建活动。

活动安排：
- 时间：2025年2月15日（周六）
- 地点：待定
- 形式：户外拓展 + 聚餐

请各部门于1月25日前统计参加人数并报送人事部。

人事部
2025年1月16日`,
    attachment_count: 0,
    attachments: []
  },
  {
    id: '5',
    subject: '信息安全培训材料',
    from: '信息中心 <it@company.com>',
    date: '2025-01-15 11:30:00',
    body: `各位同事：

根据公司信息安全管理要求，现发布信息安全培训材料，请认真学习。

培训内容包括：
1. 网络安全基础知识
2. 密码安全管理
3. 钓鱼邮件识别
4. 数据保护规范

培训完成后请参加在线测试，测试成绩将计入年度考核。

信息中心
2025年1月15日`,
    attachment_count: 3,
    attachments: [
      { filename: '信息安全培训PPT.pptx', size: 5600000, content_type: 'application/vnd.ms-powerpoint' },
      { filename: '安全知识测试题.docx', size: 28000, content_type: 'application/msword' },
      { filename: '网络安全手册.pdf', size: 1200000, content_type: 'application/pdf' }
    ]
  },
  {
    id: '6',
    subject: '项目进度汇报 - 数字化办公平台',
    from: '项目组 李四 <lisi@company.com>',
    date: '2025-01-14 15:00:00',
    body: `项目组成员：

本周项目进度汇报如下：

已完成工作：
1. 用户管理模块开发完成
2. 权限系统联调通过
3. 前端界面优化

进行中工作：
1. 报表模块开发（60%）
2. 接口文档编写

下周计划：
1. 完成报表模块
2. 开始集成测试

详见附件。

李四
项目组`,
    attachment_count: 2,
    attachments: [
      { filename: '项目进度表.xlsx', size: 89000, content_type: 'application/vnd.ms-excel' },
      { filename: '需求变更说明.docx', size: 56000, content_type: 'application/msword' }
    ]
  },
  {
    id: '7',
    subject: '会议纪要：部门协调会',
    from: '综合部 <zonghe@company.com>',
    date: '2025-01-13 17:20:00',
    body: `各部门负责人：

现将1月13日部门协调会会议纪要发送如下：

会议主题：2025年第一季度工作协调

主要议题：
1. 各部门Q1工作计划汇报
2. 跨部门协作事项确认
3. 资源调配讨论

会议决议：
1. 各部门于1月20日前提交详细计划
2. 建立周例会制度
3. 设立项目协调专员

综合部
2025年1月13日`,
    attachment_count: 1,
    attachments: [
      { filename: '会议纪要20250113.docx', size: 34000, content_type: 'application/msword' }
    ]
  },
  {
    id: '8',
    subject: '新员工入职培训安排',
    from: '人事部 <hr@company.com>',
    date: '2025-01-12 09:30:00',
    body: `各位新同事：

欢迎加入公司！现将入职培训安排通知如下：

培训时间：2025年1月20日-22日
培训地点：3楼培训室

培训内容：
- 公司文化与制度
- 业务流程介绍
- 办公系统使用
- 安全生产培训

请准时参加，如有问题请联系人事部。

人事部
2025年1月12日`,
    attachment_count: 0,
    attachments: []
  }
]

// Mock API 响应
export const mockApi = {
  connect: async (config) => {
    await new Promise(resolve => setTimeout(resolve, 800))
    return { success: true, message: '连接成功（Mock模式）' }
  },

  getMailList: async () => {
    await new Promise(resolve => setTimeout(resolve, 500))
    return { success: true, data: mockMails }
  },

  getAttachments: async (mailId) => {
    await new Promise(resolve => setTimeout(resolve, 300))
    const mail = mockMails.find(m => m.id === mailId)
    return { success: true, data: mail?.attachments || [] }
  }
}
