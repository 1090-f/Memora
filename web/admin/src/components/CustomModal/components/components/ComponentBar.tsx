import React from 'react';
import {
  Box,
  IconButton,
  MenuItem,
  Popover,
  Select,
  Stack,
  Typography,
  alpha,
} from '@mui/material';
import { v4 as uuidv4 } from 'uuid';
import { IconWangyeguajian } from '@panda-wiki/icons';
import { Dispatch, SetStateAction, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { Component } from '../../index';
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
import { useAppDispatch, useAppSelector } from '@/store';
import { setAppPreviewData } from '@/store/slices/config';
import AddCircleRoundedIcon from '@mui/icons-material/AddCircleRounded';
import { IconShanchu } from '@panda-wiki/icons';
import { DEFAULT_DATA, COMPONENTS_MAP } from '../../constants';
import { THEME_LIST, THEME_TO_PALETTE } from '@panda-wiki/themes/constants';
interface ComponentBarProps {
  components: Component[];
  setComponents: Dispatch<SetStateAction<Component[]>>;
  curComponent: Component;
  setCurComponent: Dispatch<SetStateAction<Component>>;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
  allowAdd?: boolean;
}

const ThemeCard = ({ palette, label }: any) => {
  return (
    <Box
      sx={{
        boxShadow: '0px 0px 10px 0px rgba(0, 0, 0, 0.1)',
        bgcolor: palette.background.default,
        my: 0.5,
      }}
    >
      <Stack
        sx={{
          p: 1,
          width: '150px',
          height: '50px',
          bgcolor: alpha(palette.primary.main, 0.3),
        }}
      >
        <Box
          sx={{
            fontSize: 12,
            height: 20,
            color: palette.primary.main,
          }}
        >
          {label}
        </Box>
        <Box
          sx={{
            height: '120px',
            bgcolor: palette.background.default,
          }}
        ></Box>
      </Stack>
      <Box sx={{ height: '30px', bgcolor: palette.background.default }}></Box>
    </Box>
  );
};

const ComponentBar = ({
  components,
  setComponents,
  curComponent,
  setCurComponent,
  setIsEdit,
  allowAdd = true,
}: ComponentBarProps) => {
  const dispatch = useAppDispatch();
  const appPreviewData = useAppSelector(state => state.config.appPreviewData);
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const popoverOpen = Boolean(anchorEl);
  const options = useMemo(
    () => Object.values(COMPONENTS_MAP).filter(item => !item.fixed),
    [],
  );
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 5 },
    }),
  );

  const nonFixedIds = useMemo(
    () => components.filter(c => !c.fixed).map(c => c.id),
    [components],
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over) return;
    if (active.id === over.id) return;

    // 仅对非 fixed 项进行重排，fixed 保持原位置
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
    setIsEdit(true);
  };

  const SortableItem = ({ item }: { item: Component }) => {
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
      opacity: isDragging ? 0.6 : 1,
      cursor: isDragging ? 'move' : item.disabled ? 'not-allowed' : 'pointer',
    };
    return (
      <Stack
        ref={setNodeRef}
        direction={'row'}
        sx={{
          flexShrink: 0,
          cursor: 'not-allowed',
          height: '40px',
          borderRadius: '6px',
          bgcolor:
            item.id === curComponent.id
              ? '#F2F8FF'
              : item.disabled
                ? 'var(--mui-palette-action-disabledBackground)'
                : '',
          pl: '12px',
          alignItems: 'center',
          mb: '10px',
          transition: 'all .15s ease',
          '&:hover': {
            '.icon-shanchu': {
              display: item.fixed ? 'none' : 'block',
            },
          },
        }}
        style={style}
        key={item.id}
        onClick={() => {
          if (item.disabled) return;
          setCurComponent(item);
        }}
        {...(!item.fixed ? { ...attributes, ...listeners } : {})}
      >
        <IconWangyeguajian
          sx={{
            color:
              item.id === curComponent.id
                ? 'primary.main'
                : item.disabled
                  ? 'var(--mui-palette-action-disabled)'
                  : 'text.secondary',
            fontSize: '14px',
          }}
        ></IconWangyeguajian>
        <Typography
          sx={{
            marginLeft: '8px',
            fontSize: '14px',
            color:
              item.id === curComponent.id
                ? 'primary.main'
                : item.disabled
                  ? 'var(--mui-palette-action-disabled)'
                  : 'text.secondary',
            fontWeight: 500,
          }}
        >
          {item.title}
        </Typography>
        <IconShanchu
          className='icon-shanchu'
          sx={{ fontSize: '14px', ml: 'auto', mr: 1, display: 'none' }}
          onClick={e => {
            e.stopPropagation();
            if (item.fixed) return;
            const filterComponents = components.filter(c => c.id !== item.id);
            if (curComponent.id === item.id) {
              setCurComponent(
                filterComponents.find(c => !c.disabled && !c.hidden) ||
                  filterComponents[0],
              );
            }
            setComponents(filterComponents);
            setIsEdit(true);
          }}
        />
      </Stack>
    );
  };

  return (
    <Stack
      sx={{
        width: '20px',
        minWidth: '200px',
        bgcolor: '#FFFFFF',
        borderRight: '1px solid #ECEEF1',
        height: '100%',
        overflow: 'hidden',
        flexShrink: 0,
      }}
      direction={'column'}
    >
      {appPreviewData && (
        <>
          <Stack
            direction={'row'}
            sx={{
              justifyContent: 'space-between',
              alignItems: 'center',
              paddingTop: '19px',
            }}
          >
            <Typography
              sx={{
                fontSize: '16px',
                lineHeight: '30px',
                fontWeight: 600,
              }}
            >
              配色方案
            </Typography>
          </Stack>
          <Stack sx={{ pr: '20px', marginTop: '15px' }}>
            <Select
              value={
                THEME_TO_PALETTE[
                  appPreviewData.settings?.web_app_landing_theme?.name || 'blue'
                ]?.value || 'blue'
              }
              renderValue={value => {
                return THEME_TO_PALETTE[value]?.label;
              }}
              sx={{
                width: '100%',
                height: '40px',
                bgcolor: '#F2F8FF',
                border: '1px solid #5F58FE',
                color: '#5F58FE',
                '&:focus': {
                  border: '1px solid #5F58FE',
                },
                '&:hover': {
                  border: '1px solid #5F58FE',
                },
                '&.Mui-focused': {
                  border: '1px solid #5F58FE',
                },
                '& .MuiOutlinedInput-notchedOutline': {
                  border: 'none',
                },
              }}
              onChange={e => {
                if (!appPreviewData) return;
                const newInfo = {
                  ...appPreviewData,
                  settings: {
                    ...appPreviewData.settings,
                    web_app_landing_theme: {
                      name: e.target.value,
                    },
                  },
                };
                setIsEdit(true);
                dispatch(setAppPreviewData(newInfo));
              }}
            >
              {THEME_LIST.map(item => (
                <MenuItem key={item.value} value={item.value}>
                  <ThemeCard palette={item.palette} label={item.label} />
                </MenuItem>
              ))}
            </Select>
          </Stack>
        </>
      )}
      {allowAdd && (
        <Stack
          direction={'row'}
          sx={{
            justifyContent: 'space-between',
            alignItems: 'center',
            pr: '20px',
            paddingTop: '19px',
          }}
        >
          <Typography
            sx={{
              fontSize: '16px',
              lineHeight: '30px',
              fontWeight: 600,
            }}
          >
            组件
          </Typography>
          <IconButton
            size='small'
            onClick={e => {
              setAnchorEl(e.currentTarget);
            }}
          >
            <AddCircleRoundedIcon
              sx={{ fontSize: '16px', color: 'primary.main' }}
            />
          </IconButton>
        </Stack>
      )}

      <Popover
        open={popoverOpen}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        slotProps={{
          paper: {
            sx: {
              p: '12px',
              width: '282px',
            },
          },
        }}
      >
        <Stack
          sx={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr 1fr',
            gap: '12px',
          }}
        >
          {options.map(item => (
            <Stack
              key={item.name}
              direction={'column'}
              alignItems={'center'}
              gap={1}
              sx={{
                cursor: 'pointer',
                transition: 'all .15s ease',
                color: 'text.secondary',
                '&:hover': {
                  color: 'primary.main',
                },
              }}
              onClick={() => {
                const addComponent = {
                  id: uuidv4(),
                  name: item.name,
                  title: item.title,
                  component: item.component,
                  config: item.config,
                  fixed: false,
                };
                // if (components.find(c => c.name === item.name)) return;
                const newInfo = {
                  ...appPreviewData,
                  settings: {
                    ...(appPreviewData?.settings || {}),
                    web_app_landing_configs: [
                      ...(appPreviewData?.settings?.web_app_landing_configs ||
                        []),
                      {
                        type: item.name,
                        id: addComponent.id,
                        ...DEFAULT_DATA[item.name as keyof typeof DEFAULT_DATA],
                      },
                    ],
                  },
                };
                dispatch(setAppPreviewData(newInfo));
                setCurComponent(addComponent);
                setAnchorEl(null);
                setComponents([
                  ...components.slice(0, -1),
                  addComponent,
                  ...components.slice(-1),
                ]);
                setIsEdit(true);
              }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '10px',
                  width: '60px',
                  border: '1px solid',
                  borderColor: 'divider',
                  height: '60px',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor: '#F8FAFF',
                  },
                }}
              >
                {'icon' in item &&
                  item.icon &&
                  (() => {
                    const IconComponent = item.icon;
                    return <IconComponent sx={{ fontSize: '24px' }} />;
                  })()}
              </Box>
              <Typography sx={{ fontSize: '12px' }}>{item.title}</Typography>
            </Stack>
          ))}
        </Stack>
      </Popover>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <SortableContext
          items={nonFixedIds}
          strategy={verticalListSortingStrategy}
        >
          <Stack
            direction={'column'}
            sx={{
              marginTop: '15px',
              overflowY: 'auto',
              flex: 1,
              minHeight: 0,
              pr: '20px',
              paddingBottom: '20px',
            }}
          >
            {components.map(item => (
              <SortableItem key={item.id} item={item} />
            ))}
          </Stack>
        </SortableContext>
      </DndContext>
    </Stack>
  );
};

export default ComponentBar;
