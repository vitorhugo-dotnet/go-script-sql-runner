import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import ProfileDialog from './ProfileDialog'
import type { Profile } from '../api/types'

const existing: Profile = {
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

describe('ProfileDialog', () => {
  it('renders the complete profile form with a masked password', () => {
    render(<ProfileDialog open mode="edit" profile={existing} onCancel={() => undefined} onSave={vi.fn()} />)

    expect(screen.getByLabelText('Name')).toHaveValue('Local dev')
    expect(screen.getByLabelText('Host')).toHaveValue('127.0.0.1')
    expect(screen.getByLabelText('Port')).toHaveValue(3306)
    expect(screen.getByLabelText('Database / Schema')).toHaveValue('apollo')
    expect(screen.getByLabelText('Username')).toHaveValue('root')
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password')
    expect(screen.getByLabelText('Default failure policy')).toHaveValue('continue')
    expect(screen.getByLabelText('Default transaction mode')).toHaveValue('auto_commit')
  })

  it('submits one validated draft for a new profile', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn().mockResolvedValue(undefined)

    render(<ProfileDialog open mode="create" profile={null} onCancel={() => undefined} onSave={onSave} />)

    await user.type(screen.getByLabelText('Name'), 'Production')
    await user.type(screen.getByLabelText('Host'), 'db.internal')
    await user.clear(screen.getByLabelText('Port'))
    await user.type(screen.getByLabelText('Port'), '3307')
    await user.type(screen.getByLabelText('Database / Schema'), 'app')
    await user.type(screen.getByLabelText('Username'), 'runner')
    await user.type(screen.getByLabelText('Password'), 'topsecret')
    await user.selectOptions(screen.getByLabelText('Default failure policy'), 'stop')
    await user.selectOptions(screen.getByLabelText('Default transaction mode'), 'transaction')
    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    expect(onSave).toHaveBeenCalledWith({
      id: '',
      name: 'Production',
      version: 1,
      connection: {
        host: 'db.internal',
        port: 3307,
        database: 'app',
        username: 'runner',
        password: 'topsecret',
      },
      execution: {
        onError: 'stop',
        transactionMode: 'transaction',
      },
      scripts: [],
    })
  })
})
