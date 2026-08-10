import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import SchemaSelect from './SchemaSelect'

describe('SchemaSelect', () => {
  it('filters schemas case-insensitively and selects a filtered option', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()

    render(
      <SchemaSelect
        schemas={['apollo', 'mysql', 'TestDB']}
        value={null}
        onChange={onChange}
        disabled={false}
      />,
    )

    const input = screen.getByRole('combobox', { name: 'Schema' })
    await user.click(input)
    await user.type(input, 'test')

    expect(screen.getByRole('option', { name: 'TestDB' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'apollo' })).not.toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'mysql' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('option', { name: 'TestDB' }))

    expect(onChange).toHaveBeenLastCalledWith('TestDB')
    expect(input).toHaveValue('TestDB')
  })

  it('is disabled until schemas are available', () => {
    render(<SchemaSelect schemas={[]} value={null} onChange={() => undefined} disabled />)

    expect(screen.getByRole('combobox', { name: 'Schema' })).toBeDisabled()
  })
})
