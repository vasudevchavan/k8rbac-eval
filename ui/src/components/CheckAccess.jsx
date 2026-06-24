import React, { useState, useCallback } from 'react'
import {
  Form,
  Stack,
  Grid,
  Column,
  RadioButtonGroup,
  RadioButton,
  TextInput,
  Toggle,
  Button,
  DataTable,
  Table,
  TableHead,
  TableRow,
  TableHeader,
  TableBody,
  TableCell,
  TableToolbar,
  TableToolbarContent,
  TableToolbarSearch,
  InlineLoading,
  InlineNotification,
  Tag,
} from '@carbon/react'
import { Search, Checkmark, Close } from '@carbon/icons-react'
import KubeconfigSelector from './KubeconfigSelector'
import { checkAccess } from '../api/client'

// Base resources shown on all platforms
const BASE_RESOURCES = [
  'pods', 'deployments', 'services', 'secrets', 'configmaps',
  'namespaces', 'nodes', 'persistentvolumes', 'ingresses',
  'serviceaccounts', 'roles', 'rolebindings', 'clusterroles',
  'clusterrolebindings',
]

// Extra resources shown when a specific platform is detected
const PLATFORM_RESOURCES = {
  openshift: [
    'routes', 'projects', 'buildconfigs', 'imagestreams',
    'deploymentconfigs', 'securitycontextconstraints', 'operatorgroups',
  ],
  eks: [], // standard k8s resources; IRSA is IAM-level, not k8s
  aks: [], // standard k8s resources; Azure RBAC is Azure-level
  kubernetes: [],
}

/**
 * Parse the stdout from `kubeaccess show`.
 *
 * CLI output format (one block per resource):
 *   resource: pods
 *     get                : true
 *     list               : true
 *     ...
 *
 * Returns an array of { id, resource, verb, allowed }.
 */
function parseOutput(raw) {
  const rows = []
  let currentResource = ''
  let rowIdx = 0

  for (const line of raw.split('\n')) {
    // "resource: pods"
    const resMatch = line.match(/^resource:\s*(\S+)/)
    if (resMatch) {
      currentResource = resMatch[1]
      continue
    }

    // "  get                : true"
    const verbMatch = line.match(/^\s*(\w+)\s*:\s*(true|false)\s*$/i)
    if (verbMatch) {
      rows.push({
        id: String(rowIdx++),
        resource: currentResource,
        verb: verbMatch[1],
        allowed: verbMatch[2].toLowerCase() === 'true',
      })
    }
  }

  return rows
}

/** True when the result set spans more than one resource */
function isMultiResource(rows) {
  const resources = new Set(rows.map((r) => r.resource).filter(Boolean))
  return resources.size > 1
}

function AllowedBadge({ allowed }) {
  return (
    <span className={`access-badge access-badge--${allowed ? 'allowed' : 'denied'}`}>
      {allowed ? <><Checkmark size={16} /> Allowed</> : <><Close size={16} /> Denied</>}
    </span>
  )
}

