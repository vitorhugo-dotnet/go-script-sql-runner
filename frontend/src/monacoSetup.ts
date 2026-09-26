import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'

type MonacoGlobal = typeof globalThis & {
  MonacoEnvironment?: { getWorker: (_workerID: string, _label: string) => Worker }
}

;(self as MonacoGlobal).MonacoEnvironment = {
  getWorker: () => new EditorWorker(),
}

loader.config({ monaco })
