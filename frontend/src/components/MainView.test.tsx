import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import App from '../App'

vi.mock('../monacoSetup', () => ({}))
vi.mock('@monaco-editor/react', () => ({
  default: ({ value, onChange }: { value: string; onChange: (value: string) => void }) => (
    <textarea aria-label="SQL content" value={value} onChange={(event) => onChange(event.target.value)} />
  ),
}))

const repositoryUrl = 'https://github.com/vitorhugo-dotnet/go-script-sql-runner'

const profile = {
  id: 'local-dev',
  name: 'Local dev',
  version: 1,
  connection: {
    host: '127.0.0.1',
    port: 3306,
    database: 'legacy-apollo',
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

function fakeApi() {
  let eventHandler: ((event: unknown) => void) | undefined

  const api = {
    listProfiles: vi.fn().mockResolvedValue([profile]),
    getProfile: vi.fn().mockResolvedValue(profile),
    createProfile: vi.fn().mockResolvedValue(profile),
    updateProfile: vi.fn().mockResolvedValue(profile),
    deleteProfile: vi.fn().mockResolvedValue(undefined),
    cloneProfile: vi.fn().mockResolvedValue({ ...profile, id: 'copy', name: 'Local dev (copy)' }),
    addScriptFromDialog: vi.fn().mockResolvedValue(null),
    addScriptsFromDialog: vi.fn().mockResolvedValue([]),
    removeScript: vi.fn().mockResolvedValue(undefined),
    getScriptContent: vi.fn().mockResolvedValue('SELECT 1;\n'),
    saveScriptContent: vi.fn().mockResolvedValue(undefined),
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
    connect: vi.fn().mockResolvedValue(connectionResult),
    runProfile: vi.fn().mockResolvedValue({ results: [], succeeded: 2, failed: 0, aborted: false }),
    stopRun: vi.fn().mockResolvedValue(true),
    importProfileFromDialog: vi.fn().mockResolvedValue(null),
    exportProfileToDialog: vi.fn().mockResolvedValue('profile.zip'),
    checkForUpdates: vi.fn().mockResolvedValue({
      currentTag: 'dev',
      latestTag: 'dev',
      available: false,
      releaseUrl: '',
      downloadUrl: '',
    }),
    openExternalURL: vi.fn().mockImplementation((url: string) => {
      window.runtime?.BrowserOpenURL(url)
    }),
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
  it('offers a named Edit action per script and saves exact SQL through the captured profile', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    expect(screen.getByRole('button', { name: 'Edit 001-users.sql' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit 002-seed.sql' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Edit 001-users.sql' }))

    expect(api.getScriptContent).toHaveBeenCalledWith('local-dev', 'users')
    const dialog = await screen.findByRole('dialog', { name: 'Edit 001-users.sql' })
    const editor = within(dialog).getByRole('textbox', { name: 'SQL content' })
    expect(editor).toHaveValue('SELECT 1;\n')
    await user.clear(editor)
    await user.type(editor, '-- Café{enter}SELECT 2;{enter}')
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))

    expect(api.saveScriptContent).toHaveBeenCalledWith('local-dev', 'users', '-- Café\nSELECT 2;\n')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Edit 001-users.sql' })).not.toBeInTheDocument())
  })

  it('shows load and save errors and retains the SQL draft after a failed save', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.getScriptContent.mockRejectedValueOnce(new Error('load failed')).mockResolvedValueOnce('SELECT 1;\n')
    api.saveScriptContent.mockRejectedValue(new Error('save failed'))
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Edit 001-users.sql' }))
    expect(await screen.findByText('load failed')).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: 'Edit 001-users.sql' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Edit 001-users.sql' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edit 001-users.sql' })
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
    const editor = within(dialog).getByRole('textbox', { name: 'SQL content' })
    await user.clear(editor)
    await user.type(editor, 'SELECT 3;')
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))

    expect(api.saveScriptContent).toHaveBeenCalledWith('local-dev', 'users', 'SELECT 3;')
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('save failed')
    expect(dialog).toBeInTheDocument()
    expect(editor).toHaveValue('SELECT 3;')
  })

  it('keeps the editor bound to its original profile after the selection changes', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    const clone = { ...profile, id: 'copy', name: 'Local dev (copy)' }
    api.listProfiles.mockResolvedValue([profile, clone])
    api.getProfile.mockImplementation(async (id: string) => (id === clone.id ? clone : profile))
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Edit 001-users.sql' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edit 001-users.sql' })
    fireEvent.change(screen.getByRole('combobox', { name: 'Profile' }), { target: { value: clone.id } })
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue(clone.id))
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))

    expect(api.saveScriptContent).toHaveBeenCalledWith(profile.id, 'users', 'SELECT 1;\n')
    expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue(clone.id)
  })

  it('clones the selected profile and selects the returned copy', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    const clone = { ...profile, id: 'copy', name: 'Local dev (copy)' }
    api.cloneProfile.mockResolvedValue(clone)
    api.listProfiles.mockResolvedValueOnce([profile]).mockResolvedValueOnce([profile, clone])
    api.getProfile.mockImplementation(async (id: string) => (id === clone.id ? clone : profile))
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Clone' }))

    expect(api.cloneProfile).toHaveBeenCalledWith('local-dev')
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue('copy'))
    expect(screen.getByRole('option', { name: 'Local dev (copy)' })).toBeInTheDocument()
  })

  it('requires named confirmation and Cancel never deletes', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Delete profile' })
    expect(within(dialog).getByText('Local dev')).toBeInTheDocument()
    expect(api.deleteProfile).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(api.deleteProfile).not.toHaveBeenCalled()
  })

  it('deletes the profile named in confirmation even if selection changes before confirming', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    const other = { ...profile, id: 'other', name: 'Other profile' }
    api.listProfiles.mockResolvedValueOnce([profile, other]).mockResolvedValueOnce([other])
    api.getProfile.mockImplementation(async (id: string) => (id === other.id ? other : profile))
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Delete profile' })
    expect(within(dialog).getByText('Local dev')).toBeInTheDocument()

    fireEvent.change(screen.getByRole('combobox', { name: 'Profile' }), { target: { value: other.id } })
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue(other.id))
    expect(within(dialog).getByText('Local dev')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))

    expect(api.deleteProfile).toHaveBeenCalledWith(profile.id)
    expect(api.deleteProfile).not.toHaveBeenCalledWith(other.id)
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue(other.id)
  })

  it('keeps confirmation open and shows an error when deletion fails', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.deleteProfile.mockRejectedValue(new Error('delete failed'))
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }))

    expect(api.deleteProfile).toHaveBeenCalledWith('local-dev')
    const dialog = screen.getByRole('alertdialog')
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('delete failed')
    expect(dialog).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue('local-dev')
  })

  it('closes confirmation and clears selection after deleting the last profile', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.listProfiles.mockResolvedValueOnce([profile]).mockResolvedValueOnce([])
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }))

    expect(api.deleteProfile).toHaveBeenCalledWith('local-dev')
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(screen.getByRole('combobox', { name: 'Profile' })).toHaveValue('')
    expect(screen.getByRole('option', { name: 'No profiles' })).toBeInTheDocument()
  })

  it('closes confirmation after committed deletion even when refresh fails', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.listProfiles.mockResolvedValueOnce([profile]).mockRejectedValueOnce(new Error('refresh failed'))
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(screen.getByRole('option', { name: 'No profiles' })).toBeInTheDocument()
    expect(screen.getByText('refresh failed')).toBeInTheDocument()
  })

  it('disables lifecycle actions when no profile is selected', async () => {
    const api = fakeApi()
    api.listProfiles.mockResolvedValue([])
    render(<App api={api as never} />)

    await screen.findByRole('option', { name: 'No profiles' })
    expect(screen.getByRole('button', { name: 'Clone' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Edit 001-users.sql' })).not.toBeInTheDocument()
  })

  it('shows the essential runner controls in the main window', () => {
    render(<App />)

    expect(screen.getByLabelText('Profile')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Import' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Export' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Connect' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add SQL' })).toBeInTheDocument()
    expect(screen.getByLabelText('On failure')).toBeInTheDocument()
    expect(screen.getByLabelText('Transaction')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Schema' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Run' })).toBeInTheDocument()
    expect(screen.getByText('Logs')).toBeInTheDocument()
  })

  it('loads the first profile and does not expose a persisted database in the connection summary', async () => {
    const api = fakeApi()

    render(<App api={api as never} />)

    expect(await screen.findByRole('option', { name: 'Local dev' })).toBeInTheDocument()
    expect(screen.getByText('127.0.0.1:3306')).toBeInTheDocument()
    expect(screen.queryByText('legacy-apollo')).not.toBeInTheDocument()
    expect(screen.queryByText('secret')).not.toBeInTheDocument()
    expect(api.listProfiles).toHaveBeenCalledOnce()
    expect(api.getProfile).toHaveBeenCalledWith('local-dev')
  })

  it('loads schemas after connection and does not auto-select one', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('127.0.0.1:3306')
    const schema = screen.getByRole('combobox', { name: 'Schema' })
    expect(schema).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Connect' }))

    expect(await screen.findByText('MySQL 8.0')).toBeInTheDocument()
    expect(schema).toBeEnabled()
    expect(schema).toHaveValue('')
    expect(screen.getByRole('button', { name: 'Run' })).toBeDisabled()
    expect(api.connect).toHaveBeenCalledWith('local-dev')
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

  it('sends selected runtime schema and run-only overrides without saving the profile', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<App api={api as never} />)

    await screen.findByText('001-users.sql')
    await user.click(screen.getByRole('button', { name: 'Connect' }))
    const schema = screen.getByRole('combobox', { name: 'Schema' })
    await user.click(schema)
    await user.click(screen.getByRole('option', { name: 'apollo' }))
    await user.selectOptions(screen.getByLabelText('On failure'), 'stop')
    await user.selectOptions(screen.getByLabelText('Transaction'), 'transaction')
    await user.click(screen.getByRole('button', { name: 'Run' }))

    await waitFor(() =>
      expect(api.runProfile).toHaveBeenCalledWith('local-dev', {
        schema: 'apollo',
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
    await user.click(screen.getByRole('button', { name: 'Connect' }))
    const schema = screen.getByRole('combobox', { name: 'Schema' })
    await user.click(schema)
    await user.click(screen.getByRole('option', { name: 'apollo' }))

    const runButton = screen.getByRole('button', { name: 'Run' })
    await user.click(runButton)
    expect(runButton).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Clone' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Edit 001-users.sql' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Edit 002-seed.sql' })).toBeDisabled()

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
    expect(screen.getByRole('button', { name: 'Clone' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Edit 001-users.sql' })).toBeEnabled()
  })

  it('opens the repository footer link through the Wails browser runtime', async () => {
    const user = userEvent.setup()
    const BrowserOpenURL = vi.fn()
    const api = fakeApi()

    window.runtime = {
      EventsOn: vi.fn().mockReturnValue(() => undefined),
      BrowserOpenURL,
    } as never

    try {
      render(<App api={api as never} />)
      await screen.findByText('001-users.sql')

      const repositoryLink = screen.getByRole('link', {
        name: 'GitHub · vitorhugo-dotnet/go-script-sql-runner',
      })
      expect(repositoryLink).toHaveAttribute('href', repositoryUrl)

      await user.click(repositoryLink)

      expect(BrowserOpenURL).toHaveBeenCalledWith(repositoryUrl)
    } finally {
      window.runtime = undefined
    }
  })
})
