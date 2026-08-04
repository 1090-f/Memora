const SENSITIVE_KEYS = [
  'password',
  'token',
  'secret',
  'api_key',
  'apikey',
  'authorization',
  'cookie',
  'access_token',
  'refresh_token',
]

export function redactSensitive(obj: unknown): unknown {
  if (typeof obj !== 'object' || obj === null) return obj

  if (Array.isArray(obj)) {
    return obj.map(redactSensitive)
  }

  const result: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(obj as Record<string, unknown>)) {
    if (SENSITIVE_KEYS.some(sensitive => key.toLowerCase().includes(sensitive))) {
      result[key] = '***REDACTED***'
    } else if (typeof value === 'object' && value !== null) {
      result[key] = redactSensitive(value)
    } else {
      result[key] = value
    }
  }
  return result
}

export function redactForLog(data: unknown): string {
  try {
    return JSON.stringify(redactSensitive(data))
  } catch {
    return '[Unable to serialize]'
  }
}
