import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import UpdateNotice from './UpdateNotice'

describe('UpdateNotice', () => {
  it('opens the executable download when an update is available', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()

    render(
      <UpdateNotice
        info={{
          currentTag: 'build-41',
          latestTag: 'build-42',
          available: true,
          releaseUrl: 'https://github.com/renamed-owner/go-script-sql-runner/releases/tag/build-42',
          downloadUrl:
            'https://github.com/renamed-owner/go-script-sql-runner/releases/download/build-42/go-script-sql-runner.exe',
        }}
        onOpen={onOpen}
      />,
    )

    await user.click(screen.getByRole('button', { name: 'Update build-42' }))

    expect(onOpen).toHaveBeenCalledWith(
      'https://github.com/renamed-owner/go-script-sql-runner/releases/download/build-42/go-script-sql-runner.exe',
    )
  })

  it('renders nothing when the installed build is current', () => {
    const { container } = render(
      <UpdateNotice
        info={{
          currentTag: 'build-42',
          latestTag: 'build-42',
          available: false,
          releaseUrl: 'https://example.invalid/build-42',
          downloadUrl: '',
        }}
        onOpen={vi.fn()}
      />,
    )

    expect(container).toBeEmptyDOMElement()
  })
})
