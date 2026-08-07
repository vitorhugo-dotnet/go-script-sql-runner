import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import App from '../App'

const profile = {
  id: 'local-dev',
  name: 'Local dev',
  version: 1,
  connection: {
    host: '127.0.0.1',
    port: 3306,
    database: 'apollo',
    username: 'root',
    password: 'secret',
  },
  execution: {
    onError: 'continue',
    transactionMode: 'auto_commit',
  },
  scripts: [
    {
      id: 'users',
      name: '001-users.sql',
      file: 'scripts/001-users.sql',
      enabled: true,
      order: 10,
      transactionMode: '',
    },
    {
      id: 'seed',
      name: '002-seed.sql',
      file: 'scripts/002-seed.sql',
      enabled: true,
      order: 20,
      transactionMode: 'transaction',
    },
  ],
}

function fakeApi() {
  let eventHandler: ((event: unknown) => void) | undefined

  const api = {
    listProfiles: vi.fn().mockResolvedValue([profile]),
    getProfile: vi.fn().mockResolvedValue(profile),
    createProfile: vi.fn().mockResolvedValue(profile),
    updateProfile: vi.fn().mockResolvedValue(profile),
    addScriptFromDialog: vi.fn().mockResolvedValue(null),
    addScriptsFromDialog: vi.fn().mockResolvedValue([]),
    removeScript: vi.fn().mockResolvedValue(undefined),
    reorderScripts: vi.fn().mockImplementation(async (_profileID: string, orderedIDs: string[]) => ({
      ...profile,
      scripts: orderedIDs.map((id, index) => ({
        ...profile.scripts.find((script) => script.id === id)!,
        order: (index + 1) * 10,
      })),
    })),
    setScriptEnabled: vi.fn().mockImplementation(async (_profileID: string, scriptID: string, enabled: boolean) => ({
      ...profile,
      scripts: profile.scripts.map((script) => (script.id === scriptID ? { ...script, enabled } : script)),
    })),
    setScriptTransactionMode: vi.fn().mockResolvedValue(profile),
    testConnection: vi.fn().mockResolvedValue({
      vendor: 'mysql',
      major: 8,
      minor: 0,
      patch: 39,
      rawVersion: '8.0.39',
      versionLabel: 'MySQL 8.0',
    }),
    runProfile: vi.fn().mockResolvedValue({ results: [], succeeded: 2, failed: 0, aborted: false }),
    stopRun: vi.fn().mockResolvedValue(true),
    importProfileFromDialog: vi.fn().mockResolvedValue(null),
    exportProfileToDialog: vi.fn().mockResolvedValue('profile.zip'),
    onExecutionEvent: vi.fn().mockImplementation((handler: (event: unknown) => void) => {
      eventHandler = handler
      return () => {
        eventHandler = undefined
      }
    }),
    emitExecutionEvent(event: unknown) {
      eventHandler?.(event)
    },
  }

  return api
}

describe('main runner workspace', () => {
  it('shows the essential runner controls in the main window', () => {
    render(<App />)

    expect(screen.getByLabelText('Profile')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Import' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Export' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Test Connection' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add SQL' })).toBeInTheDocument()
    expect(screen.getByLabelText('On failure')).toBeInTheDocument()
    expect(screen.getByLabelText('Transaction')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Run' })).toBeInTheDocument()
    expect(screen.getByText('Logs')).toBeInTheDocument()
  })

  it('loads the first profile and shows its safe connection summary', async () => {
    const api = fakeApi()

    render(<App api={api as never} />)

    expect(await screen.findByRole('option', { name: 'Local dev' })).toBeInTheDocument()
    expect(screen.getByText('127.0.0.1:3306 / apollo')).toBeInTheDocument()
    expect(screen.queryByText('secret')).not.toBeInTheDocument()
    expect(api.listProfiles).toHaveBeenCalledOnce()
    expect(api.getProfile).toHaveBeenCalledWith('local-dev')
  })

  it('tests the selected connection and shows the detected server version', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('127.0.0.1:3306 / apollo')
    await user.click(screen.getByRole('button', { name: 'Test Connection' }))

    expect(await screen.findByText('MySQL 8.0')).toBeInTheDocument()
    expect(api.testConnection).toHaveBeenCalledWith('local-dev')
  })

  it('opens the multi SQL operation from Add SQL', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Add SQL' }))

    expect(api.addScriptsFromDialog).toHaveBeenCalledWith('local-dev')
    expect(api.addScriptFromDialog).not.toHaveBeenCalled()
  })

  it('renders scripts and persists enable and reorder actions', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    expect(await screen.findByText('001-users.sql')).toBeInTheDocument()
    expect(screen.getByText('002-seed.sql')).toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: 'Enable 001-users.sql' }))
    expect(api.setScriptEnabled).toHaveBeenCalledWith('local-dev', 'users', false)

    await user.click(screen.getByRole('button', { name: 'Move 001-users.sql down' }))
    expect(api.reorderScripts).toHaveBeenCalledWith('local-dev', ['seed', 'users'])
  })

  it('sends run-only failure and transaction overrides without saving the profile', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.selectOptions(screen.getByLabelText('On failure'), 'stop')
    await user.selectOptions(screen.getByLabelText('Transaction'), 'transaction')
    await user.click(screen.getByRole('button', { name: 'Run' }))

    await waitFor(() =>
      expect(api.runProfile).toHaveBeenCalledWith('local-dev', {
        onError: 'stop',
        transactionMode: 'transaction',
      }),
    )
    expect(api.updateProfile).not.toHaveBeenCalled()
  })

  it('disables Run while executing and appends execution events with script context to logs', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    let finishRun: ((value: unknown) => void) | undefined
    api.runProfile.mockImplementation(
      () =>
        new Promise((resolve) => {
          finishRun = resolve
        }),
    )

    render(<App api={api as never} />)
    await screen.findByText('001-users.sql')

    const runButton = screen.getByRole('button', { name: 'Run' })
    await user.click(runButton)
    expect(runButton).toBeDisabled()

    act(() => {
      api.emitExecutionEvent({
        time: '2026-08-07T18:00:00Z',
        level: 'INFO',
        scriptId: 'users',
        message: 'Running 001-users.sql',
        detail: 'statement 1/4',
      })
    })
    expect(await screen.findByText('Running 001-users.sql')).toBeInTheDocument()
    expect(screen.getByText('[users]')).toBeInTheDocument()

    act(() => {
      finishRun?.({ results: [], succeeded: 2, failed: 0, aborted: false })
    })
    await waitFor(() => expect(runButton).not.toBeDisabled())
  })
})
