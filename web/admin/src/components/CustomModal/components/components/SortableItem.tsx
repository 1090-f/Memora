import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ComponentType } from 'react';

export interface SortableItemProps<T extends { id?: string | null }> {
  id: string;
  item: T;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ItemComponent: ComponentType<any>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

function SortableItem<T extends { id?: string | null }>({
  id,
  item,
  ItemComponent,
  ...rest
}: SortableItemProps<T>) {
  const {
    isDragging,
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
  } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition: transition || undefined,
  };

  return (
    <ItemComponent
      ref={setNodeRef}
      style={style}
      withOpacity={isDragging}
      dragHandleProps={{
        ...attributes,
        ...listeners,
      }}
      item={item}
      {...rest}
    />
  );
}

export default SortableItem;
