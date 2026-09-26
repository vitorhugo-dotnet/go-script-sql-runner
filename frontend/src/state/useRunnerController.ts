import { useCallback, useEffect, useRef, useState } from 'react'
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
  const selectedProfileIDRef = useRef<string | null>(null)

  const clearError = useCallback(() => setError(null), [])

  const clearConnectionState = useCallback(() => {
    setCapabilities(null)
    setAvailableSchemas([])
    setSelectedSchema(null)
  }, [])

  const applySelectedProfile = useCallback(
    (profile: Profile | null) => {
      selectedProfileIDRef.current = profile?.id ?? null
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

      selectedProfileIDRef.current = profileID
      try {
        setError(null)
        const profile = await api.getProfile(profileID)
        if (selectedProfileIDRef.current === profileID) applySelectedProfile(profile)
      } catch (cause) {
        if (selectedProfileIDRef.current === profileID) {
          selectedProfileIDRef.current = selectedProfile?.id ?? null
          setError(errorMessage(cause))
        }
      }
    },
    [api, applySelectedProfile, selectedProfile],
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

  const deleteProfile = useCallback(async (profileID: string): Promise<boolean> => {
    if (!profileID) return false
    try {
      setError(null)
      await api.deleteProfile(profileID)
    } catch (cause) {
      setError(errorMessage(cause))
      return false
    }

    const remaining = profiles.filter((profile) => profile.id !== profileID)
    const deletedSelection = selectedProfileIDRef.current === profileID
    setProfiles(remaining)
    if (deletedSelection) applySelectedProfile(remaining[0] ?? null)
    let expectedSelectionID = selectedProfileIDRef.current
    try {
      const loaded = (await api.listProfiles()).filter((profile) => profile.id !== profileID)
      setProfiles(loaded)
      const next = loaded.find((profile) => profile.id === expectedSelectionID) ?? loaded[0] ?? null
      if (selectedProfileIDRef.current === expectedSelectionID) {
        const preserveCurrentSelection = next?.id === expectedSelectionID
        expectedSelectionID = next?.id ?? null
        if (preserveCurrentSelection) setSelectedProfile(next)
        else applySelectedProfile(next)
      }
      if (next) {
        const fresh = await api.getProfile(next.id)
        setProfiles((current) => current.map((profile) => (profile.id === fresh.id ? fresh : profile)))
        if (selectedProfileIDRef.current === expectedSelectionID) {
          if (fresh.id === expectedSelectionID) setSelectedProfile(fresh)
          else applySelectedProfile(fresh)
        }
      }
    } catch (cause) {
      setError(errorMessage(cause))
    }
    return true
  }, [api, applySelectedProfile, profiles])

  const deleteSelectedProfile = useCallback(async (): Promise<boolean> => {
    return selectedProfile ? deleteProfile(selectedProfile.id) : false
  }, [deleteProfile, selectedProfile])

  const cloneSelectedProfile = useCallback(async (): Promise<Profile | null> => {
    if (!selectedProfile) return null
    let cloned: Profile
    try {
      setError(null)
      cloned = await api.cloneProfile(selectedProfile.id)
    } catch (cause) {
      setError(errorMessage(cause))
      return null
    }

    setProfiles((current) => [...current.filter((profile) => profile.id !== cloned.id), cloned])
    applySelectedProfile(cloned)
    const expectedSelectionID = cloned.id
    try {
      const loaded = await api.listProfiles()
      const withClone = loaded.some((profile) => profile.id === cloned.id)
        ? loaded.map((profile) => (profile.id === cloned.id ? cloned : profile))
        : [...loaded, cloned]
      setProfiles(withClone)
      if (selectedProfileIDRef.current === expectedSelectionID) {
        setSelectedProfile(withClone.find((profile) => profile.id === expectedSelectionID) ?? cloned)
      }
      const fresh = await api.getProfile(cloned.id)
      setProfiles((current) => current.map((profile) => (profile.id === fresh.id ? fresh : profile)))
      if (selectedProfileIDRef.current === expectedSelectionID) setSelectedProfile(fresh)
      return fresh
    } catch (cause) {
      setError(errorMessage(cause))
      return cloned
    }
  }, [api, applySelectedProfile, selectedProfile])

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

  const loadScriptContent = useCallback(async (profileID: string, scriptID: string): Promise<string | null> => {
    if (!selectedProfile || !profileID) return null
    try {
      setError(null)
      return await api.getScriptContent(profileID, scriptID)
    } catch (cause) {
      setError(errorMessage(cause))
      return null
    }
  }, [api, selectedProfile])

  const saveScriptContent = useCallback(async (profileID: string, scriptID: string, content: string): Promise<boolean> => {
    if (!selectedProfile || !profileID) return false
    try {
      setError(null)
      await api.saveScriptContent(profileID, scriptID, content)
    } catch (cause) {
      setError(errorMessage(cause))
      return false
    }
    try {
      const refreshed = await api.getProfile(profileID)
      setProfiles((current) => current.map((item) => (item.id === profileID ? refreshed : item)))
      setSelectedProfile((current) => (current?.id === profileID ? refreshed : current))
    } catch (cause) {
      setError(errorMessage(cause))
    }
    return true
  }, [api, selectedProfile])

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
    clearError,
    loadProfiles,
    selectProfile,
    saveProfile,
    deleteProfile,
    deleteSelectedProfile,
    cloneSelectedProfile,
    setSelectedSchema,
    setRunOnError,
    setRunTransactionMode,
    connect,
    addSQLFile: addSQLFiles,
    addSQLFiles,
    removeScript,
    loadScriptContent,
    saveScriptContent,
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
