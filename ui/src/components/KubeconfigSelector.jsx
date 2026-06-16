import React, { useEffect, useState, useCallback } from 'react'
import { Select, SelectItem, SelectSkeleton, InlineNotification, TextInput, Tag } from '@carbon/react'
import { fetchKubeconfigs, fetchPlatform } from '../api/client'

// Map platform strings to Carbon Tag types
const PLATFORM_TAG = {
  openshift: { type: 'red',    label: 'OpenShift' },
  eks:       { type: 'yellow', label: 'EKS (AWS)' },
  aks:       { type: 'blue',   label: 'AKS (Azure)' },
  kubernetes:{ type: 'teal',   label: 'Kubernetes' },
}

/**
 * Dropdown to pick a kubeconfig file, with a manual-entry fallback.
 * Shows a platform badge next to the selector once the cluster is detected.
 *
 * Props:
 *   value        – current kubeconfig path (string)
 *   onChange     – callback(newPath: string)
 *   onPlatform   – optional callback({ platform, displayName, azureRbacMode })
 */
export default function KubeconfigSelector({ value, onChange, onPlatform }) {
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [custom, setCustom] = useState(false)
  const [platformInfo, setPlatformInfo] = useState(null)
  const [platformLoading, setPlatformLoading] = useState(false)

  // Fetch platform info whenever the selected kubeconfig changes
  const refreshPlatform = useCallback((kubeconfig) => {
    setPlatformLoading(true)
    fetchPlatform(kubeconfig)
      .then((info) => {
        setPlatformInfo(info)
        onPlatform?.(info)
      })
      .catch(() => setPlatformInfo(null))
      .finally(() => setPlatformLoading(false))
  }, [onPlatform])

  useEffect(() => {
    fetchKubeconfigs()
      .then(({ files, default: def }) => {
        setOptions(files)
        if (!value && def) {
          onChange(def)
          refreshPlatform(def)
        } else if (value) {
          refreshPlatform(value)
        }
      })
      .catch(() => {
        setError('Could not list kubeconfig files — enter a path manually.')
        setCustom(true)
        refreshPlatform('')
      })
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleChange = (newValue) => {
    onChange(newValue)
    if (newValue && newValue !== '__custom__') {
      refreshPlatform(newValue)
    }
  }

  const platformTag = platformInfo ? PLATFORM_TAG[platformInfo.platform] ?? { type: 'gray', label: platformInfo.displayName } : null

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

      {/* Selector row — input + platform badge side by side */}
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: '0.75rem' }}>
        <div style={{ flex: 1 }}>
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
                  handleChange(e.target.value)
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
              onChange={(e) => handleChange(e.target.value)}
            />
          )}
        </div>

        {/* Platform badge */}
        <div style={{ paddingBottom: '1.5rem', minWidth: '110px' }}>
          {platformLoading ? (
            <span style={{ fontSize: '0.75rem', color: 'var(--cds-text-helper)' }}>Detecting…</span>
          ) : platformTag ? (
            <div>
              <Tag type={platformTag.type} size="md">
                {platformTag.label}
              </Tag>
              {platformInfo?.azureRbacMode && (
                <div style={{ fontSize: '0.7rem', color: 'var(--cds-support-warning)', marginTop: '2px' }}>
                  Azure RBAC active
                </div>
              )}
            </div>
          ) : null}
        </div>
      </div>

      {/* Toggle link */}
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
          onClick={() => { setCustom(false); handleChange(options[0]) }}
        >
          ← Back to detected files
        </button>
      )}
    </div>
  )
}
