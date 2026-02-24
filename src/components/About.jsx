import React from 'react';
import { Card, Typography, Divider, Space } from 'antd';
import { GithubOutlined, MailOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text, Link } = Typography;

// 获取版本号：优先从 Electron API 获取，否则显示开发版本
const getVersion = () => {
  if (typeof window !== 'undefined' && window.electronAPI?.version) {
    return `v${window.electronAPI.version}`;
  }
  return 'dev';
};

const About = () => {
  return (
    <div className="about-container" style={{ padding: '24px', maxWidth: '800px', margin: '0 auto' }}>
      <Card bordered={false} style={{ boxShadow: '0 1px 2px rgba(0,0,0,0.05)' }}>
        <div style={{ textAlign: 'center', marginBottom: '32px' }}>
          <Title level={2}>Knot 绳结</Title>
          <Paragraph type="secondary" style={{ fontSize: '16px' }}>
            邮件驱动的工作流管理工具
          </Paragraph>
        </div>

        <Typography>
          <Title level={4}>关于项目</Title>
          <Paragraph>
            Knot（绳结）是一个办公数字化工具，旨在简化邮件处理与工作归档流程。
            它通过将邮件转化为规范化的工作文件夹，帮助用户更高效地管理日常任务。
          </Paragraph>

          <Title level={4}>核心功能</Title>
          <Paragraph>
            <ul>
              <li>
                <Text strong>邮件浏览</Text>：直观查看邮件列表，支持附件预览。
              </li>
              <li>
                <Text strong>一键生成</Text>：基于邮件内容快速生成符合命名规范的工作文件夹。
              </li>
              <li>
                <Text strong>自动归档</Text>：按部门结构自动归档已完成的工作文件夹。
              </li>
            </ul>
          </Paragraph>

          <Divider />

          <Title level={4}>版本信息</Title>
          <Space direction="vertical">
            <Text>当前版本：{getVersion()}</Text>
            <Text>构建时间：{new Date().toLocaleDateString()}</Text>
          </Space>
        </Typography>
      </Card>
    </div>
  );
};

export default About;
