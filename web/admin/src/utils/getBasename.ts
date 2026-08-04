// import router from '@/router';

declare global {
  interface Window {
    __BASENAME__: string;
  }
}

// 路由配置类型定义
type RouteConfig = {
  path: string;
  children?: RouteConfig[];
};

// 提取所有路由路径（包括嵌套路径）
function extractAllPaths(
  routes: RouteConfig[],
  parentPath: string = '',
): string[] {
  const paths: string[] = [];

  routes.forEach(route => {
    // 处理路径
    let routePath = route.path;

    // 处理空路径（空字符串表示继承父路径）
    if (routePath === '') {
      routePath = parentPath || '/';
    }
    // 根据 React Router 规则：
    // - 如果子路径以 / 开头，它是绝对路径，替换父路径
    // - 如果子路径不以 / 开头，它是相对路径，拼接在父路径后面
    else if (!routePath.startsWith('/')) {
      // 相对路径，拼接父路径
      if (parentPath === '/' || parentPath === '') {
        routePath = '/' + routePath;
      } else {
        routePath = parentPath + '/' + routePath;
      }
    }

    // 规范化路径（合并多个连续的 /）
    let normalizedPath = routePath.replace(/\/+/g, '/');
    // 移除末尾的 /（除非是根路径）
    if (normalizedPath !== '/' && normalizedPath.endsWith('/')) {
      normalizedPath = normalizedPath.slice(0, -1);
    }
    // 确保以 / 开头
    if (!normalizedPath.startsWith('/')) {
      normalizedPath = '/' + normalizedPath;
    }

    // 添加当前路径（包括根路径）
    paths.push(normalizedPath);

    // 递归处理子路由
    if (route.children && route.children.length > 0) {
      const childPaths = extractAllPaths(route.children, normalizedPath);
      paths.push(...childPaths);
    }
  });

  return paths;
}

// 根据当前 pathname 计算 basename
export function getBasename(pathname: string): string {
  // // 提取所有路由路径
  // const allPaths = extractAllPaths(router as RouteConfig[]);
  // // 分离根路径和其他路径
  const rootPath = '/';
  // const otherPaths = allPaths.filter(p => p !== rootPath);

  // // 按路由路径的段数（segment数量）降序排序，优先匹配段数更多的路径
  // // 例如：/doc/editor/:id (3段) 应该优先于 /feedback/:tab? (2段)
  // const sortedPaths = [
  //   ...otherPaths.sort((a, b) => {
  //     const aSegments = a.split('/').filter(Boolean).length;
  //     const bSegments = b.split('/').filter(Boolean).length;
  //     return bSegments - aSegments;
  //   }),
  //   rootPath,
  // ];

  const sortedPaths = [
    '/doc/editor/history/:id',
    '/doc/editor/:id',
    '/doc/editor/space',
    '/feedback/:tab?',
    '/doc/editor',
    '/setting',
    '/contribution',
    '/release',
    '/stat',
    '/conversation',
    '/login',
    '/401',
    '/',
  ];

  // 查找匹配的路径
  for (const routePath of sortedPaths) {
    // 跳过根路径的单独处理
    if (routePath === rootPath) {
      continue;
    }

    // 将路由路径和 pathname 分割成段
    const routeSegments = routePath.split('/').filter(Boolean);
    const pathSegments = pathname.split('/').filter(Boolean);

    // 计算路由路径的最小段数（不包括可选参数）
    const routeMinSegments = routeSegments.filter(s => !s.endsWith('?')).length;

    // 如果 pathname 的段数少于路由路径的最小段数，不匹配
    if (pathSegments.length < routeMinSegments) {
      continue;
    }

    // 从后往前匹配路由路径
    let routeIndex = routeSegments.length - 1;
    let pathIndex = pathSegments.length - 1;
    let matched = true;

    while (routeIndex >= 0 && pathIndex >= 0) {
      const routeSegment = routeSegments[routeIndex];
      const pathSegment = pathSegments[pathIndex];

      // 如果是动态参数（以 : 开头），直接匹配任意路径段
      if (routeSegment.startsWith(':')) {
        // 可选参数（:tab?）可以不匹配路径段
        if (routeSegment.endsWith('?')) {
          routeIndex--;
          // 如果还有路径段，尝试匹配；否则跳过可选参数
          if (pathIndex >= 0) {
            pathIndex--;
          }
        } else {
          // 必需参数，必须匹配一个路径段
          routeIndex--;
          pathIndex--;
        }
        continue;
      }

      // 静态部分必须完全匹配
      if (routeSegment !== pathSegment) {
        matched = false;
        break;
      }

      routeIndex--;
      pathIndex--;
    }

    // 处理剩余的可选参数
    while (routeIndex >= 0 && routeSegments[routeIndex].endsWith('?')) {
      routeIndex--;
    }

    // 如果路由路径还有未匹配的部分，说明不匹配
    if (routeIndex >= 0) {
      matched = false;
    }

    // 如果匹配成功，提取 basename
    if (matched) {
      // pathIndex + 1 是路由路径开始的位置
      if (pathIndex >= 0) {
        const basenameSegments = pathSegments.slice(0, pathIndex + 1);
        if (basenameSegments.length > 0) {
          return '/' + basenameSegments.join('/');
        }
      } else {
        // 路由路径完全匹配 pathname 的末尾
        // 计算实际匹配的路由段数（不包括可选参数）
        const matchedRouteSegments = routeSegments.filter(
          s => !s.endsWith('?'),
        ).length;
        const basenameSegments = pathSegments.slice(
          0,
          pathSegments.length - matchedRouteSegments,
        );
        if (basenameSegments.length > 0) {
          return '/' + basenameSegments.join('/');
        }
        // 如果 basename 为空，说明 pathname 就是路由路径本身
        return '';
      }
    }
  }

  // 如果没有匹配到任何路由，尝试从 pathname 中提取基础路径
  // 例如：/pc/admin/login -> /pc/admin
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length > 1) {
    // 移除最后一个段（通常是具体的路由）
    segments.pop();
    return '/' + segments.join('/');
  }

  // 如果 pathname 只有一个段（如 /admin），且不是根路径，则整个 pathname 就是 basename
  if (segments.length === 1 && pathname !== '/') {
    return pathname;
  }

  // 默认返回空字符串（根路径）
  return '';
}

