import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import ProfileDeleteDialog from './ProfileDeleteDialog'

describe('ProfileDeleteDialog', () => {
  it('names the selected profile and explains that its stored scripts are removed', () => {
    render(
      <ProfileDeleteDialog open profileName="Production" onCancel={vi.fn()} onConfirm={vi.fn()} />,
    )

    const dialog = screen.getByRole('alertdialog', { name: 'Delete profile' })
    expect(within(dialog).getByText('Production')).toBeInTheDocument()
    expect(within(dialog).getByText(/stored scripts/i)).toBeInTheDocument()
  })

  it('cancels without confirming and confirms only after an explicit Delete click', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    const onConfirm = vi.fn().mockResolvedValue(undefined)
    render(<ProfileDeleteDialog open profileName="Production" onCancel={onCancel} onConfirm={onConfirm} />)

    const dialog = screen.getByRole('alertdialog', { name: 'Delete profile' })
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()

    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('renders nothing when closed', () => {
    render(<ProfileDeleteDialog open={false} profileName="Production" onCancel={vi.fn()} onConfirm={vi.fn()} />)
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('renders a delete failure as an alert inside the confirmation dialog', () => {
    render(<ProfileDeleteDialog open profileName="Production" error="delete failed" onCancel={vi.fn()} onConfirm={vi.fn()} />)

    const dialog = screen.getByRole('alertdialog', { name: 'Delete profile' })
    expect(within(dialog).getByRole('alert')).toHaveTextContent('delete failed')
  })

  it('focuses Cancel, keeps focus inside, and makes the background inert', async () => {
    const user = userEvent.setup()
    const { container } = render(
      <ProfileDeleteDialog open profileName="Production" onCancel={vi.fn()} onConfirm={vi.fn()} />,
    )
    const dialog = screen.getByRole('alertdialog', { name: 'Delete profile' })
    const cancel = within(dialog).getByRole('button', { name: 'Cancel' })
    const confirm = within(dialog).getByRole('button', { name: 'Delete' })

    expect(container).toHaveAttribute('inert')
    expect(cancel).toHaveFocus()
    await user.tab()
    expect(confirm).toHaveFocus()
    await user.tab()
    expect(cancel).toHaveFocus()
    await user.tab({ shift: true })
    expect(confirm).toHaveFocus()
  })

  it('restores background interaction and the previous focus when closed', async () => {
    const user = userEvent.setup()
    const props = { profileName: 'Production', onCancel: vi.fn(), onConfirm: vi.fn() }
    const { container, rerender } = render(
      <><button type="button">Open deletion</button><ProfileDeleteDialog open={false} {...props} /></>,
    )
    const trigger = screen.getByRole('button', { name: 'Open deletion' })
    await user.click(trigger)

    rerender(<><button type="button">Open deletion</button><ProfileDeleteDialog open {...props} /></>)
    expect(container).toHaveAttribute('inert')
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus()

    rerender(<><button type="button">Open deletion</button><ProfileDeleteDialog open={false} {...props} /></>)
    expect(container).not.toHaveAttribute('inert')
    expect(trigger).toHaveFocus()
  })
})
