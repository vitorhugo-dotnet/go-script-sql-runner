import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

interface ProfileDeleteDialogProps {
  open: boolean
  profileName: string
  error?: string | null
  onCancel: () => void
  onConfirm: () => Promise<void>
}

export default function ProfileDeleteDialog({ open, profileName, error, onCancel, onConfirm }: ProfileDeleteDialogProps) {
  const [deleting, setDeleting] = useState(false)
  const [portal] = useState(() => document.createElement('div'))
  const dialogRef = useRef<HTMLDivElement>(null)
  const cancelRef = useRef<HTMLButtonElement>(null)
  const confirmRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    document.body.appendChild(portal)
    const background = Array.from(document.body.children).filter((element) => element !== portal)
    const previousInert = background.map((element) => element.hasAttribute('inert'))
    background.forEach((element) => element.setAttribute('inert', ''))
    cancelRef.current?.focus()
    return () => {
      background.forEach((element, index) => {
        if (!previousInert[index]) element.removeAttribute('inert')
      })
      portal.remove()
      previousFocus?.focus()
    }
  }, [open, portal])

  useEffect(() => {
    if (deleting) dialogRef.current?.focus()
  }, [deleting])

  if (!open) return null

  const confirm = async () => {
    if (deleting) return
    setDeleting(true)
    try {
      await onConfirm()
    } finally {
      setDeleting(false)
    }
  }

  return createPortal(
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/70 px-4">
      <div
        ref={dialogRef}
        tabIndex={-1}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="profile-delete-title"
        aria-describedby="profile-delete-description"
        className="w-[min(92vw,420px)] rounded-xl border border-slate-700 bg-slate-950 p-4 text-slate-100 shadow-2xl"
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.preventDefault()
            if (!deleting) onCancel()
          }
          if (event.key === 'Tab') {
            const first = cancelRef.current
            const last = confirmRef.current
            if (deleting || !first || !last) {
              event.preventDefault()
              return
            }
            if (event.shiftKey && (document.activeElement === first || document.activeElement === dialogRef.current)) {
              event.preventDefault()
              last.focus()
            } else if (!event.shiftKey && (document.activeElement === last || document.activeElement === dialogRef.current)) {
              event.preventDefault()
              first.focus()
            }
          }
        }}
      >
        <h2 id="profile-delete-title" className="text-base font-semibold">Delete profile</h2>
        <p id="profile-delete-description" className="mt-2 text-sm text-slate-300">
          Delete <strong className="text-slate-100">{profileName}</strong>? Its stored scripts will be removed with it.
        </p>
        {error && <p role="alert" className="mt-3 text-sm text-red-300">{error}</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button
            ref={cancelRef}
            type="button"
            className="rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-slate-100 hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:opacity-45"
            disabled={deleting}
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            ref={confirmRef}
            type="button"
            className="rounded-md border border-red-700 bg-red-900/70 px-3 py-2 text-sm text-red-100 hover:bg-red-800 focus:outline-none focus:ring-2 focus:ring-red-500 disabled:opacity-45"
            disabled={deleting}
            onClick={() => void confirm()}
          >
            Delete
          </button>
        </div>
      </div>
    </div>,
    portal,
  )
}
