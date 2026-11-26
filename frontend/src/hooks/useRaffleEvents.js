import { useEffect, useEffectEvent } from 'react'
import { WS_URL } from '../api.js'

export default function useRaffleEvents(onEvent) {
  const handleEvent = useEffectEvent(onEvent)

  useEffect(() => {
    let socket
    let retry
    let stopped = false

    function connect() {
      try {
        socket = new WebSocket(WS_URL)
      } catch {
        return
      }

      socket.onopen = () => handleEvent(null)
      socket.onmessage = ({ data }) => {
        let event
        try {
          event = JSON.parse(data)
        } catch {
          return
        }

        if (event?.tipo && event.rifa_id != null) handleEvent(event)
      }
      socket.onclose = () => {
        if (!stopped) retry = setTimeout(connect, 2000)
      }
    }

    connect()
    return () => {
      stopped = true
      clearTimeout(retry)
      socket?.close()
    }
  }, [])
}
