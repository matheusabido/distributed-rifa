import { useCallback, useEffect, useRef, useState } from 'react'
import { request } from '../api.js'

const initialState = { data: null, error: '' }

export default function useApiResource(path, token) {
  const [state, setState] = useState(initialState)
  const activeRequest = useRef(null)

  const reload = useCallback(async () => {
    activeRequest.current?.abort()
    const controller = new AbortController()
    activeRequest.current = controller

    try {
      const data = await request(path, { token, signal: controller.signal })
      if (!controller.signal.aborted) setState({ data, error: '' })
    } catch (error) {
      if (!controller.signal.aborted) {
        setState((current) => ({ ...current, error: error.message, status: error.status }))
      }
    }
  }, [path, token])

  useEffect(() => {
    void reload()
    return () => activeRequest.current?.abort()
  }, [reload])

  return { ...state, reload, setState }
}
