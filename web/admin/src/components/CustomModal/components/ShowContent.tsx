import { useAppSelector } from '@/store';
import { Box, Stack, useColorScheme, createTheme } from '@mui/material';
import { ThemeProvider } from '@ctzhian/ui';

import {
  Dispatch,
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  memo,
  useRef,
  useState,
} from 'react';
import { handleComponentProps } from '../utils';
import { themeOptions } from '@/themes';
import { IconShanchu } from '@panda-wiki/icons';
import { Component } from '..';
import {
  DndContext,
  DragEndEvent,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  arrayMove,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { CSSProperties, MouseEvent } from 'react';
import { THEME_TO_PALETTE } from '@panda-wiki/themes/constants';

interface ShowContentProps {
  curComponent: Component;
  setCurComponent: Dispatch<SetStateAction<Component>>;
  renderMode: 'pc' | 'mobile';
  scale: number;
  components: Component[];
  setComponents: Dispatch<SetStateAction<Component[]>>;
  setIsEdit?: Dispatch<SetStateAction<boolean>>;
  baseUrl: string;
}

interface SortableItemProps {
  item: Component;
  renderMode: 'pc' | 'mobile';
  // 预先缓存好的渲染 props，避免父组件每次重新计算
  cachedProps?: Record<string, unknown>;
  isHighlighted: boolean;
  onSelect: (item: Component) => void;
  onDelete?: (item: Component) => void;
  baseUrl: string;
}

const SortableItem = memo(
  ({
    item,
    renderMode,
    cachedProps,
    isHighlighted,
    onSelect,
    onDelete,
    baseUrl,
  }: SortableItemProps) => {
    const {
      attributes,
      listeners,
      setNodeRef,
      transform,
      transition,
      isDragging,
    } = useSortable({ id: item.id, disabled: !!item.fixed });
    const style: CSSProperties = {
      transform: CSS.Transform.toString(transform),
      transition,
      opacity: isDragging ? 0.9 : 1,
      cursor: isDragging ? 'move' : undefined,
    };

    return (
      <Box
        sx={{
          position: 'relative',
          border: isHighlighted ? '2px solid #5F58FE' : '2px solid transparent',
          borderRadius: '0px',
          padding: '2px',
          cursor: item.fixed ? 'default' : 'move',
          '&:hover': {
            border: isHighlighted ? '2px solid #5F58FE' : '2px dashed #5F58FE',
          },
        }}
        data-component={item.id}
        ref={setNodeRef}
        style={style}
        {...(!item.fixed ? { ...attributes, ...listeners } : {})}
        onClick={() => onSelect(item)}
      >
        <item.component
          mobile={renderMode === 'mobile'}
          docWidth={renderMode === 'pc' ? 'full' : 'normal'}
          {...(cachedProps || {})}
          basePath={baseUrl}
        />
        {isHighlighted && (
          <Stack
            direction={'row'}
            alignItems={'center'}
            gap={2}
            sx={{
              position: 'absolute',
              left: '-2px',
              ...(item?.name === 'footer'
                ? { top: '-24px' }
                : { bottom: '-24px' }),
              fontWeight: 400,
              color: '#FFFFFF',
              fontSize: '14px',
              zIndex: 20,
            }}
          >
            <Box sx={{ bgcolor: '#5F58FE', padding: '1px 16px', height: 24 }}>
              {item?.title}
            </Box>
            {!item.fixed && (
              <Stack
                justifyContent='center'
                alignItems='center'
                sx={{ bgcolor: '#5F58FE', height: 24, px: 0.5 }}
                onClick={(e: MouseEvent<HTMLDivElement>) => {
                  e.stopPropagation();
                  onDelete?.(item);
                }}
              >
                <IconShanchu sx={{ fontSize: '16px' }} />
              </Stack>
            )}
          </Stack>
        )}
      </Box>
    );
  },
  // (prev, next) => {
  //   if (!isSameItemShallow(prev.item, next.item)) return false;
  //   if (prev.isHighlighted !== next.isHighlighted) return false;
  //   if (prev.renderMode !== next.renderMode) return false;
  //   // 仅当缓存 props 引用变化时重渲染
  //   if (prev.cachedProps !== next.cachedProps) return false;
  //   return true;
  // },
);

const ShowContent = ({
  setCurComponent,
  curComponent,
  renderMode,
  scale,
  components,
  setComponents,
  setIsEdit,
  baseUrl,
}: ShowContentProps) => {
  const { appPreviewData } = useAppSelector(state => state.config);
  const { setMode } = useColorScheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const isComponentClickRef = useRef(false);

  useEffect(() => {
    setMode(appPreviewData?.settings?.theme_mode as 'light' | 'dark');
  }, [appPreviewData?.settings?.theme_mode, setMode]);

  const handleScroll = () => {
    const targetElement = containerRef.current?.querySelector(
      `[data-component="${curComponent.id}"]`,
    );
    if (targetElement) {
      targetElement.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
        inline: 'nearest',
      });
    }
    if (!targetElement) {
      setTimeout(() => {
        handleScroll();
      }, 100);
    }
  };

  // 滚动到当前选中的组件（仅在组件真正改变时）
  useEffect(() => {
    if (
      !curComponent?.id ||
      !containerRef.current ||
      isComponentClickRef.current
    ) {
      isComponentClickRef.current = false;
      return;
    }
    handleScroll();
  }, [curComponent]);

  const handleSelect = useCallback(
    (item: Component) => {
      if (item.disabled) return;
      setCurComponent(item);
      isComponentClickRef.current = true;
    },
    [setCurComponent],
  );

  const handleDelete = useCallback(
    (item: Component) => {
      const filterComponents = components.filter(c => c.id !== item.id);
      if (curComponent?.id === item.id) {
        setCurComponent(
          filterComponents.find(c => !c.disabled && !c.hidden) ||
            filterComponents[0],
        );
      }
      setComponents(filterComponents);
      setIsEdit?.(true);
    },
    [components, curComponent?.id, setComponents, setCurComponent, setIsEdit],
  );

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  );

  const nonFixedIds = useMemo(
    () => components.filter(c => !c.fixed).map(c => c.id),
    [components],
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over) return;
    if (active.id === over.id) return;

    const nonFixedItems = components.filter(c => !c.fixed);
    const fromIdx = nonFixedItems.findIndex(c => c.id === active.id);
    const toIdx = nonFixedItems.findIndex(c => c.id === over.id);
    if (fromIdx === -1 || toIdx === -1) return;

    const newNonFixed = arrayMove(nonFixedItems, fromIdx, toIdx);

    const result: Component[] = [];
    let cursor = 0;
    for (let i = 0; i < components.length; i++) {
      const cur = components[i];
      if (cur.fixed) {
        result.push(cur);
      } else {
        result.push(newNonFixed[cursor]);
        cursor += 1;
      }
    }
    setComponents(result);
    const newCur = result.find(c => c.id === curComponent.id);
    if (newCur) setCurComponent(newCur);
    if (setIsEdit) setIsEdit(true);
  };

  // app settings 引用：作为传递给子组件的 props 变化依据
  const appSettings = appPreviewData?.settings;

  // 每个组件项的 props 缓存，仅在必要时更新
  const propsCacheRef = useRef<
    Record<string, Record<string, unknown> | undefined>
  >({});
  const [cacheTick, setCacheTick] = useState(0);

  // 初始化/同步缓存（新增、删除）
  useEffect(() => {
    const nextKeys = new Set(components.map(c => c.id));
    // 新增项：补齐缓存
    components.forEach(c => {
      if (!propsCacheRef.current[c.id]) {
        propsCacheRef.current[c.id] =
          handleComponentProps(c.name, c.id, appSettings) || {};
      }
    });
    // 移除项：清理缓存
    Object.keys(propsCacheRef.current).forEach(k => {
      if (!nextKeys.has(k)) delete propsCacheRef.current[k];
    });
    setCacheTick(t => t + 1);
  }, [appSettings, components]);

  // appSettings 变化时，只更新当前高亮组件的缓存，其他组件沿用旧 props
  useEffect(() => {
    if (!curComponent?.id) return;
    propsCacheRef.current[curComponent.id] =
      handleComponentProps(curComponent.name, curComponent.id, appSettings) ||
      {};
    setCacheTick(t => t + 1);
  }, [appSettings, curComponent?.id]);

  // 渲染项缓存：仅在关键签名或必要依赖变更时重建
  const renderedItems = useMemo(() => {
    return components
      .filter(item => !item.hidden)
      .map(item =>
        propsCacheRef.current[item.id] ? (
          <SortableItem
            key={item.id}
            item={item}
            renderMode={renderMode}
            cachedProps={propsCacheRef.current[item.id]}
            isHighlighted={curComponent?.id === item.id}
            onSelect={handleSelect}
            onDelete={handleDelete}
            baseUrl={baseUrl}
          />
        ) : null,
      );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    renderMode,
    curComponent?.id,
    handleSelect,
    handleDelete,
    cacheTick,
    baseUrl,
  ]);

  return (
    <Stack
      ref={containerRef}
      className='show-content-container'
      sx={{
        flex: 1,
        flexShrink: 0,
        my: '20px',
        border: '1px solid #ECEEF1',
        '&::-webkit-scrollbar': {
          height: '8px', // 滚动条高度
        },
        overflow: 'auto',

        '&::-webkit-scrollbar-thumb': {
          background: '#888', // 滑块颜色
          borderRadius: '4px',
        },
      }}
    >
      <Stack
        sx={{
          minWidth: renderMode === 'pc' ? `1200px` : '375px',
          width: renderMode === 'pc' ? `100%` : '375px',
          margin: '0 auto',
          boxShadow:
            renderMode === 'pc' ? null : '0 10px 15px -3px rgb(0 0 0 / 0.1)',
          // minHeight: '800px',
          // height: '100%',
          bgcolor: 'background.default',
          position: 'relative',
          transform: `scale(${scale})`,
          transformOrigin: 'center center',
          transition: 'transform 0.2s ease',
        }}
      >
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext
            items={nonFixedIds}
            strategy={verticalListSortingStrategy}
          >
            {renderedItems}
          </SortableContext>
        </DndContext>
      </Stack>
    </Stack>
  );
};

const ThemeWrapper = ({ children }: { children: React.ReactNode }) => {
  const { appPreviewData } = useAppSelector(state => state.config);

  const theme = useMemo(() => {
    const themeName =
      appPreviewData?.settings?.web_app_landing_theme?.name || 'blue';
    return createTheme(
      // @ts-expect-error themeOptions is not typed
      {
        ...themeOptions[0],
        palette:
          THEME_TO_PALETTE[themeName]?.palette || THEME_TO_PALETTE.blue.palette,
      },
      ...themeOptions.slice(1),
    );
  }, [appPreviewData?.settings?.web_app_landing_theme?.name]);

  return (
    <ThemeProvider theme={theme} storageManager={null}>
      {children}
    </ThemeProvider>
  );
};

const Content = (props: ShowContentProps) => {
  return (
    <ThemeWrapper>
      <ShowContent {...props} />
    </ThemeWrapper>
  );
};

export default Content;
