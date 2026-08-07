import { render, screen } from '@testing-library/react'
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
  scripts: [],
}

function fakeApi() {
  return {
    listProfiles: vi.fn().mockResolvedValue([profile]),
    getProfile: vi.fn().mockResolvedValue(profile),
    onExecutionEvent: vi.fn().mockReturnValue(() => undefined),
  }
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
})
