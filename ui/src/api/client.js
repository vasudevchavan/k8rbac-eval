import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  // 120 s timeout — "check all resources" can be slow on large clusters.
  timeout: 120_000,
})

/**
 * Detect the cluster platform for a given kubeconfig.
 * @param {string} [kubeconfig] - optional kubeconfig path
 * @returns {{ platform: string, displayName: string, azureRbacMode?: boolean }}
 */
export async function fetchPlatform(kubeconfig = '') {
  const params = kubeconfig ? { kubeconfig } : {}
  const { data } = await api.get('/platform', { params })
  return data
}

/**
 * Fetch available kubeconfig files from the server.
 * @returns {{ files: string[], default: string }}
 */
export async function fetchKubeconfigs() {
  const { data } = await api.get('/kubeconfigs')
  return data
}

/**
 * Check RBAC access.
 * @param {{ subjectType, name, namespace, resource, clusterScope, kubeconfig }} params
 * @returns {{ output: string, error?: string }}
 */
export async function checkAccess(params) {
  const { data } = await api.post('/check', params)
  return data
}

/**
 * Generate RBAC manifests.
 * @param {{ subjectType, name, namespace, resource, verbs, clusterScope, kubeconfig }} params
 * @returns {{ output: string, error?: string }}
 */
export async function generateRBAC(params) {
  const { data } = await api.post('/generate', params)
  return data
}
