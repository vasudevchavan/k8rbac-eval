import React, { useEffect, useState } from 'react'
import { Select, SelectItem, SelectSkeleton, InlineNotification, TextInput } from '@carbon/react'
import { fetchKubeconfigs } from '../api/client'

/**
 * Dropdown to pick a kubeconfig file, with a manual-entry fallback.
 *
 * Props:
 *   value        – current kubeconfig path (string)
 *   onChange     – callback(newPath: string)
 */
export default function KubeconfigSelector({ value, onChange }) {
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [custom, setCustom] = useState(false)

  useEffect(() => {
    fetchKubeconfigs()
      .then(({ files, default: def }) => {
        setOptions(files)
        if (!value && def) onChange(def)
      })
      .catch(() => {
        setError('Could not list kubeconfig files — enter a path manually.')
        setCustom(true)
      })
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <SelectSkeleton />

  return (
    <div>
      {error && (
        <InlineNotification
          kind="warning"
          title="Kubeconfig"
          subtitle={error}
          lowContrast
          hideCloseButton
          style={{ marginBottom: '0.75rem' }}
        />
      )}

      {!custom && options.length > 0 ? (
        <Select
          id="kubeconfig-select"
          labelText="Kubeconfig file"
          helperText="Select a kubeconfig or choose 'Custom path…'"
          value={value}
          onChange={(e) => {
            if (e.target.value === '__custom__') {
              setCustom(true)
              onChange('')
            } else {
              onChange(e.target.value)
            }
          }}
        >
          {options.map((f) => (
            <SelectItem key={f} value={f} text={f} />
          ))}
          <SelectItem value="__custom__" text="Custom path…" />
        </Select>
      ) : (
        <TextInput
          id="kubeconfig-custom"
          labelText="Kubeconfig file path"
          helperText="Leave blank to use the default (~/.kube/config)"
          placeholder="/home/user/.kube/my-cluster.yaml"
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      )}

      {!custom && options.length > 0 && (
        <button
          type="button"
          style={{ all: 'unset', color: 'var(--cds-link-primary)', cursor: 'pointer', fontSize: '0.75rem', marginTop: '0.25rem', display: 'inline-block' }}
          onClick={() => { setCustom(true); onChange('') }}
        >
          Enter custom path instead
        </button>
      )}
      {custom && options.length > 0 && (
        <button
          type="button"
          style={{ all: 'unset', color: 'var(--cds-link-primary)', cursor: 'pointer', fontSize: '0.75rem', marginTop: '0.25rem', display: 'inline-block' }}
          onClick={() => { setCustom(false); onChange(options[0]) }}
        >
          ← Back to detected files
        </button>
      )}
    </div>
  )
}
