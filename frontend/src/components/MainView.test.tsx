import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from '../App'

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
})
