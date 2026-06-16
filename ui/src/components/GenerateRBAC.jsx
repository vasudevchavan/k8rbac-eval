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
  Checkbox,
  Button,
  CodeSnippet,
  InlineLoading,
  InlineNotification,
} from '@carbon/react'
import { DocumentAdd } from '@carbon/icons-react'
import KubeconfigSelector from './KubeconfigSelector'
import { generateRBAC } from '../api/client'

const ALL_VERBS = ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete', 'deletecollection']
const DEFAULT_VERBS = new Set(['get', 'list', 'watch'])

const BASE_RESOURCES = [
  'pods', 'deployments', 'services', 'secrets', 'configmaps',
  'namespaces', 'nodes', 'persistentvolumes', 'ingresses',
  'serviceaccounts', 'roles', 'rolebindings', 'clusterroles',
  'clusterrolebindings',
]

const PLATFORM_RESOURCES = {
  openshift: [
    'routes', 'projects', 'buildconfigs', 'imagestreams',
    'deploymentconfigs', 'securitycontextconstraints', 'operatorgroups',
  ],
  eks: [],
  aks: [],
  kubernetes: [],
}

export default function GenerateRBAC() {
  const [subjectType, setSubjectType] = useState('user')
  const [name, setName] = useState('')
  const [namespace, setNamespace] = useState('default')
  const [resource, setResource] = useState('')
  const [verbs, setVerbs] = useState(new Set(DEFAULT_VERBS))
  const [clusterScope, setClusterScope] = useState(false)
  const [kubeconfig, setKubeconfig] = useState('')
  const [platformInfo, setPlatformInfo] = useState(null)

  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState(null)
  const [formErrors, setFormErrors] = useState({})

  const handlePlatform = useCallback((info) => setPlatformInfo(info), [])

  const allResources = [
    ...BASE_RESOURCES,
    ...(PLATFORM_RESOURCES[platformInfo?.platform] ?? []),
  ]

  const toggleVerb = (verb) => {
    setVerbs((prev) => {
      const next = new Set(prev)
      next.has(verb) ? next.delete(verb) : next.add(verb)
      return next
    })
  }

  const validate = () => {
    const errs = {}
    if (!name.trim()) errs.name = 'Name is required'
    if (!resource.trim()) errs.resource = 'Resource is required'
    if (verbs.size === 0) errs.verbs = 'Select at least one verb'
    return errs
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    const errs = validate()
    setFormErrors(errs)
    if (Object.keys(errs).length) return

    setLoading(true)
    setResult(null)
    try {
      const data = await generateRBAC({
        subjectType,
        name: name.trim(),
        namespace: clusterScope ? '' : namespace,
        resource: resource.trim(),
        verbs: [...verbs],
        clusterScope,
        kubeconfig,
      })
      setResult({
        yaml: data.output,
        serverError: data.error || null,
        warnings: data.warnings || [],
      })
    } catch (err) {
      setResult({ yaml: '', serverError: err.response?.data?.error || err.message, warnings: [] })
    } finally {
      setLoading(false)
    }
  }

  const reset = () => {
    setName(''); setNamespace('default'); setResource('')
    setVerbs(new Set(DEFAULT_VERBS)); setClusterScope(false)
    setResult(null); setFormErrors({})
  }

  const copyYaml = () => {
    if (result?.yaml) navigator.clipboard.writeText(result.yaml)
  }

  const downloadYaml = () => {
    if (!result?.yaml) return
    const blob = new Blob([result.yaml], { type: 'text/yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `kubeaccess-${subjectType}-${name.trim() || 'rbac'}.yaml`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="tab-panel-inner">
      <Form onSubmit={handleSubmit}>
        <Stack gap={6}>
          {/* Kubeconfig + platform badge */}
          <KubeconfigSelector value={kubeconfig} onChange={setKubeconfig} onPlatform={handlePlatform} />

          {/* Subject type */}
          <RadioButtonGroup
            legendText="Subject type"
            name="gen-subject-type"
            valueSelected={subjectType}
            onChange={(val) => setSubjectType(val)}
            orientation="horizontal"
          >
            <RadioButton labelText="User" value="user" id="gen-type-user" />
            <RadioButton labelText="Service Account" value="sa" id="gen-type-sa" />
          </RadioButtonGroup>

          {/* Name + namespace */}
          <Grid narrow>
            <Column sm={4} md={4} lg={6}>
              <TextInput
                id="gen-name"
                labelText={subjectType === 'user' ? 'Username' : 'Service Account name'}
                placeholder={subjectType === 'user' ? 'bob' : 'monitor-sa'}
                value={name}
                onChange={(e) => setName(e.target.value)}
                invalid={!!formErrors.name}
                invalidText={formErrors.name}
              />
            </Column>
            <Column sm={4} md={4} lg={6}>
              <TextInput
                id="gen-namespace"
                labelText="Namespace"
                placeholder="default"
                value={namespace}
                onChange={(e) => setNamespace(e.target.value)}
                disabled={clusterScope}
              />
            </Column>
          </Grid>

          {/* Resource quick-pick (platform-aware) */}
          <div>
            <p className="cds--label">
              Resource <span style={{ color: 'var(--cds-support-error)' }}>*</span>
              {platformInfo?.platform === 'openshift' && (
                <span style={{ color: 'var(--cds-text-helper)', fontWeight: 400, marginLeft: '0.5rem' }}>
                  — includes OpenShift-specific resources
                </span>
              )}
            </p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '0.75rem' }}>
              {allResources.map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setResource(r)}
                  style={{
                    padding: '0.25rem 0.75rem',
                    borderRadius: '1rem',
                    border: `1px solid ${resource === r ? 'var(--cds-interactive)' : 'var(--cds-border-subtle-01)'}`,
                    background: resource === r ? 'var(--cds-interactive)' : 'var(--cds-layer-01)',
                    color: resource === r ? '#fff' : 'var(--cds-text-primary)',
                    cursor: 'pointer',
                    fontSize: '0.8rem',
                    fontFamily: 'inherit',
                    transition: 'all 0.1s',
                  }}
                >
                  {r}
                </button>
              ))}
            </div>
            <TextInput
              id="gen-resource"
              labelText=""
              hideLabel
              placeholder="or type a custom resource…"
              value={resource}
              onChange={(e) => setResource(e.target.value)}
              invalid={!!formErrors.resource}
              invalidText={formErrors.resource}
            />
          </div>

          {/* Verbs */}
          <fieldset style={{ border: 'none', padding: 0, margin: 0 }}>
            <legend className="cds--label" style={{ marginBottom: '0.5rem' }}>
              Verbs <span style={{ color: 'var(--cds-support-error)' }}>*</span>
              {formErrors.verbs && (
                <span style={{ color: 'var(--cds-support-error)', marginLeft: '0.5rem', fontWeight: 400 }}>
                  — {formErrors.verbs}
                </span>
              )}
            </legend>
            <div className="verb-checkbox-group">
              {ALL_VERBS.map((v) => (
                <Checkbox
                  key={v}
                  id={`verb-${v}`}
                  labelText={v}
                  checked={verbs.has(v)}
                  onChange={() => toggleVerb(v)}
                />
              ))}
            </div>
          </fieldset>

          {/* Cluster scope */}
          <Toggle
            id="gen-clusterscope"
            labelText="Cluster-scoped role (ClusterRole / ClusterRoleBinding)"
            labelA="Namespace-scoped"
            labelB="Cluster-scoped"
            toggled={clusterScope}
            onToggle={(v) => setClusterScope(v)}
          />

          {/* Actions */}
          <div style={{ display: 'flex', gap: '1rem' }}>
            <Button type="submit" renderIcon={DocumentAdd} disabled={loading}>
              {loading ? 'Generating…' : 'Generate Manifests'}
            </Button>
            <Button kind="secondary" onClick={reset} disabled={loading}>Reset</Button>
          </div>

          {loading && <InlineLoading description="Running kubeaccess generate…" />}
        </Stack>
      </Form>

      {/* Output */}
      {result && (
        <div className="output-panel">
          {/* Platform / cloud advisories */}
          {result.warnings?.map((w, i) => (
            <InlineNotification
              key={i}
              kind="info"
              title="Advisory"
              subtitle={w}
              lowContrast
              style={{ marginBottom: '0.5rem' }}
            />
          ))}

          {result.serverError && (
            <InlineNotification
              kind="error"
              title="Error"
              subtitle={result.serverError}
              lowContrast
            />
          )}

          {result.yaml && (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
                <p className="cds--label" style={{ margin: 0 }}>Generated YAML</p>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <Button kind="ghost" size="sm" onClick={copyYaml}>Copy</Button>
                  <Button kind="ghost" size="sm" onClick={downloadYaml}>Download .yaml</Button>
                </div>
              </div>
              <CodeSnippet type="multi" wrapText>
                {result.yaml}
              </CodeSnippet>
            </>
          )}
        </div>
      )}
    </div>
  )
}
