export type BrowserOfficeType = 'docx' | 'pptx' | 'spreadsheet';

export function resolveBrowserOfficeType(fileName?: string, mediaType?: string): BrowserOfficeType | undefined {
  const normalizedName = fileName?.trim().toLowerCase() ?? '';
  const extension = normalizedName.includes('.') ? normalizedName.slice(normalizedName.lastIndexOf('.')) : '';
  const normalizedMediaType = mediaType?.split(';', 1)[0]?.trim().toLowerCase() ?? '';
  if (extension === '.docx' || normalizedMediaType.includes('wordprocessingml')) return 'docx';
  if (extension === '.pptx' || normalizedMediaType.includes('presentationml')) return 'pptx';
  if (
    extension === '.xlsx' || extension === '.xls' || extension === '.csv'
    || normalizedMediaType.includes('spreadsheetml')
    || normalizedMediaType === 'application/vnd.ms-excel'
    || normalizedMediaType === 'text/csv'
  ) return 'spreadsheet';
  return undefined;
}
