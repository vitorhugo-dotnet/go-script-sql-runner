import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import ScriptEditorDialog from './ScriptEditorDialog'

vi.mock('../monacoSetup', () => ({}))
vi.mock('@monaco-editor/react', () => ({
  default: ({ value, onChange }: { value: string; onChange: (value: string) => void }) => (
    <textarea aria-label="SQL content" value={value} onChange={(event) => onChange(event.target.value)} />
  ),
}))

const baseProps = {
  open: true,
  scriptName: '001-users.sql',
  initialContent: 'SELECT 1;\n',
  saving: false,
}

describe('ScriptEditorDialog', () => {
  it('loads exact SQL, edits multiline text, and closes only after a successful save', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn().mockResolvedValue(true)
    const onCancel = vi.fn()
    render(<ScriptEditorDialog {...baseProps} onSave={onSave} onCancel={onCancel} />)

    const dialog = screen.getByRole('dialog', { name: 'Edit 001-users.sql' })
    const editor = within(dialog).getByRole('textbox', { name: 'SQL content' })
    expect(editor).toHaveValue('SELECT 1;\n')
    await user.clear(editor)
    await user.type(editor, '-- Café{enter}SELECT 2;{enter}')
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))

    expect(onSave).toHaveBeenCalledWith('-- Café\nSELECT 2;\n')
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('keeps the draft and dialog open when save returns false', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn().mockResolvedValue(false)
    const onCancel = vi.fn()
    render(<ScriptEditorDialog {...baseProps} onSave={onSave} onCancel={onCancel} />)

    const editor = screen.getByRole('textbox', { name: 'SQL content' })
    await user.clear(editor)
    await user.type(editor, 'SELECT 3;')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onSave).toHaveBeenCalledWith('SELECT 3;')
    expect(onCancel).not.toHaveBeenCalled()
    expect(editor).toHaveValue('SELECT 3;')
    expect(screen.getByRole('dialog', { name: 'Edit 001-users.sql' })).toBeInTheDocument()
  })

  it('renders a save failure as an alert inside the editor dialog', () => {
    render(<ScriptEditorDialog {...baseProps} error="save failed" onSave={vi.fn()} onCancel={vi.fn()} />)

    const dialog = screen.getByRole('dialog', { name: 'Edit 001-users.sql' })
    expect(within(dialog).getByRole('alert')).toHaveTextContent('save failed')
  })

  it('disables Save while saving and cancels a clean editor immediately', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn().mockResolvedValue(true)
    const onCancel = vi.fn()
    const { rerender } = render(<ScriptEditorDialog {...baseProps} saving onSave={onSave} onCancel={onCancel} />)

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSave).not.toHaveBeenCalled()

    rerender(<ScriptEditorDialog {...baseProps} onSave={onSave} onCancel={onCancel} />)
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('keeps or discards a dirty draft through an in-app confirmation', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<ScriptEditorDialog {...baseProps} onSave={vi.fn().mockResolvedValue(true)} onCancel={onCancel} />)

    const editor = screen.getByRole('textbox', { name: 'SQL content' })
    await user.clear(editor)
    await user.type(editor, 'SELECT 4;')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    const discard = screen.getByRole('alertdialog', { name: 'Discard changes?' })
    expect(onCancel).not.toHaveBeenCalled()
    await user.click(within(discard).getByRole('button', { name: 'Keep editing' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(editor).toHaveValue('SELECT 4;')

    await user.click(screen.getByRole('button', { name: 'Close editor' }))
    await user.click(screen.getByRole('button', { name: 'Discard changes' }))
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('closes only the discard prompt on Escape and preserves the dirty editor draft', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<ScriptEditorDialog {...baseProps} onSave={vi.fn().mockResolvedValue(true)} onCancel={onCancel} />)

    const editor = screen.getByRole('textbox', { name: 'SQL content' })
    await user.clear(editor)
    await user.type(editor, 'SELECT 5;')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('alertdialog', { name: 'Discard changes?' })).toBeInTheDocument()

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Edit 001-users.sql' })).toBeInTheDocument()
    expect(editor).toHaveValue('SELECT 5;')
    expect(onCancel).not.toHaveBeenCalled()
  })
})
