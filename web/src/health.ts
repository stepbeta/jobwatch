import { useCallback, useEffect, useState } from 'react'
import { ApiError, api } from './api'

// ConnectionIssue is the shared cause of a failed API probe: the host was
// refused by the request policy, or the server did not answer at all.
export type ConnectionIssue = 'forbidden_host' | 'unreachable' | null

// useConnectionIssue checks the local API when the shell starts, when the
// browser restores the page and when the tab becomes visible again. It
// reports the shared cause once instead of letting every panel show its own
// copy of the same failure.
export function useConnectionIssue(): ConnectionIssue {
  const [issue, setIssue] = useState<ConnectionIssue>(null)

  const check = useCallback(async () => {
    try {
      await api.health()
      setIssue(null)
    } catch (err) {
      if (err instanceof ApiError && err.status === 403 && err.code === 'forbidden_host') {
        setIssue('forbidden_host')
      } else {
        setIssue('unreachable')
      }
    }
  }, [])

  useEffect(() => {
    void check()
    const recheck = () => void check()
    const onVisibility = () => {
      if (document.visibilityState === 'visible') void check()
    }
    window.addEventListener('pageshow', recheck)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      window.removeEventListener('pageshow', recheck)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [check])

  return issue
}

// connectionMessage explains a failed health check: the address to open and
// the flag that allows this host, or that the server is not answering.
export function connectionMessage(
  issue: Exclude<ConnectionIssue, null>,
  hostname: string,
): string {
  if (issue === 'forbidden_host') {
    const host = hostname || '<host>'
    return (
      `The API refused this host (${host}). Open the UI at ` +
      `http://127.0.0.1:8080 or the address serve printed, ` +
      `or start serve with --allow-host ${host}.`
    )
  }
  return 'The JobWatch API is not responding. Check that serve is running, then reload.'
}
