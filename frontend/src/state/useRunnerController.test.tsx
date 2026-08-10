import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ExecutionEvent, Profile, RunnerApi } from '../api/types'
import { useRunnerController } from './useRunnerController'

const firstProfile: Profile = {
  id: 'first',
  name: 'First',
  version: 1,
  connection: {
    host: '127.0.0.1',
    port: 3306,
    database: 'legacy',
    username: 'dev',
    password: 'dev',
  },
  execution: {
    onError: 'continue',
    transactionMode: 'auto_commit',
  },
  scripts: [],
}

const secondProfile: Profile = {
  ...firstProfile,
  id: 'second',
  name: 'Second',
  execution: {
    onError: 'stop',
    transactionMode: 'transaction',
  },
}

const connectionResult = {
  capabilities: {
    vendor: 'mysql',
    major: 8,
    minor: 0,
    patch: 39,
    rawVersion: '8.0.39',
    versionLabel: 'MySQL 8.0',
  },
  schemas: ['apollo', 'mysql', 'TestDB'],
}

function fakeApi(overrides: Partial<RunnerApi> = {}) {
  let executionHandler: ((event: ExecutionEvent) => void) | undefined
  const profiles = new Map([
    [firstProfile.id, firstProfile],
    [secondProfile.id, secondProfile],
  ])

  const api: RunnerApi & { emit: (event: ExecutionEvent) => void } = {
    listProfiles: vi.fn().mockResolvedValue([firstProfile, secondProfile]),
    getProfile: vi.fn().mockImplementation(async (id: string) => profiles.get(id) ?? firstProfile),
    createProfile: vi.fn().mockImplementation(async (profile: Profile) => profile),
    updateProfile: vi.fn().mockImplementation(async (profile: Profile) => profile),
    addScriptFromDialog: vi.fn().mockResolvedValue(null),
    addScriptsFromDialog: vi.fn().mockResolvedValue([]),
    removeScript: vi.fn().mockResolvedValue(undefined),
    reorderScripts: vi.fn().mockResolvedValue(firstProfile),
    setScriptEnabled: vi.fn().mockResolvedValue(firstProfile),
    setScriptTransactionMode: vi.fn().mockResolvedValue(firstProfile),
    testConnection: vi.fn().mockResolvedValue(connectionResult),
    runProfile: vi.fn().mockResolvedValue({ results: [], succeeded: 0, failed: 0, aborted: false }),
    stopRun: vi.fn().mockResolvedValue(true),
    importProfileFromDialog: vi.fn().mockResolvedValue(null),
    exportProfileToDialog: vi.fn().mockResolvedValue(''),
    checkForUpdates: vi.fn().mockResolvedValue({
      currentTag: 'dev',
      latestTag: 'dev',
      available: false,
      releaseUrl: '',
      downloadUrl: '',
    }),
    openExternalURL: vi.fn(),
    onExecutionEvent: vi.fn().mockImplementation((handler: (event: ExecutionEvent) => void) => {
      executionHandler = handler
      return () => {
        executionHandler = undefined
      }
    }),
    emit(event) {
      executionHandler?.(event)
    },
    ...overrides,
  }

  return api
}

describe('useRunnerController', () => {
  it('selects the first profile initially and clears runtime schema state on profile selection', async () => {
    const api = fakeApi()
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.testConnection()
    })
    act(() => result.current.setSelectedSchema('apollo'))
    expect(result.current.selectedSchema).toBe('apollo')

    await act(async () => {
      await result.current.selectProfile('second')
    })

    expect(result.current.selectedProfile?.id).toBe('second')
    expect(result.current.runOnError).toBe('stop')
    expect(result.current.runTransactionMode).toBe('transaction')
    expect(result.current.capabilities).toBeNull()
    expect(result.current.availableSchemas).toEqual([])
    expect(result.current.selectedSchema).toBeNull()
    expect(api.getProfile).toHaveBeenCalledWith('second')
  })

  it('stores capabilities and visible schemas without auto-selecting a schema', async () => {
    const api = fakeApi()
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.testConnection()
    })

    expect(result.current.capabilities?.versionLabel).toBe('MySQL 8.0')
    expect(result.current.availableSchemas).toEqual(['apollo', 'mysql', 'TestDB'])
    expect(result.current.selectedSchema).toBeNull()
  })

  it('clears schema state when connection testing fails', async () => {
    const testConnection = vi
      .fn()
      .mockResolvedValueOnce(connectionResult)
      .mockRejectedValueOnce(new Error('connection failed'))
    const api = fakeApi({ testConnection })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.testConnection()
    })
    act(() => result.current.setSelectedSchema('apollo'))

    await act(async () => {
      await result.current.testConnection()
    })

    expect(result.current.capabilities).toBeNull()
    expect(result.current.availableSchemas).toEqual([])
    expect(result.current.selectedSchema).toBeNull()
    expect(result.current.error).toBe('connection failed')
  })

  it('keeps only the newest 1000 execution events', async () => {
    const api = fakeApi()
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))

    act(() => {
      for (let index = 0; index < 1005; index += 1) {
        api.emit({
          time: `2026-08-07T18:00:${String(index % 60).padStart(2, '0')}Z`,
          level: 'INFO',
          message: `event-${index}`,
        })
      }
    })

    expect(result.current.logs).toHaveLength(1000)
    expect(result.current.logs[0].message).toBe('event-5')
    expect(result.current.logs[999].message).toBe('event-1004')
  })

  it('requires a selected schema before running', async () => {
    const runProfile = vi.fn()
    const api = fakeApi({ runProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.testConnection()
      await result.current.run()
    })

    expect(runProfile).not.toHaveBeenCalled()
    expect(result.current.error).toContain('schema')
  })

  it('resets running after failure and sends runtime schema plus run-only overrides without saving the profile', async () => {
    let rejectRun: ((reason?: unknown) => void) | undefined
    const runProfile = vi.fn().mockImplementation(
      () =>
        new Promise((_, reject) => {
          rejectRun = reject
        }),
    )
    const updateProfile = vi.fn().mockResolvedValue(firstProfile)
    const api = fakeApi({ runProfile, updateProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.testConnection()
    })
    act(() => {
      result.current.setSelectedSchema('apollo')
      result.current.setRunOnError('stop')
      result.current.setRunTransactionMode('transaction')
    })

    let runPromise: Promise<void>
    act(() => {
      runPromise = result.current.run()
    })
    await waitFor(() => expect(result.current.running).toBe(true))

    await act(async () => {
      rejectRun?.(new Error('synthetic failure'))
      await runPromise
    })

    expect(result.current.running).toBe(false)
    expect(result.current.error).toBe('synthetic failure')
    expect(runProfile).toHaveBeenCalledWith('first', {
      schema: 'apollo',
      onError: 'stop',
      transactionMode: 'transaction',
    })
    expect(updateProfile).not.toHaveBeenCalled()
  })

  it('refreshes profiles and selects the imported profile with no runtime schema selected', async () => {
    const imported: Profile = { ...firstProfile, id: 'imported', name: 'Imported' }
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile])
      .mockResolvedValueOnce([firstProfile, imported])
    const getProfile = vi.fn().mockImplementation(async (id: string) => (id === 'imported' ? imported : firstProfile))
    const api = fakeApi({
      listProfiles,
      getProfile,
      importProfileFromDialog: vi.fn().mockResolvedValue(imported),
    })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.importProfile()
    })

    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['first', 'imported'])
    expect(result.current.selectedProfile?.id).toBe('imported')
    expect(result.current.selectedSchema).toBeNull()
  })
})
