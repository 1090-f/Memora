export function loadScript(url: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!url) {
      resolve();
      return;
    }

    const existing = Array.from(document.getElementsByTagName('script')).find(
      s => s.src === url,
    );
    if (existing) {
      if ((existing as any)._loaded) {
        resolve();
        return;
      }
      existing.addEventListener('load', () => resolve());
      existing.addEventListener('error', () =>
        reject(new Error(`Failed to load script: ${url}`)),
      );
      return;
    }

    const script = document.createElement('script');
    script.src = url;
    script.async = true;
    script.defer = true;
    (script as any)._loaded = false;
    script.addEventListener('load', () => {
      (script as any)._loaded = true;
      resolve();
    });
    script.addEventListener('error', () => {
      reject(new Error(`Failed to load script: ${url}`));
    });
    document.head.appendChild(script);
  });
}

export async function loadScriptsInOrder(urls: string[]): Promise<void> {
  for (const url of urls) {
    await loadScript(url);
  }
}
