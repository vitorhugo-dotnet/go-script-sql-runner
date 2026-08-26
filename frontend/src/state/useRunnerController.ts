import { useCallback, useEffect, useState } from 'react'
import type {
  ExecutionEvent,
  OnError,
  Profile,
  RunnerApi,
  ServerCapabilities,
  TransactionMode,
} from '../api/types'

const MAX_LOG_EVENTS = 1000

function errorMessage(cause: unknown) {
  return cause instanceof Error ? cause.message : String(cause)
}

export function useRunnerController(api: RunnerApi) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [selectedProfile, setSelectedProfile] = useState<Profile | null>(null)
  const [capabilities, setCapabilities] = useState<ServerCapabilities | null>(null)
  const [availableSchemas, setAvailableSchemas] = useState<string[]>([])
  const [selectedSchema, setSelectedSchema] = useState<string | null>(null)
  const [logs, setLogs] = useState<ExecutionEvent[]>([])
  const [runOnError, setRunOnError] = useState<OnError>('continue')
  const [runTransactionMode, setRunTransactionMode] = useState<TransactionMode>('auto_commit')
  const [loading, setLoading] = useState(true)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const clearConnectionState = useCallback(() => {
    setCapabilities(null)
    setAvailableSchemas([])
    setSelectedSchema(null)
  }, [])

  const applySelectedProfile = useCallback(
    (profile: Profile | null) => {
      setSelectedProfile(profile)
      clearConnectionState()
      if (profile) {
        setRunOnError(profile.execution.onError)
        setRunTransactionMode(profile.execution.transactionMode)
      }
    },
    [clearConnectionState],
  )

  const selectProfile = useCallback(
    async (profileID: string) => {
      if (!profileID) {
        applySelectedProfile(null)
        return
      }

      try {
        setError(null)
        const profile = await api.getProfile(profileID)
        applySelectedProfile(profile)
      } catch (cause) {
        setError(errorMessage(cause))
      }
    },
    [api, applySelectedProfile],
  )

  const loadProfiles = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const loaded = await api.listProfiles()
      setProfiles(loaded)

      if (loaded.length === 0) {
        applySelectedProfile(null)
        return
      }

      const first = await api.getProfile(loaded[0].id)
      applySelectedProfile(first)
    } catch (cause) {
      setProfiles([])
      applySelectedProfile(null)
      setError(errorMessage(cause))
    } finally {
      setLoading(false)
    }
  }, [api, applySelectedProfile])

  const saveProfile = useCallback(
    async (draft: Profile) => {
      try {
        setError(null)
        const saved = draft.id ? await api.updateProfile(draft) : await api.createProfile(draft)
        const loaded = await api.listProfiles()
        setProfiles(loaded)
        const fresh = await api.getProfile(saved.id)
        applySelectedProfile(fresh)
        return fresh
      } catch (cause) {
        setError(errorMessage(cause))
        throw cause
      }
    },
    [api, applySelectedProfile],
  )

  const refreshSelectedProfile = useCallback(async () => {
    if (!selectedProfile) return null
    const refreshed = await api.getProfile(selectedProfile.id)
    setSelectedProfile(refreshed)
    setProfiles((current) => current.map((item) => (item.id === refreshed.id ? refreshed : item)))
    return refreshed
  }, [api, selectedProfile])

  const connect = useCallback(async () => {
    if (!selectedProfile) return
    try {
      setError(null)
      clearConnectionState()
      const result = await api.connect(selectedProfile.id)
      setCapabilities(result.capabilities)
      setAvailableSchemas(result.schemas)
    } catch (cause) {
      clearConnectionState()
      setError(errorMessage(cause))
    }
  }, [api, clearConnectionState, selectedProfile])

  const addSQLFiles = useCallback(async () => {
    if (!selectedProfile) return
    try {
      setError(null)
      const added = await api.addScriptsFromDialog(selectedProfile.id)
      if (added.length > 0) {
        await refreshSelectedProfile()
      }
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }, [api, refreshSelectedProfile, selectedProfile])

  const removeScript = useCallback(
    async (scriptID: string) => {
      if (!selectedProfile) return
      try {
        setError(null)
        await api.removeScript(selectedProfile.id, scriptID)
        await refreshSelectedProfile()
      } catch (cause) {
        setError(errorMessage(cause))
      }
    },
    [api, refreshSelectedProfile, selectedProfile],
  )

  const setScriptEnabled = useCallback(
    async (scriptID: string, enabled: boolean) => {
      if (!selectedProfile) return
      try {
        setError(null)
        const updated = await api.setScriptEnabled(selectedProfile.id, scriptID, enabled)
        setSelectedProfile(updated)
        setProfiles((current) => current.map((item) => (item.id === updated.id ? updated : item)))
      } catch (cause) {
        setError(errorMessage(cause))
      }
    },
    [api, selectedProfile],
  )

  const setScriptTransactionMode = useCallback(
    async (scriptID: string, mode: TransactionMode | '') => {
      if (!selectedProfile) return
      try {
        setError(null)
        const updated = await api.setScriptTransactionMode(selectedProfile.id, scriptID, mode)
        setSelectedProfile(updated)
        setProfiles((current) => current.map((item) => (item.id === updated.id ? updated : item)))
      } catch (cause) {
        setError(errorMessage(cause))
      }
    },
    [api, selectedProfile],
  )

  const moveScript = useCallback(
    async (scriptID: string, direction: -1 | 1) => {
      if (!selectedProfile) return

      const scripts = [...selectedProfile.scripts].sort((left, right) => left.order - right.order)
      const index = scripts.findIndex((script) => script.id === scriptID)
      const target = index + direction
      if (index < 0 || target < 0 || target >= scripts.length) return

      ;[scripts[index], scripts[target]] = [scripts[target], scripts[index]]

      try {
        setError(null)
        const updated = await api.reorderScripts(
          selectedProfile.id,
          scripts.map((script) => script.id),
        )
        setSelectedProfile(updated)
        setProfiles((current) => current.map((item) => (item.id === updated.id ? updated : item)))
      } catch (cause) {
        setError(errorMessage(cause))
      }
    },
    [api, selectedProfile],
  )

  const run = useCallback(async () => {
    if (!selectedProfile || running) return
    if (!selectedSchema) {
      setError('Select a schema before running.')
      return
    }
    try {
      setRunning(true)
      setError(null)
      await api.runProfile(selectedProfile.id, {
        schema: selectedSchema,
        onError: runOnError,
        transactionMode: runTransactionMode,
      })
    } catch (cause) {
      setError(errorMessage(cause))
    } finally {
      setRunning(false)
    }
  }, [api, runOnError, runTransactionMode, running, selectedProfile, selectedSchema])

  const stopRun = useCallback(async () => {
    try {
      await api.stopRun()
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }, [api])

  const importProfile = useCallback(async () => {
    try {
      setError(null)
      const imported = await api.importProfileFromDialog()
      if (!imported) return

      const loaded = await api.listProfiles()
      setProfiles(loaded)
      const fresh = await api.getProfile(imported.id)
      applySelectedProfile(fresh)
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }, [api, applySelectedProfile])

  const exportProfile = useCallback(async () => {
    if (!selectedProfile) return
    try {
      setError(null)
      await api.exportProfileToDialog(selectedProfile.id)
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }, [api, selectedProfile])

  const clearLogs = useCallback(() => setLogs([]), [])

  useEffect(() => {
    void loadProfiles()
  }, [loadProfiles])

  useEffect(
    () =>
      api.onExecutionEvent((event) => {
        setLogs((current) => [...current, event].slice(-MAX_LOG_EVENTS))
      }),
    [api],
  )

  return {
    profiles,
    selectedProfile,
    capabilities,
    availableSchemas,
    selectedSchema,
    logs,
    runOnError,
    runTransactionMode,
    loading,
    running,
    error,
    loadProfiles,
    selectProfile,
    saveProfile,
    setSelectedSchema,
    setRunOnError,
    setRunTransactionMode,
    connect,
    addSQLFile: addSQLFiles,
    addSQLFiles,
    removeScript,
    setScriptEnabled,
    setScriptTransactionMode,
    moveScript,
    run,
    stopRun,
    importProfile,
    exportProfile,
    clearLogs,
  }
}
