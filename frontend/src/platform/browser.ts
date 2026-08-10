export function openExternalUrl(url: string) {
  if (!window.runtime?.BrowserOpenURL) {
    throw new Error('Wails BrowserOpenURL runtime is unavailable')
  }

  window.runtime.BrowserOpenURL(url)
}
