import { useState } from 'react'
import { wailsRunnerApi } from './api/runner'
import type { OnError, RunnerApi, TransactionMode } from './api/types'
import ProfileDialog from './components/ProfileDialog'
import { useRunnerController } from './state/useRunnerController'

const buttonClass =
  'rounded-md border border-slate-700 bg-slate-800 px-2.5 py-1.5 text-xs font-medium text-slate-100 transition hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:cursor-not-allowed disabled:opacity-45'

const selectClass =
  'rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-xs text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:cursor-not-allowed disabled:opacity-45'

interface AppProps {
  api?: RunnerApi
}

function formatEventTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

export default function App({ api = wailsRunnerApi }: AppProps) {
  const controller = useRunnerController(api)
  const [detailedLogs, setDetailedLogs] = useState(false)
  const [profileDialogMode, setProfileDialogMode] = useState<'create' | 'edit' | null>(null)
  const profile = controller.selectedProfile
  const scripts = [...(profile?.scripts ?? [])].sort((left, right) => left.order - right.order)
  const connectionSummary = profile
    ? `${profile.connection.host}:${profile.connection.port} / ${profile.connection.database}`
    : 'No connection configured'

  return (
    <>
      <main className="grid h-full min-h-0 grid-rows-[auto_auto_minmax(0,1fr)_132px] bg-slate-950 text-slate-100">
        <header className="flex items-center gap-2 border-b border-slate-800 px-3 py-2">
          <label className="text-xs font-medium text-slate-300" htmlFor="profile-select">
            Profile
          </label>
          <select
            id="profile-select"
            className={`${selectClass} min-w-36`}
            value={profile?.id ?? ''}
            onChange={(event) => void controller.selectProfile(event.target.value)}
            disabled={controller.loading || controller.running}
          >
            {controller.profiles.length === 0 && <option value="">No profiles</option>}
            {controller.profiles.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <button
            className={buttonClass}
            type="button"
            disabled={controller.running}
            onClick={() => setProfileDialogMode('create')}
          >
            New
          </button>
          <button
            className={buttonClass}
            type="button"
            disabled={!profile || controller.running}
            onClick={() => setProfileDialogMode('edit')}
          >
            Edit
          </button>
          <div className="flex-1" />
          <button
            className={buttonClass}
            type="button"
            disabled={controller.running}
            onClick={() => void controller.importProfile()}
          >
            Import
          </button>
          <button
            className={buttonClass}
            type="button"
            disabled={!profile || controller.running}
            onClick={() => void controller.exportProfile()}
          >
            Export
          </button>
        </header>

        <section className="grid gap-2 border-b border-slate-800 px-3 py-2 sm:grid-cols-[minmax(0,1fr)_auto]">
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate text-xs text-slate-400">{connectionSummary}</span>
            <span className="rounded-full border border-slate-700 bg-slate-900 px-2 py-0.5 text-[11px] text-slate-400">
              {controller.capabilities?.versionLabel ?? 'Disconnected'}
            </span>
            <button
              className={buttonClass}
              type="button"
              disabled={!profile || controller.running}
              onClick={() => void controller.testConnection()}
            >
              Test Connection
            </button>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <label className="flex items-center gap-1 text-xs text-slate-300">
              <span>On failure</span>
              <select
                aria-label="On failure"
                className={selectClass}
                value={controller.runOnError}
                onChange={(event) => controller.setRunOnError(event.target.value as OnError)}
                disabled={!profile || controller.running}
              >
                <option value="continue">Continue</option>
                <option value="stop">Stop</option>
              </select>
            </label>
            <label className="flex items-center gap-1 text-xs text-slate-300">
              <span>Transaction</span>
              <select
                aria-label="Transaction"
                className={selectClass}
                value={controller.runTransactionMode}
                onChange={(event) => controller.setRunTransactionMode(event.target.value as TransactionMode)}
                disabled={!profile || controller.running}
              >
                <option value="auto_commit">Auto commit</option>
                <option value="transaction">Transaction</option>
                <option value="script_managed">Script managed</option>
              </select>
            </label>
            <button
              className={`${buttonClass} border-sky-700 bg-sky-900/70 hover:bg-sky-800`}
              type="button"
              disabled={!profile || controller.running}
              onClick={() => void controller.run()}
            >
              Run
            </button>
            {controller.running && (
              <button className={buttonClass} type="button" onClick={() => void controller.stopRun()}>
                Stop
              </button>
            )}
          </div>
        </section>

        <section className="flex min-h-0 flex-col px-3 py-2">
          <div className="mb-2 flex items-center justify-between">
            <h1 className="text-sm font-semibold">Scripts</h1>
            <button
              className={buttonClass}
              type="button"
              disabled={!profile || controller.running}
              onClick={() => void controller.addSQLFile()}
            >
              Add SQL
            </button>
          </div>
          <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-slate-800 bg-slate-900/50">
            {scripts.length === 0 ? (
              <div className="flex h-full items-center justify-center px-4 text-center text-xs text-slate-500">
                {profile ? 'No SQL scripts configured.' : 'Select or create a profile to add SQL scripts.'}
              </div>
            ) : (
              <div className="divide-y divide-slate-800">
                {scripts.map((script, index) => (
                  <div key={script.id} className="grid grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-2 px-2.5 py-2">
                    <input
                      type="checkbox"
                      checked={script.enabled}
                      disabled={controller.running}
                      aria-label={`Enable ${script.name}`}
                      onChange={(event) => void controller.setScriptEnabled(script.id, event.target.checked)}
                    />
                    <div className="min-w-0">
                      <div className="truncate text-xs font-medium text-slate-200">{script.name}</div>
                      <div className="truncate text-[10px] text-slate-500">{script.file}</div>
                    </div>
                    <select
                      className={selectClass}
                      value={script.transactionMode ?? ''}
                      disabled={controller.running}
                      aria-label={`Transaction mode for ${script.name}`}
                      onChange={(event) =>
                        void controller.setScriptTransactionMode(
                          script.id,
                          event.target.value as TransactionMode | '',
                        )
                      }
                    >
                      <option value="">Profile default</option>
                      <option value="auto_commit">Auto commit</option>
                      <option value="transaction">Transaction</option>
                      <option value="script_managed">Script managed</option>
                    </select>
                    <div className="flex items-center gap-1">
                      <button
                        className={buttonClass}
                        type="button"
                        aria-label={`Move ${script.name} up`}
                        disabled={controller.running || index === 0}
                        onClick={() => void controller.moveScript(script.id, -1)}
                      >
                        ↑
                      </button>
                      <button
                        className={buttonClass}
                        type="button"
                        aria-label={`Move ${script.name} down`}
                        disabled={controller.running || index === scripts.length - 1}
                        onClick={() => void controller.moveScript(script.id, 1)}
                      >
                        ↓
                      </button>
                      <button
                        className={buttonClass}
                        type="button"
                        aria-label={`Remove ${script.name}`}
                        disabled={controller.running}
                        onClick={() => void controller.removeScript(script.id)}
                      >
                        ×
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </section>

        <section className="flex min-h-0 flex-col border-t border-slate-800 bg-slate-950 px-3 py-2">
          <div className="mb-1 flex items-center gap-3">
            <h2 className="text-xs font-semibold text-slate-300">Logs</h2>
            <label className="flex items-center gap-1 text-[11px] text-slate-400">
              <input
                type="checkbox"
                checked={detailedLogs}
                onChange={(event) => setDetailedLogs(event.target.checked)}
              />
              Detailed logs
            </label>
            {controller.running && <span className="text-[11px] text-sky-400">Running…</span>}
            <button
              className="ml-auto text-[11px] text-slate-400 hover:text-slate-200"
              type="button"
              onClick={controller.clearLogs}
            >
              Clear
            </button>
          </div>
          <div className="min-h-0 flex-1 overflow-auto rounded-md border border-slate-800 bg-slate-900/60 px-2 py-1.5 text-[11px] text-slate-400">
            {controller.error && <div className="mb-1 text-red-300">{controller.error}</div>}
            {controller.logs.length === 0 && !controller.error && (
              <span className="text-slate-500">Execution logs will appear here.</span>
            )}
            {controller.logs.map((event, index) => (
              <div
                key={`${event.time}-${event.scriptId ?? 'runner'}-${index}`}
                className="grid grid-cols-[64px_42px_auto_minmax(0,1fr)] gap-2 font-mono"
              >
                <span className="shrink-0 text-slate-600">{formatEventTime(event.time)}</span>
                <span
                  className={
                    event.level === 'ERROR'
                      ? 'text-red-300'
                      : event.level === 'WARN'
                        ? 'text-amber-300'
                        : 'text-slate-500'
                  }
                >
                  {event.level}
                </span>
                <span className="text-slate-500">{event.scriptId ? `[${event.scriptId}]` : '[runner]'}</span>
                <span className="min-w-0 text-slate-300">
                  {event.message}
                  {detailedLogs && event.detail ? <span className="text-slate-500"> · {event.detail}</span> : null}
                </span>
              </div>
            ))}
          </div>
        </section>
      </main>

      <ProfileDialog
        open={profileDialogMode !== null}
        mode={profileDialogMode ?? 'create'}
        profile={profileDialogMode === 'edit' ? profile : null}
        onCancel={() => setProfileDialogMode(null)}
        onSave={async (draft) => {
          await controller.saveProfile(draft)
          setProfileDialogMode(null)
        }}
      />
    </>
  )
}
