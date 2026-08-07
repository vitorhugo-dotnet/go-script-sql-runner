export type OnError = 'continue' | 'stop'
export type TransactionMode = 'auto_commit' | 'transaction' | 'script_managed'
export type ExecutionLevel = 'INFO' | 'WARN' | 'ERROR'

export interface Connection {
  host: string
  port: number
  database: string
  username: string
  password: string
}

export interface ExecutionConfig {
  onError: OnError
  transactionMode: TransactionMode
}

export interface Script {
  id: string
  name: string
  file: string
  enabled: boolean
  order: number
  transactionMode?: TransactionMode | ''
}

export interface Profile {
  id: string
  name: string
  version: number
  connection: Connection
  execution: ExecutionConfig
  scripts: Script[]
}

export interface ServerCapabilities {
  vendor: 'mysql' | 'mariadb' | string
  major: number
  minor: number
  patch: number
  rawVersion: string
  versionLabel: string
}

export interface ExecutionEvent {
  time: string
  level: ExecutionLevel
  scriptId?: string
  message: string
  detail?: string
}

export interface ScriptResult {
  scriptId: string
  name: string
  startedAt: string
  endedAt: string
  success: boolean
  error?: string
}

export interface RunSummary {
  results: ScriptResult[]
  succeeded: number
  failed: number
  aborted: boolean
}

export interface RunOptions {
  onError?: OnError | ''
  transactionMode?: TransactionMode | ''
}

export interface RunnerApi {
  listProfiles(): Promise<Profile[]>
  getProfile(profileID: string): Promise<Profile>
  createProfile(profile: Profile): Promise<Profile>
  updateProfile(profile: Profile): Promise<Profile>
  addScriptFromDialog(profileID: string): Promise<Script | null>
  removeScript(profileID: string, scriptID: string): Promise<void>
  reorderScripts(profileID: string, orderedIDs: string[]): Promise<Profile>
  setScriptEnabled(profileID: string, scriptID: string, enabled: boolean): Promise<Profile>
  setScriptTransactionMode(profileID: string, scriptID: string, mode: TransactionMode | ''): Promise<Profile>
  testConnection(profileID: string): Promise<ServerCapabilities>
  runProfile(profileID: string, options: RunOptions): Promise<RunSummary>
  stopRun(): Promise<boolean>
  importProfileFromDialog(): Promise<Profile | null>
  exportProfileToDialog(profileID: string): Promise<string>
  onExecutionEvent(handler: (event: ExecutionEvent) => void): () => void
}
