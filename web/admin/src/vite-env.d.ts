/// <reference types="vite/client" />

declare module '*.html?raw' {
  const source: string;
  export default source;
}
