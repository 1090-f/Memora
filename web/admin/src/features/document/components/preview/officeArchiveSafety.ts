const ZIP_END_SIGNATURE = 0x06054b50;
const ZIP_CENTRAL_ENTRY_SIGNATURE = 0x02014b50;
const MAX_END_RECORD_SEARCH = 65_557;
const MAX_ARCHIVE_ENTRIES = 10_000;
const MAX_UNCOMPRESSED_BYTES = 256 * 1024 * 1024;
const MAX_SINGLE_ENTRY_BYTES = 128 * 1024 * 1024;
const MAX_COMPRESSION_RATIO = 200;

export function isZipArchive(buffer: ArrayBuffer) {
  return buffer.byteLength >= 4 && new DataView(buffer).getUint32(0, true) === 0x04034b50;
}

export function validateOfficeArchive(buffer: ArrayBuffer) {
  const view = new DataView(buffer);
  const searchStart = Math.max(0, buffer.byteLength - MAX_END_RECORD_SEARCH);
  let endOffset = -1;
  for (let offset = buffer.byteLength - 22; offset >= searchStart; offset -= 1) {
    if (view.getUint32(offset, true) === ZIP_END_SIGNATURE) {
      endOffset = offset;
      break;
    }
  }
  if (endOffset < 0) throw new Error('文件不是有效的 Office ZIP 文档');

  const entryCount = view.getUint16(endOffset + 10, true);
  const centralDirectorySize = view.getUint32(endOffset + 12, true);
  const centralDirectoryOffset = view.getUint32(endOffset + 16, true);
  if (entryCount > MAX_ARCHIVE_ENTRIES) throw new Error(`压缩包文件项过多（${entryCount}）`);
  if (centralDirectoryOffset + centralDirectorySize > buffer.byteLength) throw new Error('Office 文档目录结构损坏');

  let offset = centralDirectoryOffset;
  let totalUncompressedBytes = 0;
  for (let index = 0; index < entryCount; index += 1) {
    if (offset + 46 > buffer.byteLength || view.getUint32(offset, true) !== ZIP_CENTRAL_ENTRY_SIGNATURE) {
      throw new Error('Office 文档目录项损坏');
    }
    const compressedBytes = view.getUint32(offset + 20, true);
    const uncompressedBytes = view.getUint32(offset + 24, true);
    if (compressedBytes === 0xffffffff || uncompressedBytes === 0xffffffff) {
      throw new Error('浏览器预览暂不支持 ZIP64 Office 文档');
    }
    if (uncompressedBytes > MAX_SINGLE_ENTRY_BYTES) throw new Error('Office 文档内单个资源过大');
    if (compressedBytes > 0 && uncompressedBytes / compressedBytes > MAX_COMPRESSION_RATIO) {
      throw new Error('Office 文档压缩率异常，已停止预览');
    }
    totalUncompressedBytes += uncompressedBytes;
    if (totalUncompressedBytes > MAX_UNCOMPRESSED_BYTES) throw new Error('Office 文档解压后超过 256 MB');

    const fileNameLength = view.getUint16(offset + 28, true);
    const extraLength = view.getUint16(offset + 30, true);
    const commentLength = view.getUint16(offset + 32, true);
    offset += 46 + fileNameLength + extraLength + commentLength;
  }
}
