export const getBasePath = (path: string) => {
  if (!path || path.startsWith('http') || path.startsWith('blob')) {
    return path;
  }
  const basePathValue = window.__BASENAME__ || '';
  if (path.startsWith(basePathValue)) {
    return path;
  }
  return `${basePathValue}${path}`;
};
