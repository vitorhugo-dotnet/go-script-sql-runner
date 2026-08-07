import { useEffect, useState } from 'react'
import type { OnError, Profile, TransactionMode } from '../api/types'

interface ProfileDialogProps {
  open: boolean
  mode: 'create' | 'edit'
  profile: Profile | null
  onCancel: () => void
  onSave: (profile: Profile) => Promise<void>
}

function emptyProfile(): Profile {
  return {
    id: '',
    name: '',
    version: 1,
    connection: {
      host: '',
      port: 3306,
      database: '',
      username: '',
      password: '',
    },
    execution: {
      onError: 'continue',
      transactionMode: 'auto_commit',
    },
    scripts: [],
  }
}

export default function ProfileDialog({ open, mode, profile, onCancel, onSave }: ProfileDialogProps) {
  const [draft, setDraft] = useState<Profile>(() => profile ?? emptyProfile())
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    setDraft(profile ? structuredClone(profile) : emptyProfile())
    setSaving(false)
    setError(null)
  }, [open, profile])

  if (!open) return null

  const updateConnection = (field: keyof Profile['connection'], value: string | number) => {
    setDraft((current) => ({
      ...current,
      connection: { ...current.connection, [field]: value },
    }))
  }

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const name = draft.name.trim()
    const host = draft.connection.host.trim()
    const database = draft.connection.database.trim()
    const username = draft.connection.username.trim()
    const port = Number(draft.connection.port)

    if (!name || !host || !database || !username) {
      setError('Name, host, database and username are required.')
      return
    }
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      setError('Port must be an integer between 1 and 65535.')
      return
    }

    const validated: Profile = {
      ...draft,
      name,
      connection: {
        ...draft.connection,
        host,
        port,
        database,
        username,
      },
    }

    try {
      setSaving(true)
      setError(null)
      await onSave(validated)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setSaving(false)
    }
  }

  return (
    <dialog
      open
      aria-label={mode === 'create' ? 'New profile' : 'Edit profile'}
      className="fixed inset-0 z-50 m-auto w-[min(92vw,520px)] rounded-xl border border-slate-700 bg-slate-950 p-0 text-slate-100 shadow-2xl backdrop:bg-black/60"
      onCancel={(event) => {
        event.preventDefault()
        onCancel()
      }}
    >
      <form className="grid gap-3 p-4" onSubmit={submit}>
        <div>
          <h2 className="text-base font-semibold">{mode === 'create' ? 'New profile' : 'Edit profile'}</h2>
          <p className="mt-0.5 text-xs text-slate-500">Database credentials stay in the local profile configuration.</p>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <label className="col-span-2 grid gap-1 text-xs text-slate-300">
            Name
            <input
              autoFocus
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              value={draft.name}
              onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            />
          </label>

          <label className="grid gap-1 text-xs text-slate-300">
            Host
            <input
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              value={draft.connection.host}
              onChange={(event) => updateConnection('host', event.target.value)}
            />
          </label>

          <label className="grid gap-1 text-xs text-slate-300">
            Port
            <input
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              type="number"
              min={1}
              max={65535}
              value={draft.connection.port}
              onChange={(event) => updateConnection('port', Number(event.target.value))}
            />
          </label>

          <label className="col-span-2 grid gap-1 text-xs text-slate-300">
            Database / Schema
            <input
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              value={draft.connection.database}
              onChange={(event) => updateConnection('database', event.target.value)}
            />
          </label>

          <label className="grid gap-1 text-xs text-slate-300">
            Username
            <input
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              autoComplete="username"
              value={draft.connection.username}
              onChange={(event) => updateConnection('username', event.target.value)}
            />
          </label>

          <label className="grid gap-1 text-xs text-slate-300">
            Password
            <input
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              type="password"
              autoComplete="current-password"
              value={draft.connection.password}
              onChange={(event) => updateConnection('password', event.target.value)}
            />
          </label>

          <label className="grid gap-1 text-xs text-slate-300">
            Default failure policy
            <select
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              value={draft.execution.onError}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  execution: { ...current.execution, onError: event.target.value as OnError },
                }))
              }
            >
              <option value="continue">Continue</option>
              <option value="stop">Stop</option>
            </select>
          </label>

          <label className="grid gap-1 text-xs text-slate-300">
            Default transaction mode
            <select
              className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-sky-500"
              value={draft.execution.transactionMode}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  execution: {
                    ...current.execution,
                    transactionMode: event.target.value as TransactionMode,
                  },
                }))
              }
            >
              <option value="auto_commit">Auto commit</option>
              <option value="transaction">Transaction</option>
              <option value="script_managed">Script managed</option>
            </select>
          </label>
        </div>

        {error && <p role="alert" className="text-xs text-red-300">{error}</p>}

        <div className="flex justify-end gap-2 pt-1">
          <button
            type="button"
            className="rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-xs hover:bg-slate-800"
            disabled={saving}
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type="submit"
            className="rounded-md border border-sky-700 bg-sky-900 px-3 py-2 text-xs font-medium hover:bg-sky-800 disabled:opacity-50"
            disabled={saving}
          >
            {saving ? 'Saving…' : 'Save profile'}
          </button>
        </div>
      </form>
    </dialog>
  )
}
