import {
  closestCenter,
  DndContext,
  DragEndEvent,
  DragOverlay,
  DragStartEvent,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  arrayMove,
  rectSortingStrategy,
  SortableContext,
  SortingStrategy,
} from '@dnd-kit/sortable';
import { Stack, SxProps, Theme } from '@mui/material';
import {
  ComponentType,
  CSSProperties,
  Dispatch,
  SetStateAction,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';

export interface DragListProps<T extends { id?: string | null }> {
  data: T[];
  onChange: (data: T[]) => void;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
  SortableItemComponent: ComponentType<{
    id: string;
    item: T;
    handleRemove: (id: string) => void;
    handleUpdateItem: (item: T) => void;
    setIsEdit: Dispatch<SetStateAction<boolean>>;
  }>;
  ItemComponent: ComponentType<{
    isDragging?: boolean;
    item: T;
    style?: CSSProperties;
    setIsEdit: Dispatch<SetStateAction<boolean>>;
    handleUpdateItem?: (item: T) => void;
  }>;
  containerSx?: SxProps<Theme>;
  sortingStrategy?: SortingStrategy;
  direction?: 'row' | 'column';
  gap?: number;
}

function DragList<T extends { id?: string | null }>({
  data,
  onChange,
  setIsEdit,
  SortableItemComponent,
  ItemComponent,
  containerSx,
  sortingStrategy = rectSortingStrategy,
  direction = 'row',
  gap = 2,
}: DragListProps<T>) {
  const [activeId, setActiveId] = useState<string | null>(null);
  const sensors = useSensors(useSensor(MouseSensor), useSensor(TouchSensor));
  const dataRef = useRef(data);

  // 保持 ref 与 data 同步
  useEffect(() => {
    dataRef.current = data;
  }, [data]);

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveId(event.active.id as string);
  }, []);

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      if (active.id !== over?.id) {
        const currentData = dataRef.current;
        const oldIndex = currentData.findIndex(
          item => (item.id || '') === active.id,
        );
        const newIndex = currentData.findIndex(
          item => (item.id || '') === over!.id,
        );
        const newData = arrayMove(currentData, oldIndex, newIndex);
        onChange(newData);
      }
      setActiveId(null);
    },
    [onChange],
  );

  const handleDragCancel = useCallback(() => {
    setActiveId(null);
  }, []);

  const handleRemove = useCallback(
    (id: string) => {
      const currentData = dataRef.current;
      const newData = currentData.filter(item => (item.id || '') !== id);
      onChange(newData);
    },
    [onChange],
  );

  const handleUpdateItem = useCallback(
    (updatedItem: T) => {
      const currentData = dataRef.current;
      const newData = currentData.map(item =>
        (item.id || '') === (updatedItem.id || '') ? updatedItem : item,
      );
      onChange(newData);
    },
    [onChange],
  );

  if (data.length === 0) return null;

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      <SortableContext
        items={data.map(item => item.id || '')}
        strategy={sortingStrategy}
      >
        <Stack
          direction={direction}
          flexWrap={'wrap'}
          gap={gap}
          sx={containerSx}
        >
          {data.map(item => (
            <SortableItemComponent
              key={item.id || ''}
              id={item.id || ''}
              item={item}
              handleRemove={handleRemove}
              handleUpdateItem={handleUpdateItem}
              setIsEdit={setIsEdit}
            />
          ))}
        </Stack>
      </SortableContext>
      <DragOverlay adjustScale style={{ transformOrigin: '0 0' }}>
        {activeId ? (
          <ItemComponent
            isDragging
            item={data.find(item => (item.id || '') === activeId)!}
            setIsEdit={setIsEdit}
            handleUpdateItem={handleUpdateItem}
          />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}

export default DragList;
