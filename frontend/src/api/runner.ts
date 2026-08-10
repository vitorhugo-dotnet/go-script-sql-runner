import type {
  ConnectionResult,
  ExecutionEvent,
  Profile,
  RunOptions,
  RunSummary,
  RunnerApi,
  Script,
  TransactionMode,
  UpdateInfo,
} from './types'

interface DesktopBinding {
  ListProfiles(): Promise<Profile[]>
  GetProfile(profileID: string): Promise<Profile>
  CreateProfile(profile: Profile): Promise<Profile>
  UpdateProfile(profile: Profile): Promise<Profile>
  AddScriptFromDialog(profileID: string): Promise<Script | null>
  AddScriptsFromDialog(profileID: string): Promise<Script[]>
  RemoveScript(profileID: string, scriptID: string): Promise<void>
  ReorderScripts(profileID: string, orderedIDs: string[]): Promise<Profile>
  SetScriptEnabled(profileID: string, scriptID: string, enabled: boolean): Promise<Profile>
  SetScriptTransactionMode(profileID: string, scriptID: string, mode: TransactionMode | ''): Promise<Profile>
  TestConnection(profileID: string): Promise<ConnectionResult>
  RunProfile(profileID: string, options: RunOptions): Promise<RunSummary>
  StopRun(): Promise<boolean>
  ImportProfileFromDialog(): Promise<Profile | null>
  ExportProfileToDialog(profileID: string): Promise<string>
  CheckForUpdates(): Promise<UpdateInfo>
}

interface WailsRuntime {
  EventsOn(eventName: string, callback: (payload?: unknown) => void): () => void
  BrowserOpenURL(url: string): void
}

declare global {
  interface Window {
    go?: {
      wailsui?: {
        DesktopApp?: DesktopBinding
      }
    }
    runtime?: WailsRuntime
  }
}

function desktop(): DesktopBinding {
  const binding = window.go?.wailsui?.DesktopApp
  if (!binding) {
    throw new Error('Wails desktop bindings are unavailable')
  }
  return binding
}

export const wailsRunnerApi: RunnerApi = {
  listProfiles: () => desktop().ListProfiles(),
  getProfile: (profileID) => desktop().GetProfile(profileID),
  createProfile: (profile) => desktop().CreateProfile(profile),
  updateProfile: (profile) => desktop().UpdateProfile(profile),
  addScriptFromDialog: (profileID) => desktop().AddScriptFromDialog(profileID),
  addScriptsFromDialog: (profileID) => desktop().AddScriptsFromDialog(profileID),
  removeScript: (profileID, scriptID) => desktop().RemoveScript(profileID, scriptID),
  reorderScripts: (profileID, orderedIDs) => desktop().ReorderScripts(profileID, orderedIDs),
  setScriptEnabled: (profileID, scriptID, enabled) => desktop().SetScriptEnabled(profileID, scriptID, enabled),
  setScriptTransactionMode: (profileID, scriptID, mode) =>
    desktop().SetScriptTransactionMode(profileID, scriptID, mode),
  testConnection: (profileID) => desktop().TestConnection(profileID),
  runProfile: (profileID, options) => desktop().RunProfile(profileID, options),
  stopRun: () => desktop().StopRun(),
  importProfileFromDialog: () => desktop().ImportProfileFromDialog(),
  exportProfileToDialog: (profileID) => desktop().ExportProfileToDialog(profileID),
  checkForUpdates: () => desktop().CheckForUpdates(),
  openExternalURL: (url) => {
    if (window.runtime?.BrowserOpenURL) {
      window.runtime.BrowserOpenURL(url)
      return
    }
    window.open(url, '_blank', 'noopener,noreferrer')
  },
  onExecutionEvent: (handler) => {
    if (!window.runtime?.EventsOn) {
      return () => undefined
    }
    return window.runtime.EventsOn('runner:execution-event', (payload) => {
      if (payload && typeof payload === 'object') {
        handler(payload as ExecutionEvent)
      }
    })
  },
}
