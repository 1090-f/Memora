import { readFileSync, writeFileSync } from 'fs';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * 按特殊性排序路由（最具体的在前）
 * 规则：
 * 1. 路径段数多的优先级高
 * 2. 静态段优先于参数段
 * 3. 必需参数优先于可选参数
 */
function sortRoutesBySpecificity(routes) {
  return routes.sort((a, b) => {
    const aParts = a.split('/').filter(Boolean);
    const bParts = b.split('/').filter(Boolean);

    // 首先按路径段数排序（段数多的在前）
    if (aParts.length !== bParts.length) {
      return bParts.length - aParts.length;
    }

    // 如果段数相同，比较每个段
    for (let i = 0; i < aParts.length; i++) {
      const aPart = aParts[i];
      const bPart = bParts[i];

      // 静态段优先于参数段
      const aIsStatic = !aPart.startsWith(':');
      const bIsStatic = !bPart.startsWith(':');

      if (aIsStatic && !bIsStatic) return -1;
      if (!aIsStatic && bIsStatic) return 1;

      // 如果都是参数，必需参数优先于可选参数
      if (!aIsStatic && !bIsStatic) {
        const aIsOptional = aPart.endsWith('?');
        const bIsOptional = bPart.endsWith('?');
        if (!aIsOptional && bIsOptional) return -1;
        if (aIsOptional && !bIsOptional) return 1;
      }

      // 如果都是静态段，按字母顺序（保持稳定性）
      if (aIsStatic && bIsStatic) {
        if (aPart !== bPart) {
          return aPart.localeCompare(bPart);
        }
      }
    }

    return 0;
  });
}

/**
 * 构建完整路径
 */
function buildFullPath(path, parentPath) {
  if (path === '/') {
    return parentPath || '';
  } else if (path.startsWith('/')) {
    return path;
  } else {
    if (!parentPath || parentPath === '/') {
      return `/${path}`;
    } else {
      return `${parentPath}/${path}`;
    }
  }
}

/**
 * 规范化路径
 */
function normalizePath(path) {
  if (!path || path === '/') return path;
  return path.endsWith('/') ? path.slice(0, -1) : path;
}

/**
 * 解析路由对象，返回路径和子路由内容
 */
function parseRouteObject(objContent, parentPath = '') {
  const routes = [];

  // 提取 path
  const pathMatch = objContent.match(/path:\s*['"`]([^'"`]+)['"`]/);
  if (!pathMatch) return routes;

  const path = pathMatch[1];
  const fullPath = normalizePath(buildFullPath(path, parentPath));

  // 如果路径不为空且不是根路径，添加到列表
  if (fullPath && fullPath !== '/') {
    routes.push(fullPath);
  }

  // 检查是否有 children，使用括号计数来正确匹配嵌套数组
  const childrenIndex = objContent.indexOf('children:');
  if (childrenIndex !== -1) {
    // 找到 children: 后面的 [
    let bracketStart = childrenIndex;
    while (
      bracketStart < objContent.length &&
      objContent[bracketStart] !== '['
    ) {
      bracketStart++;
    }

    if (bracketStart < objContent.length) {
      // 使用括号计数找到匹配的 ]
      let bracketCount = 1;
      let bracketEnd = bracketStart + 1;

      for (let i = bracketStart + 1; i < objContent.length; i++) {
        if (objContent[i] === '[') bracketCount++;
        if (objContent[i] === ']') {
          bracketCount--;
          if (bracketCount === 0) {
            bracketEnd = i;
            break;
          }
        }
      }

      // 提取 children 数组内容（不包括外层的 []）
      const childrenContent = objContent.substring(
        bracketStart + 1,
        bracketEnd,
      );

      // 分割 children 数组中的各个对象
      const childObjects = [];
      let depth = 0;
      let start = 0;

      for (let i = 0; i < childrenContent.length; i++) {
        if (childrenContent[i] === '{') {
          if (depth === 0) start = i;
          depth++;
        } else if (childrenContent[i] === '}') {
          depth--;
          if (depth === 0) {
            const childObj = childrenContent.substring(start, i + 1);
            childObjects.push(childObj);
          }
        }
      }

      // 递归处理每个子路由对象
      for (const childObj of childObjects) {
        const childRoutes = parseRouteObject(childObj, fullPath);
        routes.push(...childRoutes);
      }
    }
  }

  return routes;
}

/**
 * 解析路由配置
 */
function parseRoutes(content) {
  const routes = [];

  // 分割顶层路由数组中的各个对象
  const topLevelObjects = [];
  let depth = 0;
  let start = 0;

  for (let i = 0; i < content.length; i++) {
    if (content[i] === '{') {
      if (depth === 0) start = i;
      depth++;
    } else if (content[i] === '}') {
      depth--;
      if (depth === 0) {
        const obj = content.substring(start, i + 1);
        topLevelObjects.push(obj);
      }
    }
  }

  // 处理每个顶层路由对象
  for (const obj of topLevelObjects) {
    const objRoutes = parseRouteObject(obj);
    routes.push(...objRoutes);
  }

  return routes;
}

/**
 * 更新 index.html 中的路由列表
 */
function updateIndexHtml() {
  const routerPath = resolve(__dirname, '../src/router.tsx');
  const indexPath = resolve(__dirname, '../index.html');

  // 读取 router.tsx 文件
  const routerContent = readFileSync(routerPath, 'utf-8');

  // 提取 router 数组的内容
  const routerMatch = routerContent.match(
    /const\s+router\s*=\s*\[([\s\S]*?)\];/,
  );
  if (!routerMatch) {
    throw new Error('无法在 router.tsx 中找到 router 配置');
  }

  // 解析路由配置
  const extractedRoutes = parseRoutes(routerMatch[1]);

  // 去重并排序
  const uniqueRoutes = [...new Set(extractedRoutes)];
  const sortedRoutes = sortRoutesBySpecificity(uniqueRoutes);

  // 读取 index.html
  const htmlContent = readFileSync(indexPath, 'utf-8');

  // 生成新的路由数组字符串
  const routesString = sortedRoutes
    .map(route => `          '${route}'`)
    .join(',\n');

  // 替换路由数组（匹配 var routes = [...] 部分）
  const updatedContent = htmlContent.replace(
    /var routes = \[[\s\S]*?\];/,
    `var routes = [\n${routesString},\n        ];`,
  );

  // 写回文件
  writeFileSync(indexPath, updatedContent, 'utf-8');

  console.log('✅ 路由列表已更新:');
  sortedRoutes.forEach(route => console.log(`   ${route}`));
}

// 执行更新
try {
  updateIndexHtml();
} catch (error) {
  console.error('❌ 更新路由列表失败:', error.message);
  console.error(error.stack);
  process.exit(1);
}
