#!/usr/bin/env node
// Standalone mock API server — use during UI development when the Go server
// or a real cluster is not available.
// Run: node ui/mock-api.js
const http = require('http')

const KUBECONFIGS = {
  files: [
    `${process.env.HOME}/.kube/config`,
    `${process.env.HOME}/.kube/staging.yaml`,
  ],
  default: `${process.env.HOME}/.kube/config`,
}

// Multi-resource mock output matching the CLI's printAccess format:
//   resource: <name>
//     <verb>                : <true|false>
const CHECK_OUTPUT = `resource: pods
  get                : true
  list               : true
  watch              : true
  create             : false
  update             : false
  patch              : false
  delete             : false
resource: deployments
  get                : true
  list               : true
  watch              : true
  create             : false
  update             : false
  patch              : false
  delete             : false
resource: secrets
  get                : false
  list               : false
  watch              : false
  create             : false
  update             : false
  patch              : false
  delete             : false
`

const GENERATE_OUTPUT = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: alice-pods-role
  namespace: default
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: alice-pods-rolebinding
  namespace: default
subjects:
- kind: User
  name: alice
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: alice-pods-role
  apiGroup: rbac.authorization.k8s.io
`

function json(res, code, body) {
  res.writeHead(code, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
  })
  res.end(JSON.stringify(body))
}

http.createServer((req, res) => {
  if (req.method === 'OPTIONS') {
    res.writeHead(204, { 'Access-Control-Allow-Origin': '*', 'Access-Control-Allow-Methods': 'GET, POST, OPTIONS', 'Access-Control-Allow-Headers': 'Content-Type' })
    res.end()
    return
  }

  console.log(`${new Date().toISOString()}  ${req.method} ${req.url}`)

  if (req.url === '/api/health')                        return json(res, 200, { status: 'ok' })
  if (req.url === '/api/kubeconfigs')                   return json(res, 200, KUBECONFIGS)
  if (req.url.startsWith('/api/platform'))              return json(res, 200, { platform: 'kubernetes', displayName: 'Kubernetes', azureRbacMode: false })

  if (req.url === '/api/check' && req.method === 'POST') {
    let body = ''
    req.on('data', d => body += d)
    req.on('end', () => {
      const { name = 'alice', subjectType = 'user', resource = 'pods' } = JSON.parse(body || '{}')
      console.log(`  → check ${subjectType}/${name} resource=${resource}`)
      return json(res, 200, { output: CHECK_OUTPUT })
    })
    return
  }

  if (req.url === '/api/generate' && req.method === 'POST') {
    let body = ''
    req.on('data', d => body += d)
    req.on('end', () => {
      const { name = 'alice', subjectType = 'user' } = JSON.parse(body || '{}')
      console.log(`  → generate ${subjectType}/${name}`)
      return json(res, 200, { output: GENERATE_OUTPUT })
    })
    return
  }

  json(res, 404, { error: 'not found' })
}).listen(8080, () => {
  console.log('✅  Mock API server running on http://localhost:8080')
  console.log('    Proxy target for Vite dev server (:3000)')
  console.log('    Press Ctrl+C to stop\n')
})