// 检查 URL 是否是绝对路径（http/https）
function isAbsoluteUrl(url: string): boolean {
  return /^https?:\/\//i.test(url);
}

// 检查 URL 是否已经以 basename 开头
function startsWithBasename(url: string, basename: string): boolean {
  if (!basename) return false;
  // 移除开头的 /，统一处理
  const normalizedUrl = url.startsWith('/') ? url : '/' + url;
  const normalizedBasename = basename.startsWith('/')
    ? basename
    : '/' + basename;
  return normalizedUrl.startsWith(normalizedBasename);
}

// 处理 URL，如果需要则添加 basename
function processUrl(url: string, basename: string): string {
  // 如果是绝对路径（http/https），不处理
  if (isAbsoluteUrl(url)) {
    return url;
  }

  // 如果已经以 basename 开头，不处理
  if (startsWithBasename(url, basename)) {
    return url;
  }

  // 否则添加 basename
  const normalizedBasename = basename.endsWith('/')
    ? basename.slice(0, -1)
    : basename;
  const normalizedUrl = url.startsWith('/') ? url : '/' + url;
  return normalizedBasename + normalizedUrl;
}

// 包装 window.open，自动处理 basename
export function wrapWindowOpen(basename: string): void {
  const originalOpen = window.open;

  window.open = function (
    url?: string | URL | null,
    target?: string | undefined,
    features?: string | undefined,
  ): Window | null {
    // 如果 url 是字符串，处理 basename
    if (typeof url === 'string' && url) {
      const processedUrl = processUrl(url, basename);
      return originalOpen.call(window, processedUrl, target, features);
    }

    // 其他情况直接调用原始方法（处理 null 的情况）
    return originalOpen.call(window, url ?? undefined, target, features);
  };
}

// 初始化并注册 basename 到 window
export function initBasename(): string {
  const basename = getBasename(window.location.pathname);

  // 注册到 window 对象
  window.__BASENAME__ = basename.replace(/\/$/, '');

  // 包装 window.open
  wrapWindowOpen(basename);

  return basename;
}
