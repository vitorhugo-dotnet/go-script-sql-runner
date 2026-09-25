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

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
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
    deleteProfile: vi.fn().mockResolvedValue(undefined),
    cloneProfile: vi.fn().mockResolvedValue({ ...firstProfile, id: 'clone', name: 'First (copy)' }),
    addScriptFromDialog: vi.fn().mockResolvedValue(null),
    addScriptsFromDialog: vi.fn().mockResolvedValue([]),
    removeScript: vi.fn().mockResolvedValue(undefined),
    getScriptContent: vi.fn().mockResolvedValue('SELECT 1;\n'),
    saveScriptContent: vi.fn().mockResolvedValue(undefined),
    reorderScripts: vi.fn().mockResolvedValue(firstProfile),
    setScriptEnabled: vi.fn().mockResolvedValue(firstProfile),
    setScriptTransactionMode: vi.fn().mockResolvedValue(firstProfile),
    connect: vi.fn().mockResolvedValue(connectionResult),
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
  it('loads exact SQL text for a script in the selected profile', async () => {
    const content = '-- Café\nSELECT 2;\n'
    const getScriptContent = vi.fn().mockResolvedValue(content)
    const api = fakeApi({ getScriptContent })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    let loaded: string | null = null
    await act(async () => { loaded = await result.current.loadScriptContent('first', 'script-one') })

    expect(loaded).toBe(content)
    expect(getScriptContent).toHaveBeenCalledWith('first', 'script-one')
    expect(result.current.error).toBeNull()
  })

  it('saves SQL text, refreshes the selected profile, and returns true', async () => {
    const refreshed: Profile = { ...firstProfile, scripts: [{ id: 'script-one', name: 'One', file: 'scripts/one.sql', enabled: true, order: 10 }] }
    const getProfile = vi.fn().mockResolvedValueOnce(firstProfile).mockResolvedValueOnce(refreshed)
    const saveScriptContent = vi.fn().mockResolvedValue(undefined)
    const api = fakeApi({ getProfile, saveScriptContent })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.saveScriptContent('first', 'script-one', '-- Café\nSELECT 2;\n')).toBe(true)
    })

    expect(saveScriptContent).toHaveBeenCalledWith('first', 'script-one', '-- Café\nSELECT 2;\n')
    expect(getProfile).toHaveBeenCalledTimes(2)
    expect(result.current.selectedProfile).toEqual(refreshed)
    expect(result.current.error).toBeNull()
  })

  it('reports a refresh error after a committed script save without turning it into a failed save', async () => {
    const getProfile = vi.fn().mockResolvedValueOnce(firstProfile).mockRejectedValueOnce(new Error('refresh failed'))
    const saveScriptContent = vi.fn().mockResolvedValue(undefined)
    const api = fakeApi({ getProfile, saveScriptContent })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.saveScriptContent('first', 'script-one', 'changed')).toBe(true)
    })

    expect(saveScriptContent).toHaveBeenCalledWith('first', 'script-one', 'changed')
    expect(result.current.selectedProfile).toEqual(firstProfile)
    expect(result.current.error).toBe('refresh failed')
  })

  it('reports script content failures without losing the selected profile', async () => {
    const getScriptContent = vi.fn().mockRejectedValue(new Error('read failed'))
    const saveScriptContent = vi.fn().mockRejectedValue(new Error('write failed'))
    const api = fakeApi({ getScriptContent, saveScriptContent })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => { expect(await result.current.loadScriptContent('first', 'script-one')).toBeNull() })
    expect(result.current.error).toBe('read failed')
    await act(async () => { expect(await result.current.saveScriptContent('first', 'script-one', 'changed')).toBe(false) })
    expect(result.current.error).toBe('write failed')
    expect(result.current.selectedProfile?.id).toBe('first')
  })

  it('does not call script content APIs when no profile is selected', async () => {
    const api = fakeApi({ listProfiles: vi.fn().mockResolvedValue([]) })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.loading).toBe(false))
    await act(async () => {
      expect(await result.current.loadScriptContent('first', 'script-one')).toBeNull()
      expect(await result.current.saveScriptContent('first', 'script-one', 'changed')).toBe(false)
    })
    expect(api.getScriptContent).not.toHaveBeenCalled()
    expect(api.saveScriptContent).not.toHaveBeenCalled()
  })

  it('saves to the profile where the editor loaded content after selection changes to a clone', async () => {
    const clone: Profile = { ...firstProfile, id: 'clone', name: 'First (copy)' }
    const refreshed: Profile = { ...firstProfile, name: 'First refreshed' }
    const getProfile = vi.fn()
      .mockResolvedValueOnce(firstProfile)
      .mockResolvedValueOnce(clone)
      .mockResolvedValueOnce(refreshed)
    const getScriptContent = vi.fn().mockResolvedValue('original SQL')
    const saveScriptContent = vi.fn().mockResolvedValue(undefined)
    const api = fakeApi({
      listProfiles: vi.fn().mockResolvedValue([firstProfile, clone]),
      getProfile,
      getScriptContent,
      saveScriptContent,
    })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.loadScriptContent('first', 'shared-script')).toBe('original SQL')
    })
    await act(async () => { await result.current.selectProfile('clone') })
    await act(async () => {
      expect(await result.current.saveScriptContent('first', 'shared-script', 'edited SQL')).toBe(true)
    })

    expect(getScriptContent).toHaveBeenCalledWith('first', 'shared-script')
    expect(saveScriptContent).toHaveBeenCalledWith('first', 'shared-script', 'edited SQL')
    expect(getProfile).toHaveBeenLastCalledWith('first')
    expect(result.current.selectedProfile).toEqual(clone)
    expect(result.current.profiles.find((item) => item.id === 'first')).toEqual(refreshed)
  })

  it('does not restore the old selection when a pending save resolves', async () => {
    const clone: Profile = { ...firstProfile, id: 'clone', name: 'First (copy)' }
    const refreshed: Profile = { ...firstProfile, name: 'First refreshed' }
    const getProfile = vi.fn()
      .mockResolvedValueOnce(firstProfile)
      .mockResolvedValueOnce(clone)
      .mockResolvedValueOnce(refreshed)
    let releaseSave: (() => void) | undefined
    const saveScriptContent = vi.fn().mockImplementation(() => new Promise<void>((resolve) => {
      releaseSave = resolve
    }))
    const api = fakeApi({
      listProfiles: vi.fn().mockResolvedValue([firstProfile, clone]),
      getProfile,
      saveScriptContent,
    })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    let savePromise!: Promise<boolean>
    act(() => { savePromise = result.current.saveScriptContent('first', 'shared-script', 'edited SQL') })
    await waitFor(() => expect(saveScriptContent).toHaveBeenCalledWith('first', 'shared-script', 'edited SQL'))
    await act(async () => { await result.current.selectProfile('clone') })
    await act(async () => {
      releaseSave?.()
      expect(await savePromise).toBe(true)
    })

    expect(getProfile).toHaveBeenLastCalledWith('first')
    expect(result.current.selectedProfile).toEqual(clone)
    expect(result.current.profiles.find((item) => item.id === 'first')).toEqual(refreshed)
  })

  it('deletes an explicit profile ID while preserving a different active selection', async () => {
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockResolvedValueOnce([secondProfile])
    const deleteProfile = vi.fn().mockResolvedValue(undefined)
    const api = fakeApi({ listProfiles, deleteProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.selectProfile('second')
    })
    await act(async () => {
      await result.current.connect()
    })
    act(() => result.current.setSelectedSchema('apollo'))

    await act(async () => {
      expect(await result.current.deleteProfile('first')).toBe(true)
    })

    expect(deleteProfile).toHaveBeenCalledWith('first')
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['second'])
    expect(result.current.selectedProfile?.id).toBe('second')
    expect(result.current.selectedSchema).toBe('apollo')
  })

  it('clones the selected profile, refreshes the list, and selects the returned clone', async () => {
    const clone: Profile = { ...firstProfile, id: 'clone', name: 'First (copy)' }
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockResolvedValueOnce([firstProfile, secondProfile, clone])
    const getProfile = vi.fn().mockImplementation(async (id: string) => (id === clone.id ? clone : firstProfile))
    const cloneProfile = vi.fn().mockResolvedValue(clone)
    const api = fakeApi({ listProfiles, getProfile, cloneProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    let returned: Profile | null = null
    await act(async () => {
      returned = await result.current.cloneSelectedProfile()
    })

    expect(cloneProfile).toHaveBeenCalledWith('first')
    expect(listProfiles).toHaveBeenCalledTimes(2)
    expect(getProfile).toHaveBeenCalledWith('clone')
    expect(returned).toEqual(clone)
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['first', 'second', 'clone'])
    expect(result.current.selectedProfile?.id).toBe('clone')
  })

  it('preserves user profile selections made while clone list and get refreshes are pending', async () => {
    const clone: Profile = { ...firstProfile, id: 'clone', name: 'First (copy)' }
    const refreshedClone: Profile = { ...clone, name: 'First (copy) refreshed' }
    const thirdProfile: Profile = { ...secondProfile, id: 'third', name: 'Third' }
    const listRefresh = deferred<Profile[]>()
    const cloneRefresh = deferred<Profile>()
    const listProfiles = vi.fn().mockResolvedValueOnce([firstProfile, secondProfile, thirdProfile]).mockReturnValueOnce(listRefresh.promise)
    const getProfile = vi.fn().mockImplementation((id: string) => {
      if (id === clone.id) return cloneRefresh.promise
      if (id === secondProfile.id) return Promise.resolve(secondProfile)
      if (id === thirdProfile.id) return Promise.resolve(thirdProfile)
      return Promise.resolve(firstProfile)
    })
    const api = fakeApi({ listProfiles, getProfile, cloneProfile: vi.fn().mockResolvedValue(clone) })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    let clonePromise!: Promise<Profile | null>
    act(() => { clonePromise = result.current.cloneSelectedProfile() })
    await waitFor(() => expect(listProfiles).toHaveBeenCalledTimes(2))

    await act(async () => { await result.current.selectProfile('second') })
    expect(result.current.selectedProfile?.id).toBe('second')

    await act(async () => {
      listRefresh.resolve([firstProfile, secondProfile, thirdProfile, clone])
      await Promise.resolve()
    })
    await waitFor(() => expect(getProfile).toHaveBeenCalledWith('clone'))
    expect(result.current.selectedProfile?.id).toBe('second')

    await act(async () => { await result.current.selectProfile('third') })
    await act(async () => {
      cloneRefresh.resolve(refreshedClone)
      await clonePromise
    })

    expect(result.current.selectedProfile?.id).toBe('third')
    expect(result.current.profiles.find((profile) => profile.id === 'clone')).toEqual(refreshedClone)
  })

  it('preserves user profile selections made while delete list and get refreshes are pending', async () => {
    const thirdProfile: Profile = { ...secondProfile, id: 'third', name: 'Third' }
    const fourthProfile: Profile = { ...secondProfile, id: 'fourth', name: 'Fourth' }
    const refreshedSecond: Profile = { ...secondProfile, name: 'Second refreshed' }
    const listRefresh = deferred<Profile[]>()
    const secondRefresh = deferred<Profile>()
    const listProfiles = vi.fn().mockResolvedValueOnce([firstProfile, secondProfile, thirdProfile]).mockReturnValueOnce(listRefresh.promise)
    const getProfile = vi.fn().mockImplementation((id: string) => {
      if (id === 'second') return secondRefresh.promise
      if (id === 'third') return Promise.resolve(thirdProfile)
      if (id === 'fourth') return Promise.resolve(fourthProfile)
      return Promise.resolve(firstProfile)
    })
    const api = fakeApi({ listProfiles, getProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    let deletePromise!: Promise<boolean>
    act(() => { deletePromise = result.current.deleteProfile('first') })
    await waitFor(() => expect(listProfiles).toHaveBeenCalledTimes(2))

    await act(async () => { await result.current.selectProfile('third') })
    expect(result.current.selectedProfile?.id).toBe('third')
    await act(async () => {
      listRefresh.resolve([secondProfile, thirdProfile, fourthProfile])
      await Promise.resolve()
    })
    await waitFor(() => expect(getProfile).toHaveBeenCalledWith('second'))
    expect(result.current.selectedProfile?.id).toBe('third')

    await act(async () => { await result.current.selectProfile('fourth') })
    await act(async () => {
      secondRefresh.resolve(refreshedSecond)
      expect(await deletePromise).toBe(true)
    })

    expect(result.current.selectedProfile?.id).toBe('fourth')
    expect(result.current.profiles.find((profile) => profile.id === 'second')).toEqual(refreshedSecond)
  })

  it.each([
    { remaining: [secondProfile], expectedID: 'second' },
    { remaining: [] as Profile[], expectedID: null },
  ])('deletes the selected profile and selects $expectedID', async ({ remaining, expectedID }) => {
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockResolvedValueOnce(remaining)
    const getProfile = vi.fn().mockImplementation(async (id: string) => (id === 'second' ? secondProfile : firstProfile))
    const deleteProfile = vi.fn().mockResolvedValue(undefined)
    const api = fakeApi({ listProfiles, getProfile, deleteProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    let deleted = false
    await act(async () => {
      deleted = await result.current.deleteSelectedProfile()
    })

    expect(deleted).toBe(true)
    expect(deleteProfile).toHaveBeenCalledWith('first')
    expect(listProfiles).toHaveBeenCalledTimes(2)
    expect(result.current.profiles).toEqual(remaining)
    expect(result.current.selectedProfile?.id ?? null).toBe(expectedID)
    if (expectedID) expect(getProfile).toHaveBeenCalledWith(expectedID)
  })

  it('keeps the current selection and reports clone or delete API errors', async () => {
    const cloneProfile = vi.fn().mockRejectedValue(new Error('clone failed'))
    const deleteProfile = vi.fn().mockRejectedValue(new Error('delete failed'))
    const api = fakeApi({ cloneProfile, deleteProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.cloneSelectedProfile()).toBeNull()
    })
    expect(result.current.error).toBe('clone failed')
    expect(result.current.selectedProfile?.id).toBe('first')
    await act(async () => {
      expect(await result.current.deleteSelectedProfile()).toBe(false)
    })
    expect(result.current.error).toBe('delete failed')
    expect(result.current.selectedProfile?.id).toBe('first')
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['first', 'second'])
  })

  it('keeps committed deletion and a valid fallback when list refresh fails', async () => {
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockRejectedValueOnce(new Error('refresh failed'))
    const api = fakeApi({ listProfiles })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.deleteSelectedProfile()).toBe(true)
    })
    expect(result.current.error).toBe('refresh failed')
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['second'])
    expect(result.current.selectedProfile?.id).toBe('second')
  })

  it('keeps committed deletion when loading the refreshed fallback fails', async () => {
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockResolvedValueOnce([secondProfile])
    const getProfile = vi
      .fn()
      .mockResolvedValueOnce(firstProfile)
      .mockRejectedValueOnce(new Error('load failed'))
    const api = fakeApi({ listProfiles, getProfile })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.deleteSelectedProfile()).toBe(true)
    })
    expect(result.current.error).toBe('load failed')
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['second'])
    expect(result.current.selectedProfile?.id).toBe('second')
  })

  it('clears selection after deleting the last profile even if refresh fails', async () => {
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile])
      .mockRejectedValueOnce(new Error('refresh failed'))
    const api = fakeApi({ listProfiles })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.deleteSelectedProfile()).toBe(true)
    })
    expect(result.current.profiles).toEqual([])
    expect(result.current.selectedProfile).toBeNull()
    expect(result.current.error).toBe('refresh failed')
  })

  it('keeps the returned clone selected when list refresh fails', async () => {
    const clone: Profile = { ...firstProfile, id: 'clone', name: 'First (copy)' }
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockRejectedValueOnce(new Error('refresh failed'))
    const api = fakeApi({ listProfiles, cloneProfile: vi.fn().mockResolvedValue(clone) })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.cloneSelectedProfile()).toEqual(clone)
    })
    expect(result.current.error).toBe('refresh failed')
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['first', 'second', 'clone'])
    expect(result.current.selectedProfile).toEqual(clone)
  })

  it('keeps the returned clone selected when loading it after refresh fails', async () => {
    const clone: Profile = { ...firstProfile, id: 'clone', name: 'First (copy)' }
    const listProfiles = vi
      .fn()
      .mockResolvedValueOnce([firstProfile, secondProfile])
      .mockResolvedValueOnce([firstProfile, secondProfile])
    const getProfile = vi
      .fn()
      .mockResolvedValueOnce(firstProfile)
      .mockRejectedValueOnce(new Error('load failed'))
    const api = fakeApi({ listProfiles, getProfile, cloneProfile: vi.fn().mockResolvedValue(clone) })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      expect(await result.current.cloneSelectedProfile()).toEqual(clone)
    })
    expect(result.current.error).toBe('load failed')
    expect(result.current.profiles.map((profile) => profile.id)).toEqual(['first', 'second', 'clone'])
    expect(result.current.selectedProfile).toEqual(clone)
  })

  it('selects the first profile initially and clears runtime schema state on profile selection', async () => {
    const api = fakeApi()
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.connect()
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
      await result.current.connect()
    })

    expect(result.current.capabilities?.versionLabel).toBe('MySQL 8.0')
    expect(result.current.availableSchemas).toEqual(['apollo', 'mysql', 'TestDB'])
    expect(result.current.selectedSchema).toBeNull()
  })

  it('clears schema state when connection testing fails', async () => {
    const connect = vi
      .fn()
      .mockResolvedValueOnce(connectionResult)
      .mockRejectedValueOnce(new Error('connection failed'))
    const api = fakeApi({ connect })
    const { result } = renderHook(() => useRunnerController(api))

    await waitFor(() => expect(result.current.selectedProfile?.id).toBe('first'))
    await act(async () => {
      await result.current.connect()
    })
    act(() => result.current.setSelectedSchema('apollo'))

    await act(async () => {
      await result.current.connect()
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
      await result.current.connect()
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
      await result.current.connect()
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
