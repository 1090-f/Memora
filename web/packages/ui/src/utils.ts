export const decodeBase64 = (text: string) => {
  if (typeof window !== 'undefined') {
    // 客户端逻辑 (使用 atob + 字节转换来处理 UTF-8)
    try {
      // 1. atob 解码 Base64 字符串为 Latin-1 字符串 (包含原始字节数据)
      const binaryString = window.atob(text);

      // 2. 将 Latin-1 字符串转换为字节数组 (Uint8Array)
      const len = binaryString.length;
      const bytes = new Uint8Array(len);
      for (let i = 0; i < len; i++) {
        bytes[i] = binaryString.charCodeAt(i);
      }

      // 3. 使用 TextDecoder (浏览器 API) 将字节数组转换为 UTF-8 字符串
      return new TextDecoder('utf-8').decode(bytes);
    } catch (e) {
      console.error('Client-side Base64/UTF-8 decoding failed:', e);
      return text; // 解码失败时返回原始文本
    }
  }

  // 服务端逻辑 (Node.js/Next.js SSR)
  try {
    const buff = Buffer.from(text, 'base64');
    return buff.toString('utf-8');
  } catch (e) {
    console.error('Server-side Base64 decoding failed:', e);
    return text;
  }
};
