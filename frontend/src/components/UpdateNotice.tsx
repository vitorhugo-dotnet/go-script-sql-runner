import type { UpdateInfo } from '../api/types'

interface UpdateNoticeProps {
  info: UpdateInfo | null
  onOpen(url: string): void
}

export default function UpdateNotice({ info, onOpen }: UpdateNoticeProps) {
  if (!info?.available) return null

  const targetUrl = info.downloadUrl || info.releaseUrl
  if (!targetUrl) return null

  return (
    <button
      type="button"
      className="rounded-md border border-emerald-700 bg-emerald-950/70 px-2.5 py-1.5 text-xs font-medium text-emerald-200 transition hover:bg-emerald-900 focus:outline-none focus:ring-2 focus:ring-emerald-500"
      onClick={() => onOpen(targetUrl)}
    >
      Update {info.latestTag}
    </button>
  )
}
