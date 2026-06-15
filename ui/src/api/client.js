import axios from 'axios'

const api = axios.create({ baseURL: '/api' })

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
