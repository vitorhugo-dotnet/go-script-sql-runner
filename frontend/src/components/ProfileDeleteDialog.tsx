import { useState } from 'react'

interface ProfileDeleteDialogProps {
  open: boolean
  profileName: string
  onCancel: () => void
  onConfirm: () => Promise<void>
}

export default function ProfileDeleteDialog({ open, profileName, onCancel, onConfirm }: ProfileDeleteDialogProps) {
  const [deleting, setDeleting] = useState(false)

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

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/70 px-4">
      <div
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
        }}
      >
        <h2 id="profile-delete-title" className="text-base font-semibold">Delete profile</h2>
        <p id="profile-delete-description" className="mt-2 text-sm text-slate-300">
          Delete <strong className="text-slate-100">{profileName}</strong>? Its stored scripts will be removed with it.
        </p>
        <div className="mt-5 flex justify-end gap-2">
          <button
            autoFocus
            type="button"
            className="rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-slate-100 hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:opacity-45"
            disabled={deleting}
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type="button"
            className="rounded-md border border-red-700 bg-red-900/70 px-3 py-2 text-sm text-red-100 hover:bg-red-800 focus:outline-none focus:ring-2 focus:ring-red-500 disabled:opacity-45"
            disabled={deleting}
            onClick={() => void confirm()}
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  )
}