/** Single-resource table: Verb | Allowed */
function SingleResourceTable({ rows }) {
  const headers = [
    { key: 'verb', header: 'Verb' },
    { key: 'allowed', header: 'Allowed' },
  ]
  return (
    <DataTable rows={rows} headers={headers} isSortable>
      {({ rows: tRows, headers, getTableProps, getHeaderProps, getRowProps }) => (
        <Table {...getTableProps()} size="sm">
          <TableHead>
            <TableRow>
              {headers.map((h) => (
                <TableHeader key={h.key} {...getHeaderProps({ header: h })}>{h.header}</TableHeader>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {tRows.map((row) => {
              const original = rows.find((r) => r.id === row.id)
              return (
                <TableRow key={row.id} {...getRowProps({ row })}>
                  {row.cells.map((cell) => (
                    <TableCell key={cell.id}>
                      {cell.info.header === 'allowed'
                        ? <AllowedBadge allowed={original?.allowed} />
                        : <code>{cell.value}</code>}
                    </TableCell>
                  ))}
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </DataTable>
  )
}

/** Multi-resource table: Resource | Verb | Allowed  (with toolbar search) */
function MultiResourceTable({ rows }) {
  const [filter, setFilter] = useState('')

  const headers = [
    { key: 'resource', header: 'Resource' },
    { key: 'verb', header: 'Verb' },
    { key: 'allowed', header: 'Allowed' },
  ]

  const filtered = filter
    ? rows.filter(
        (r) =>
          r.resource.toLowerCase().includes(filter.toLowerCase()) ||
          r.verb.toLowerCase().includes(filter.toLowerCase()),
      )
    : rows

  return (
    <DataTable rows={filtered} headers={headers} isSortable>
      {({ rows: tRows, headers, getTableProps, getHeaderProps, getRowProps, getToolbarProps }) => (
        <>
          <TableToolbar {...getToolbarProps()}>
            <TableToolbarContent>
              <TableToolbarSearch
                placeholder="Filter resource or verb…"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                persistent
              />
            </TableToolbarContent>
          </TableToolbar>
          <Table {...getTableProps()} size="sm">
            <TableHead>
              <TableRow>
                {headers.map((h) => (
                  <TableHeader key={h.key} {...getHeaderProps({ header: h })}>{h.header}</TableHeader>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {tRows.map((row) => {
                const original = filtered.find((r) => r.id === row.id)
                // Detect first row of each resource group to render a divider
                const rowIdx = tRows.indexOf(row)
                const prevRow = rowIdx > 0 ? tRows[rowIdx - 1] : null
                const prevOriginal = prevRow ? filtered.find((r) => r.id === prevRow.id) : null
                const isNewResource = !prevOriginal || prevOriginal.resource !== original?.resource

                return (
                  <TableRow
                    key={row.id}
                    {...getRowProps({ row })}
                    style={isNewResource && rowIdx > 0
                      ? { borderTop: '2px solid var(--cds-border-subtle-01)' }
                      : {}}
                  >
                    {row.cells.map((cell) => (
                      <TableCell key={cell.id}>
                        {cell.info.header === 'allowed' ? (
                          <AllowedBadge allowed={original?.allowed} />
                        ) : cell.info.header === 'resource' ? (
                          // Only show resource name on the first row of each group
                          isNewResource
                            ? <Tag type="blue" size="sm">{cell.value}</Tag>
                            : null
                        ) : (
                          <code>{cell.value}</code>
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </>
      )}
    </DataTable>
  )
}

export default function CheckAccess() {
  const [subjectType, setSubjectType] = useState('user')
  const [name, setName] = useState('')
  const [namespace, setNamespace] = useState('default')
  const [resource, setResource] = useState('')
  const [clusterScope, setClusterScope] = useState(false)
  const [kubeconfig, setKubeconfig] = useState('')
  const [platformInfo, setPlatformInfo] = useState(null)

  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState(null)
  const [formError, setFormError] = useState('')

  const handlePlatform = useCallback((info) => setPlatformInfo(info), [])

  // Build the full resource list for the current platform
  const allResources = [
    ...BASE_RESOURCES,
    ...(PLATFORM_RESOURCES[platformInfo?.platform] ?? []),
  ]

  const handleSubmit = async (e) => {
    e.preventDefault()
    setFormError('')
    if (!name.trim()) { setFormError('Name is required'); return }

    setLoading(true)
    setResult(null)
    try {
      const data = await checkAccess({
        subjectType,
        name: name.trim(),
        namespace: clusterScope ? '' : namespace,
        resource: resource.trim(),
        clusterScope,
        kubeconfig,
      })
      setResult({
        rows: parseOutput(data.output),
        rawOutput: data.output,
        serverError: data.error || null,
        warnings: data.warnings || [],
      })
    } catch (err) {
      const msg = err.response?.data?.error || err.message
      if (err.response?.status === 400 && msg.includes('not found')) {
        setFormError(msg)
        setResult(null)
      } else {
        setResult({ rows: [], rawOutput: '', serverError: msg })
      }
    } finally {
      setLoading(false)
    }
  }

  const reset = () => {
    setName(''); setNamespace('default'); setResource('')
    setClusterScope(false); setResult(null); setFormError('')
  }

  const multiResource = result ? isMultiResource(result.rows) : false

  return (
    <div className="tab-panel-inner">
      <Form onSubmit={handleSubmit}>
        <Stack gap={6}>
          {/* Kubeconfig + platform badge */}
          <KubeconfigSelector value={kubeconfig} onChange={setKubeconfig} onPlatform={handlePlatform} />

          {/* Subject type */}
          <RadioButtonGroup
            legendText="Subject type"
            name="subject-type"
            valueSelected={subjectType}
            onChange={(val) => setSubjectType(val)}
            orientation="horizontal"
          >
            <RadioButton labelText="User" value="user" id="check-type-user" />
            <RadioButton labelText="Service Account" value="sa" id="check-type-sa" />
          </RadioButtonGroup>

          {/* Name + namespace */}
          <Grid narrow>
            <Column sm={4} md={4} lg={6}>
              <TextInput
                id="check-name"
                labelText={subjectType === 'user' ? 'Username' : 'Service Account name'}
                placeholder={subjectType === 'user' ? 'alice' : 'my-app'}
                value={name}
                onChange={(e) => setName(e.target.value)}
                invalid={!!formError}
                invalidText={formError}
                required
              />
            </Column>
            <Column sm={4} md={4} lg={6}>
              <TextInput
                id="check-namespace"
                labelText="Namespace"
                placeholder="default"
                value={namespace}
                onChange={(e) => setNamespace(e.target.value)}
                disabled={clusterScope}
              />
            </Column>
          </Grid>

          {/* Resource quick-pick */}
          <div>
            <p className="cds--label">
              Resource <span style={{ color: 'var(--cds-text-helper)' }}>(optional — blank = all resources)</span>
            </p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '0.75rem' }}>
              {allResources.map((r) => (
                <Tag
                  key={r}
                  type={resource === r ? 'blue' : 'gray'}
                  style={{ cursor: 'pointer' }}
                  onClick={() => setResource(resource === r ? '' : r)}
                >
                  {r}
                </Tag>
              ))}
            </div>
            <TextInput
              id="check-resource"
              labelText=""
              hideLabel
              placeholder="or type a custom resource…"
              value={resource}
              onChange={(e) => setResource(e.target.value)}
            />
          </div>

          {/* Cluster scope */}
          <Toggle
            id="check-clusterscope"
            labelText="Cluster-scoped check"
            labelA="Namespace"
            labelB="Cluster"
            toggled={clusterScope}
            onToggle={(v) => setClusterScope(v)}
          />

          {/* Actions */}
          <div style={{ display: 'flex', gap: '1rem' }}>
            <Button type="submit" renderIcon={Search} disabled={loading}>
              {loading ? 'Checking…' : 'Check Access'}
            </Button>
            <Button kind="secondary" onClick={reset} disabled={loading}>Reset</Button>
          </div>

          {loading && (
            <InlineLoading description={
              resource
                ? `Checking ${resource} permissions…`
                : 'Checking all resource permissions (this may take a moment)…'
            } />
          )}
        </Stack>
      </Form>

      {/* Results */}
      {result && (
        <div className="output-panel">
          {/* Cloud identity / platform warnings (IRSA, Workload Identity, Azure RBAC) */}
          {result.warnings?.map((w, i) => (
            <InlineNotification
              key={i}
              kind="warning"
              title="Advisory"
              subtitle={w}
              lowContrast
              style={{ marginBottom: '0.5rem' }}
            />
          ))}

          {result.serverError && (
            <InlineNotification
              kind="error"
              title="Access check failed"
              subtitle={result.serverError}
              lowContrast
            />
          )}

          {result.rows.length > 0 && (
            <>
              {multiResource && (
                <p style={{ fontSize: '0.875rem', color: 'var(--cds-text-secondary)', marginBottom: '0.75rem' }}>
                  Showing permissions across <strong>{new Set(result.rows.map(r => r.resource)).size}</strong> resources
                  — {result.rows.filter(r => r.allowed).length} allowed,{' '}
                  {result.rows.filter(r => !r.allowed).length} denied.
                  Use the search box to filter.
                </p>
              )}

              <div className="results-table-wrapper">
                {multiResource
                  ? <MultiResourceTable rows={result.rows} />
                  : <SingleResourceTable rows={result.rows} />}
              </div>
            </>
          )}

          {result.rows.length === 0 && !result.serverError && (
            <InlineNotification
              kind="info"
              title="No results"
              subtitle="No access data returned. The subject may have no permissions, or try a different resource."
              lowContrast
              hideCloseButton
            />
          )}

          {/* Raw output */}
          {result.rawOutput && (
            <details style={{ marginTop: '1rem' }}>
              <summary style={{ cursor: 'pointer', color: 'var(--cds-link-primary)', fontSize: '0.875rem' }}>
                Raw CLI output
              </summary>
              <pre style={{
                background: 'var(--cds-layer-02)',
                padding: '1rem',
                borderRadius: '4px',
                overflowX: 'auto',
                fontSize: '0.8rem',
                marginTop: '0.5rem',
              }}>
                {result.rawOutput}
              </pre>
            </details>
          )}
        </div>
      )}
    </div>
  )
}
