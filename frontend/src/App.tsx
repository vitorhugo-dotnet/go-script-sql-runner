import type { RunnerApi } from './api/types'
import { wailsRunnerApi } from './api/runner'
import { useRunnerController } from './state/useRunnerController'

const buttonClass =
  'rounded-md border border-slate-700 bg-slate-800 px-2.5 py-1.5 text-xs font-medium text-slate-100 transition hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-sky-500'

const selectClass =
  'rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-xs text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500'

interface AppProps {
  api?: RunnerApi
}

export default function App({ api = wailsRunnerApi }: AppProps) {
  const controller = useRunnerController(api)
  const profile = controller.selectedProfile
  const connectionSummary = profile
    ? `${profile.connection.host}:${profile.connection.port} / ${profile.connection.database}`
    : 'No connection configured'

  return (
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
          disabled={controller.loading}
        >
          {controller.profiles.length === 0 && <option value="">No profiles</option>}
          {controller.profiles.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
        <button className={buttonClass} type="button">
          New
        </button>
        <button className={buttonClass} type="button" disabled={!profile}>
          Edit
        </button>
        <div className="flex-1" />
        <button className={buttonClass} type="button">
          Import
        </button>
        <button className={buttonClass} type="button" disabled={!profile}>
          Export
        </button>
      </header>

      <section className="grid gap-2 border-b border-slate-800 px-3 py-2 sm:grid-cols-[minmax(0,1fr)_auto]">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-xs text-slate-400">{connectionSummary}</span>
          <span className="rounded-full border border-slate-700 bg-slate-900 px-2 py-0.5 text-[11px] text-slate-400">
            Disconnected
          </span>
          <button className={buttonClass} type="button" disabled={!profile}>
            Test Connection
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-1 text-xs text-slate-300">
            <span>On failure</span>
            <select
              className={selectClass}
              value={profile?.execution.onError ?? 'continue'}
              onChange={() => undefined}
              disabled={!profile}
            >
              <option value="continue">Continue</option>
              <option value="stop">Stop</option>
            </select>
          </label>
          <label className="flex items-center gap-1 text-xs text-slate-300">
            <span>Transaction</span>
            <select
              className={selectClass}
              value={profile?.execution.transactionMode ?? 'auto_commit'}
              onChange={() => undefined}
              disabled={!profile}
            >
              <option value="auto_commit">Auto commit</option>
              <option value="transaction">Transaction</option>
              <option value="script_managed">Script managed</option>
            </select>
          </label>
          <button
            className={`${buttonClass} border-sky-700 bg-sky-900/70 hover:bg-sky-800`}
            type="button"
            disabled={!profile}
          >
            Run
          </button>
        </div>
      </section>

      <section className="flex min-h-0 flex-col px-3 py-2">
        <div className="mb-2 flex items-center justify-between">
          <h1 className="text-sm font-semibold">Scripts</h1>
          <button className={buttonClass} type="button" disabled={!profile}>
            Add SQL
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-slate-800 bg-slate-900/50">
          <div className="flex h-full items-center justify-center px-4 text-center text-xs text-slate-500">
            {profile ? 'No SQL scripts configured.' : 'Select or create a profile to add SQL scripts.'}
          </div>
        </div>
      </section>

      <section className="flex min-h-0 flex-col border-t border-slate-800 bg-slate-950 px-3 py-2">
        <div className="mb-1 flex items-center gap-3">
          <h2 className="text-xs font-semibold text-slate-300">Logs</h2>
          <label className="flex items-center gap-1 text-[11px] text-slate-400">
            <input type="checkbox" /> Detailed logs
          </label>
          <button className="ml-auto text-[11px] text-slate-400 hover:text-slate-200" type="button">
            Clear
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-auto rounded-md border border-slate-800 bg-slate-900/60 px-2 py-1.5 text-[11px] text-slate-500">
          {controller.error ?? 'Execution logs will appear here.'}
        </div>
      </section>
    </main>
  )
}
