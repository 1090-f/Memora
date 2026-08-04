type SSECallback<T> = (data: T) => void;
type SSEErrorCallback = (error: Error) => void;
type SSECompleteCallback = () => void;

type ResponseMode = 'raw' | 'sse-json';

interface SSEClientOptions {
  url: string;
  headers?: Record<string, string>;
  onOpen?: SSECompleteCallback;
  onError?: SSEErrorCallback;
  onComplete?: SSECompleteCallback;
  responseMode?: ResponseMode;
}

class SSEClient<T> {
  private controller: AbortController;
  private reader: ReadableStreamDefaultReader<Uint8Array> | null;
  private textDecoder: TextDecoder;
  private buffer: string;
  private completed: boolean;

  constructor(private options: SSEClientOptions) {
    this.controller = new AbortController();
    this.reader = null;
    this.textDecoder = new TextDecoder();
    this.buffer = '';
    this.completed = false;
  }

  private finish() {
    if (!this.completed) {
      this.completed = true;
      this.options.onComplete?.();
    }
  }

  private cleanup(triggerComplete: boolean) {
    this.controller.abort();
    if (this.reader) {
      this.reader.cancel().catch(() => {});
    }
    this.reader = null;
    if (triggerComplete) {
      this.finish();
    }
  }

  public subscribe(body: BodyInit, onMessage: SSECallback<T>) {
    this.cleanup(false);
    this.controller = new AbortController();
    const { url, headers, onOpen, onError } = this.options;
    this.buffer = '';
    this.completed = false;

    const token = localStorage.getItem('panda_wiki_token') || '';

    const timeoutDuration = 300000;
    const timeoutId = setTimeout(() => {
      this.unsubscribe();
      onError?.(new Error('Request timed out after 5 minutes'));
    }, timeoutDuration);

    fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
        Authorization: `Bearer ${token}`,
        ...headers,
      },
      body,
      signal: this.controller.signal,
    })
      .then(async response => {
        if (!response.ok) {
          clearTimeout(timeoutId);
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        if (!response.body) {
          clearTimeout(timeoutId);
          onError?.(new Error('No response body'));
          return;
        }

        onOpen?.();
        this.reader = response.body.getReader();

        while (true) {
          const { done, value } = await this.reader.read();
          if (done) {
            clearTimeout(timeoutId);
            this.finish();
            break;
          }

          this.processChunk(value, onMessage);
        }
      })
      .catch(error => {
        clearTimeout(timeoutId);
        if (error.name !== 'AbortError') {
          onError?.(error);
        }
      });
  }

  private processChunk(
    chunk: Uint8Array | undefined,
    callback: SSECallback<T>,
  ) {
    if (!chunk) return;

    const text = this.textDecoder.decode(chunk, { stream: true });
    if (this.options.responseMode !== 'sse-json') {
      callback(text as T);
      return;
    }

    this.buffer += text;
    const events = this.buffer.split('\n\n');
    this.buffer = events.pop() || '';

    for (const event of events) {
      const dataLines = event
        .split('\n')
        .filter(line => line.startsWith('data:'))
        .map(line => line.slice(5).trim());

      if (dataLines.length === 0) {
        continue;
      }

      const payload = dataLines.join('\n');
      try {
        callback(JSON.parse(payload) as T);
      } catch {
        throw new Error('Invalid SSE JSON payload');
      }
    }
  }

  public unsubscribe() {
    this.cleanup(true);
  }
}

export default SSEClient;
