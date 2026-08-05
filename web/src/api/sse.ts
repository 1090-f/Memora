import type { ParsedAgentEvent } from '@/features/agent-run/types'
import { parseSseBlock } from '@/features/agent-run/events'
import { appErrorFromResponse } from './client'

export interface StreamAgentEventsOptions {
  url: string
  access_token: string
  signal: AbortSignal
  after_sequence?: number
  on_event: (event: ParsedAgentEvent) => void
}

export async function streamAgentEvents(options: StreamAgentEventsOptions): Promise<void> {
  const url = new URL(options.url, window.location.origin)
  if (options.after_sequence !== undefined) {
    url.searchParams.set('after_sequence', String(options.after_sequence))
  }

  const response = await fetch(url.toString(), {
    headers: {
      Authorization: `Bearer ${options.access_token}`,
      Accept: 'text/event-stream',
    },
    signal: options.signal,
  })

  if (!response.ok || !response.body) {
    throw await appErrorFromResponse(response)
  }

  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
  let buffer = ''

  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) break

      buffer += value.replace(/\r\n/g, '\n')
      let boundary = buffer.indexOf('\n\n')

      while (boundary >= 0) {
        const block = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)

        const parsed = parseSseBlock(block)
        if (parsed) {
          options.on_event(parsed)
        }

        boundary = buffer.indexOf('\n\n')
      }
    }
  } finally {
    reader.releaseLock()
  }
}
