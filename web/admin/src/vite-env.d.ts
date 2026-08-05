/// <reference types="vite/client" />

declare module '*.html?raw' {
  const source: string;
  export default source;
}
/// <reference types="@memora/themes/types" />

declare module 'swiper/css' {
  const content: string;
  export default content;
}

declare module 'swiper/css/pagination' {
  const content: string;
  export default content;
}
