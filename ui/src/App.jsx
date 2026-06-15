import React, { useState } from 'react'
import {
  Header,
  HeaderName,
  HeaderNavigation,
  HeaderMenuItem,
  SkipToContent,
  Content,
  Tabs,
  Tab,
  TabList,
  TabPanels,
  TabPanel,
  Theme,
} from '@carbon/react'
import { AssemblyCluster, DeploymentPolicy } from '@carbon/icons-react'
import CheckAccess from './components/CheckAccess'
import GenerateRBAC from './components/GenerateRBAC'

export default function App() {
  const [theme, setTheme] = useState('white') // 'white' | 'g10' | 'g90' | 'g100'

  return (
    <Theme theme={theme}>
      <Header aria-label="KubeAccess UI">
        <SkipToContent />
        <HeaderName href="/" prefix="">
          <AssemblyCluster size={20} style={{ marginRight: '0.5rem', verticalAlign: 'middle' }} />
          KubeAccess
        </HeaderName>

        <HeaderNavigation aria-label="Theme">
          <HeaderMenuItem
            isActive={theme === 'white'}
            onClick={() => setTheme('white')}
          >
            Light
          </HeaderMenuItem>
          <HeaderMenuItem
            isActive={theme === 'g100'}
            onClick={() => setTheme('g100')}
          >
            Dark
          </HeaderMenuItem>
        </HeaderNavigation>
      </Header>

      <Content style={{ paddingTop: '3rem' }}>
        <div className="kubeaccess-content">
          <h1 style={{ marginBottom: '0.25rem' }}>Kubernetes RBAC Inspector</h1>
          <p style={{ color: 'var(--cds-text-secondary)', marginBottom: '2rem' }}>
            Inspect access levels and generate RBAC manifests for users and service accounts.
          </p>

          <Tabs>
            <TabList aria-label="KubeAccess actions">
              <Tab renderIcon={AssemblyCluster}>Check Access</Tab>
              <Tab renderIcon={DeploymentPolicy}>Generate RBAC</Tab>
            </TabList>

            <TabPanels>
              <TabPanel>
                <CheckAccess />
              </TabPanel>
              <TabPanel>
                <GenerateRBAC />
              </TabPanel>
            </TabPanels>
          </Tabs>
        </div>
      </Content>
    </Theme>
  )
}
