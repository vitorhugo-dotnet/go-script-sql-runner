import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import SchemaSelect from './SchemaSelect'

const schemas = ['apollo', 'mysql', 'TestDB']

function renderSelector(onChange = vi.fn()) {
  render(
    <SchemaSelect
      schemas={schemas}
      value={null}
      onChange={onChange}
      disabled={false}
    />,
  )

  return {
    input: screen.getByRole('combobox', { name: 'Schema' }),
    onChange,
  }
}

describe('SchemaSelect', () => {
  it('filters schemas case-insensitively and selects a filtered option', async () => {
    const user = userEvent.setup()
    const { input, onChange } = renderSelector()

    await user.click(input)
    await user.type(input, 'test')

    expect(screen.getByRole('option', { name: 'TestDB' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'apollo' })).not.toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'mysql' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('option', { name: 'TestDB' }))

    expect(onChange).toHaveBeenLastCalledWith('TestDB')
    expect(input).toHaveValue('TestDB')
  })

  it('opens with the first visible schema active and navigates with arrow keys', async () => {
    const user = userEvent.setup()
    const { input } = renderSelector()

    await user.click(input)

    const apollo = screen.getByRole('option', { name: 'apollo' })
    const mysql = screen.getByRole('option', { name: 'mysql' })

    expect(apollo.id).not.toBe('')
    expect(mysql.id).not.toBe('')
    expect(input).toHaveAttribute('aria-activedescendant', apollo.id)

    await user.keyboard('{ArrowDown}')
    expect(input).toHaveAttribute('aria-activedescendant', mysql.id)

    await user.keyboard('{ArrowUp}')
    expect(input).toHaveAttribute('aria-activedescendant', apollo.id)
  })

  it('does not wrap keyboard navigation past list boundaries', async () => {
    const user = userEvent.setup()
    const { input } = renderSelector()

    await user.click(input)

    const apollo = screen.getByRole('option', { name: 'apollo' })
    const testDB = screen.getByRole('option', { name: 'TestDB' })

    await user.keyboard('{ArrowUp}{ArrowUp}')
    expect(input).toHaveAttribute('aria-activedescendant', apollo.id)

    await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}')
    expect(input).toHaveAttribute('aria-activedescendant', testDB.id)
  })

  it('commits only the active schema when Enter is pressed', async () => {
    const user = userEvent.setup()
    const { input, onChange } = renderSelector()

    await user.click(input)
    const apollo = screen.getByRole('option', { name: 'apollo' })
    const mysql = screen.getByRole('option', { name: 'mysql' })

    await user.keyboard('{ArrowDown}')

    expect(apollo).toHaveAttribute('aria-selected', 'false')
    expect(mysql).toHaveAttribute('aria-selected', 'false')
    expect(input).toHaveAttribute('aria-activedescendant', mysql.id)

    await user.keyboard('{Enter}')

    expect(onChange).toHaveBeenLastCalledWith('mysql')
    expect(input).toHaveValue('mysql')
    expect(input).toHaveAttribute('aria-expanded', 'false')
  })

  it('closes with Escape without committing the highlighted schema', async () => {
    const user = userEvent.setup()
    const { input, onChange } = renderSelector()

    await user.click(input)
    await user.keyboard('{ArrowDown}{Escape}')

    expect(onChange).not.toHaveBeenCalledWith('mysql')
    expect(input).toHaveAttribute('aria-expanded', 'false')
  })

  it('resets the active schema to the first filtered result', async () => {
    const user = userEvent.setup()
    const { input, onChange } = renderSelector()

    await user.click(input)
    await user.keyboard('{ArrowDown}')
    await user.type(input, 'test')

    const testDB = screen.getByRole('option', { name: 'TestDB' })
    expect(input).toHaveAttribute('aria-activedescendant', testDB.id)

    await user.keyboard('{Enter}')
    expect(onChange).toHaveBeenLastCalledWith('TestDB')
  })

  it('does not commit anything with Enter when filtering has no results', async () => {
    const user = userEvent.setup()
    const { input, onChange } = renderSelector()

    await user.click(input)
    await user.type(input, 'missing-schema')
    expect(screen.getByText('No schemas found')).toBeInTheDocument()

    onChange.mockClear()
    await user.keyboard('{Enter}')

    expect(onChange).not.toHaveBeenCalled()
  })

  it('scrolls the highlighted keyboard option into view', async () => {
    const user = userEvent.setup()
    const original = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollIntoView')
    const scrollIntoView = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })

    try {
      const { input } = renderSelector()
      await user.click(input)
      scrollIntoView.mockClear()

      await user.keyboard('{ArrowDown}')

      expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest' })
    } finally {
      if (original) {
        Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', original)
      } else {
        delete (HTMLElement.prototype as { scrollIntoView?: unknown }).scrollIntoView
      }
    }
  })

  it('is disabled until schemas are available', () => {
    render(<SchemaSelect schemas={[]} value={null} onChange={() => undefined} disabled />)

    expect(screen.getByRole('combobox', { name: 'Schema' })).toBeDisabled()
  })
})
