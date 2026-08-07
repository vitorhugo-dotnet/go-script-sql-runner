import { useCallback, useEffect, useState } from 'react'
import type { Profile, RunnerApi } from '../api/types'

export function useRunnerController(api: RunnerApi) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [selectedProfile, setSelectedProfile] = useState<Profile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const selectProfile = useCallback(
    async (profileID: string) => {
      if (!profileID) {
        setSelectedProfile(null)
        return
      }

      try {
        setError(null)
        const profile = await api.getProfile(profileID)
        setSelectedProfile(profile)
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause))
      }
    },
    [api],
  )

  const loadProfiles = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const loaded = await api.listProfiles()
      setProfiles(loaded)

      if (loaded.length === 0) {
        setSelectedProfile(null)
        return
      }

      const first = await api.getProfile(loaded[0].id)
      setSelectedProfile(first)
    } catch (cause) {
      setProfiles([])
      setSelectedProfile(null)
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setLoading(false)
    }
  }, [api])

  useEffect(() => {
    void loadProfiles()
  }, [loadProfiles])

  return {
    profiles,
    selectedProfile,
    loading,
    error,
    loadProfiles,
    selectProfile,
  }
}
