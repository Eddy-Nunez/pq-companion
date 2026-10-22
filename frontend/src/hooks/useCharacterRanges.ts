import { useCallback, useEffect, useState } from 'react'
import { getCharacterRanges, listCharacters, type CharacterRanges } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'

/**
 * Returns the active character's effective spell and ranged-attack reach,
 * worked out from their gear (see buffmod.ComputeRanges). Re-reads when the
 * active character changes and when a Quarmy export is re-parsed (gear may
 * have changed). Null until loaded and on any error, so callers fall back to
 * the configured ranges.
 */
export function useCharacterRanges(): CharacterRanges | null {
  const [ranges, setRanges] = useState<CharacterRanges | null>(null)

  const read = useCallback(() => {
    listCharacters()
      .then((r) => {
        const active = r.characters.find(
          (c) => c.name.toLowerCase() === (r.active ?? '').toLowerCase(),
        )
        if (!active) {
          setRanges(null)
          return
        }
        return getCharacterRanges(active.id).then(setRanges)
      })
      .catch(() => setRanges(null))
  }, [])

  useEffect(() => {
    read()
  }, [read])

  const handle = useCallback(
    (msg: WsMessage) => {
      if (msg.type === WSEvent.ConfigCharacterDetected || msg.type === WSEvent.ZealQuarmy) {
        read()
      }
    },
    [read],
  )
  useWebSocket(handle)

  return ranges
}
