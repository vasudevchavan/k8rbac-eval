import React, { useState } from 'react'
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
  InlineLoading,
  InlineNotification,
  Tag,
} from '@carbon/react'
import { Search, Checkmark, Close } from '@carbon/icons-react'
import KubeconfigSelector from './KubeconfigSelector'
import { checkAccess } from '../api/client'

const COMMON_RESOURCES = [
  'pods', 'deployments', 'services', 'secrets', 'configmaps',
  'namespaces', 'nodes', 'persistentvolumes', 'ingresses',
  'serviceaccounts', 'roles', 'rolebindings', 'clusterroles',
  'clusterrolebindings',
]

/**
 * Parse the plaintext output of `kubeaccess show` into rows.
 * Each line looks like:   get    : true
 */
function parseOutput(raw) {
  const rows = []
  for (const line of raw.split('\n')) {
    const m = line.match(/^\s*(\S+)\s*:\s*(true|false)\s*$/i)
    if (m) {
      rows.push({ id: m[1], verb: m[1], allowed: m[2].toLowerCase() === 'true' })
    }
  }
  return rows
}

const headers = [
  { key: 'verb', header: 'Verb' },
  { key: 'allowed', header: 'Allowed' },
]

export default function CheckAccess() {
  const [subjectType, setSubjectType] = useState('user')
  const [name, setName] = useState('')
  const [namespace, setNamespace] = useState('default')
  const [resource, setResource] = useState('')
  const [clusterScope, setClusterScope] = useState(false)
  const [kubeconfig, setKubeconfig] = useState('')

  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState(null)   // { rows, rawOutput, serverError }
  const [formError, setFormError] = useState('')

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
      })
    } catch (err) {
      setResult({ rows: [], rawOutput: '', serverError: err.response?.data?.error || err.message })
    } finally {
      setLoading(false)
    }
  }

  const reset = () => {
    setName(''); setNamespace('default'); setResource('')
    setClusterScope(false); setResult(null); setFormError('')
  }

  return (
    <div className="tab-panel-inner">
      <Form onSubmit={handleSubmit}>
        <Stack gap={6}>
          {/* Kubeconfig */}
          <KubeconfigSelector value={kubeconfig} onChange={setKubeconfig} />

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

          {/* Name + namespace row */}
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

          {/* Resource — quick-pick chips + free-text */}
          <div>
            <p className="cds--label">Resource <span style={{ color: 'var(--cds-text-helper)' }}>(optional — blank = all)</span></p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '0.75rem' }}>
              {COMMON_RESOURCES.map((r) => (
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

          {/* Cluster scope toggle */}
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

          {loading && <InlineLoading description="Running kubeaccess show…" />}
        </Stack>
      </Form>

      {/* Results */}
      {result && (
        <div className="output-panel">
          {result.serverError && (
            <InlineNotification
              kind="error"
              title="Error"
              subtitle={result.serverError}
              lowContrast
            />
          )}

          {result.rows.length > 0 ? (
            <div className="results-table-wrapper">
              <DataTable rows={result.rows} headers={headers} isSortable>
                {({ rows, headers, getTableProps, getHeaderProps, getRowProps }) => (
                  <Table {...getTableProps()} size="sm">
                    <TableHead>
                      <TableRow>
                        {headers.map((h) => (
                          <TableHeader key={h.key} {...getHeaderProps({ header: h })}>
                            {h.header}
                          </TableHeader>
                        ))}
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {rows.map((row) => {
                        const allowedCell = row.cells.find((c) => c.info.header === 'allowed')
                        const isAllowed = result.rows.find((r) => r.id === row.id)?.allowed
                        return (
                          <TableRow key={row.id} {...getRowProps({ row })}>
                            {row.cells.map((cell) => (
                              <TableCell key={cell.id}>
                                {cell.info.header === 'allowed' ? (
                                  <span className={`access-badge access-badge--${isAllowed ? 'allowed' : 'denied'}`}>
                                    {isAllowed
                                      ? <><Checkmark size={16} /> Allowed</>
                                      : <><Close size={16} /> Denied</>}
                                  </span>
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
                )}
              </DataTable>
            </div>
          ) : (
            !result.serverError && (
              <InlineNotification
                kind="info"
                title="No results"
                subtitle="No access data returned. Try a different resource or subject."
                lowContrast
                hideCloseButton
              />
            )
          )}

          {/* Raw output collapsible */}
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
